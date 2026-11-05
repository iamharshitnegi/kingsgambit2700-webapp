//go:build integration

package report

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresRepository_MonthlyPublicationAndReceiptLifecycle(t *testing.T) {
	databaseURL := os.Getenv("KG_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)

	coachID := platformid.New()
	guardianID := platformid.New()
	studentUserID := platformid.New()
	studentID := platformid.New()
	reportID := platformid.New()
	versionID := platformid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedReportRepositoryTest(
		t,
		pool,
		coachID,
		guardianID,
		studentUserID,
		studentID,
		now,
	)
	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}
	service, err := NewService(repository, integrationReportAuthorizer{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	monthly, err := service.CreateMonthly(t.Context(), CreateCommand{
		ReportID:        reportID,
		StudentID:       studentID,
		Year:            now.Year(),
		Month:           now.Month(),
		VersionID:       versionID,
		RubricVersionID: platformid.New(),
		Content:         integrationContent("draft"),
		ActorID:         coachID,
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateMonthly() error = %v", err)
	}
	monthly, err = service.EditDraft(t.Context(), EditCommand{
		ReportID:         monthly.ID(),
		VersionID:        versionID,
		ExpectedRevision: monthly.Revision(),
		Content:          integrationContent("reviewed"),
		ActorID:          coachID,
		UpdatedAt:        now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("EditDraft() error = %v", err)
	}
	monthly, err = service.Publish(t.Context(), PublishCommand{
		ReportID:         monthly.ID(),
		VersionID:        versionID,
		ExpectedRevision: monthly.Revision(),
		ActorID:          coachID,
		PublishedAt:      now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Publish() error = %v", err)
	}
	monthly, err = service.RecordView(t.Context(), ReceiptCommand{
		ReportID:         monthly.ID(),
		VersionID:        versionID,
		ViewerID:         guardianID,
		ExpectedRevision: monthly.Revision(),
		OccurredAt:       now.Add(3 * time.Minute),
	})
	if err != nil {
		t.Fatalf("RecordView() error = %v", err)
	}
	monthly, err = service.Acknowledge(t.Context(), ReceiptCommand{
		ReportID:         monthly.ID(),
		VersionID:        versionID,
		ViewerID:         guardianID,
		ExpectedRevision: monthly.Revision(),
		OccurredAt:       now.Add(4 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Acknowledge() error = %v", err)
	}

	loaded, err := repository.Report(t.Context(), reportID)
	if err != nil {
		t.Fatalf("Report() error = %v", err)
	}
	if loaded.Revision() != 5 {
		t.Fatalf("Report() revision = %d, want 5", loaded.Revision())
	}
	versions := loaded.Versions()
	if len(versions) != 1 || versions[0].Status != VersionStatusPublished ||
		versions[0].Content != integrationContent("reviewed") {
		t.Fatalf("Report() versions = %+v", versions)
	}
	receipts := loaded.Receipts()
	if len(receipts) != 1 || receipts[0].AcknowledgedAt == nil {
		t.Fatalf("Report() receipts = %+v", receipts)
	}
}

type integrationReportAuthorizer struct{}

func (integrationReportAuthorizer) AuthorizeReport(
	context.Context,
	string,
	string,
	Action,
) error {
	return nil
}

func seedReportRepositoryTest(
	t *testing.T,
	pool *pgxpool.Pool,
	coachID string,
	guardianID string,
	studentUserID string,
	studentID string,
	now time.Time,
) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES
			($1::uuid, $2, 'Integration Coach', 'active', $7, $7),
			($3::uuid, $4, 'Integration Guardian', 'active', $7, $7),
			($5::uuid, $6, 'Integration Student', 'active', $7, $7)`,
		coachID,
		coachID+"@example.invalid",
		guardianID,
		guardianID+"@example.invalid",
		studentUserID,
		studentUserID+"@example.invalid",
		now,
	)
	if err != nil {
		t.Fatalf("inserting users: %v", err)
	}
	_, err = pool.Exec(t.Context(), `
		INSERT INTO student_profiles
			(id, user_id, preferred_name, age_band, country, time_zone, locale,
			 language, status, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, 'Student', '8-12', 'IN', 'Asia/Kolkata',
		        'en-IN', 'en', 'active', $3, $3)`,
		studentID,
		studentUserID,
		now,
	)
	if err != nil {
		t.Fatalf("inserting student: %v", err)
	}
}

func integrationContent(prefix string) Content {
	return Content{
		Learned:      Written(prefix + " learned"),
		Strengths:    Written(prefix + " strengths"),
		FocusNext:    Written(prefix + " focus next"),
		PracticePlan: Written(prefix + " practice plan"),
		CoachComment: Written(prefix + " coach comment"),
	}
}
