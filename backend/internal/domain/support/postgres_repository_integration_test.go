//go:build integration

package support

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresRepository_SupportLifecycleRedactsInternalTimeline(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("KG_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL or KG_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)

	now := time.Now().UTC().Truncate(time.Microsecond)
	familyID := platformid.New()
	customerID := platformid.New()
	managerID := platformid.New()
	assigneeID := platformid.New()
	intruderID := platformid.New()
	seedSupportRepositoryTest(
		t,
		pool,
		familyID,
		[]string{customerID, managerID, assigneeID, intruderID},
		now,
	)

	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}
	authorizer := integrationSupportAuthorizer{
		familyID:   familyID,
		customerID: customerID,
		managerID:  managerID,
		assigneeID: assigneeID,
	}
	service, err := NewService(repository, authorizer, DefaultConfig())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	supportCase, err := service.Create(t.Context(), CreateCommand{
		CaseID:     platformid.New(),
		EventID:    platformid.New(),
		FamilyID:   familyID,
		Category:   CategoryLearning,
		Severity:   SeverityHigh,
		Subject:    "Assignment review question",
		Message:    "Please help us understand the requested revision.",
		ActorID:    customerID,
		OccurredAt: now,
		RequestID:  "support-create-integration",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if supportCase.Version != 1 || len(supportCase.Events) != 1 {
		t.Fatalf("Create() = %+v", supportCase)
	}

	supportCase, err = service.AppendEvent(t.Context(), AppendEventCommand{
		CaseID:          supportCase.ID,
		EventID:         platformid.New(),
		ExpectedVersion: supportCase.Version,
		Visibility:      VisibilityInternal,
		Body:            "Do not disclose this staff-only triage note.",
		ActorID:         managerID,
		OccurredAt:      now.Add(time.Minute),
		RequestID:       "support-internal-integration",
	})
	if err != nil {
		t.Fatalf("AppendEvent(internal) error = %v", err)
	}
	supportCase, err = service.AppendEvent(t.Context(), AppendEventCommand{
		CaseID:          supportCase.ID,
		EventID:         platformid.New(),
		ExpectedVersion: supportCase.Version,
		Visibility:      VisibilityCustomerVisible,
		Body:            "We are reviewing this with the academic team.",
		ActorID:         managerID,
		OccurredAt:      now.Add(2 * time.Minute),
		RequestID:       "support-visible-integration",
	})
	if err != nil {
		t.Fatalf("AppendEvent(visible) error = %v", err)
	}
	supportCase, err = service.Assign(t.Context(), AssignCommand{
		CaseID:          supportCase.ID,
		EventID:         platformid.New(),
		ExpectedVersion: supportCase.Version,
		AssigneeID:      assigneeID,
		ActorID:         managerID,
		Reason:          "academic support ownership",
		OccurredAt:      now.Add(3 * time.Minute),
		RequestID:       "support-assign-integration",
	})
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
	supportCase, err = service.ChangeStatus(t.Context(), ChangeStatusCommand{
		CaseID:          supportCase.ID,
		EventID:         platformid.New(),
		ExpectedVersion: supportCase.Version,
		Status:          StatusInvestigating,
		ActorID:         managerID,
		Reason:          "academic review started",
		OccurredAt:      now.Add(4 * time.Minute),
		RequestID:       "support-status-integration",
	})
	if err != nil {
		t.Fatalf("ChangeStatus() error = %v", err)
	}

	customerView, err := service.View(t.Context(), customerID, supportCase.ID)
	if err != nil {
		t.Fatalf("View(customer) error = %v", err)
	}
	if len(customerView.Events) != 3 {
		t.Fatalf("View(customer) events = %+v, want 3", customerView.Events)
	}
	for _, event := range customerView.Events {
		if event.Visibility != VisibilityCustomerVisible ||
			event.Body == "Do not disclose this staff-only triage note." {
			t.Fatalf("View(customer) leaked event = %+v", event)
		}
	}

	managerView, err := service.View(t.Context(), managerID, supportCase.ID)
	if err != nil {
		t.Fatalf("View(manager) error = %v", err)
	}
	if len(managerView.Events) != 5 {
		t.Fatalf("View(manager) events = %+v, want 5", managerView.Events)
	}

	if _, err := service.View(t.Context(), intruderID, supportCase.ID); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("View(intruder) error = %v, want permission denied", err)
	}
	if _, err := service.AppendEvent(t.Context(), AppendEventCommand{
		CaseID:          supportCase.ID,
		EventID:         platformid.New(),
		ExpectedVersion: 1,
		Visibility:      VisibilityCustomerVisible,
		Body:            "stale reply",
		ActorID:         customerID,
		OccurredAt:      now.Add(5 * time.Minute),
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("AppendEvent(stale) error = %v, want conflict", err)
	}

	page, err := service.List(t.Context(), ListCommand{
		FamilyID: familyID,
		ActorID:  customerID,
		Options:  ListOptions{Limit: 20},
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != supportCase.ID {
		t.Fatalf("List() = %+v", page)
	}

	var auditCount int
	if err := pool.QueryRow(t.Context(), `
		SELECT count(*)
		FROM audit_events
		WHERE object_type = 'support_case' AND object_id = $1`,
		supportCase.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("counting support audit rows: %v", err)
	}
	if auditCount != 5 {
		t.Fatalf("support audit count = %d, want 5", auditCount)
	}
	if _, err := pool.Exec(t.Context(), `
		UPDATE support_case_events SET body = 'tampered' WHERE support_case_id = $1::uuid`,
		supportCase.ID,
	); err == nil {
		t.Fatal("append-only support event accepted an update")
	}
}

type integrationSupportAuthorizer struct {
	familyID   string
	customerID string
	managerID  string
	assigneeID string
}

func (a integrationSupportAuthorizer) SupportAccess(
	_ context.Context,
	actorID string,
	familyID string,
) (Access, error) {
	if actorID == a.managerID {
		return Access{CanManage: true}, nil
	}
	if actorID == a.customerID && familyID == a.familyID {
		return Access{CanViewFamily: true}, nil
	}
	return Access{}, nil
}

func (a integrationSupportAuthorizer) AuthorizeSupportAssignee(
	_ context.Context,
	actorID string,
	assigneeID string,
) error {
	if actorID != a.managerID || assigneeID != a.assigneeID {
		return ErrPermissionDenied
	}
	return nil
}

func seedSupportRepositoryTest(
	t *testing.T,
	pool *pgxpool.Pool,
	familyID string,
	userIDs []string,
	now time.Time,
) {
	t.Helper()
	for index, userID := range userIDs {
		_, err := pool.Exec(t.Context(), `
			INSERT INTO users (id, email, display_name, status, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, 'active', $4, $4)`,
			userID,
			userID+"@example.invalid",
			"Support user "+string(rune('A'+index)),
			now,
		)
		if err != nil {
			t.Fatalf("inserting support user: %v", err)
		}
	}
	if _, err := pool.Exec(t.Context(), `
		INSERT INTO families (id, display_name, created_at, updated_at)
		VALUES ($1::uuid, 'Integration Support Family', $2, $2)`,
		familyID,
		now,
	); err != nil {
		t.Fatalf("inserting support family: %v", err)
	}
}
