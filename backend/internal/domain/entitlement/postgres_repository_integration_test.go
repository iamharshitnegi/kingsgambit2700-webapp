//go:build integration

package entitlement

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresRepository_ApplyAndRead(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	fixture := createEntitlementFixture(t, pool)
	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)

	base := OperationCommand{
		AccountID:      fixture.accountID,
		StudentID:      fixture.studentID,
		IdempotencyKey: "opening-balance-2026",
		ReferenceType:  "migration",
		ReferenceID:    "opening-balance-row-1",
		Units:          4,
		Reason:         "approved opening balance",
		ActorID:        fixture.actorID,
		OccurredAt:     now,
	}

	const attempts = 12
	results := make(chan ApplyResult, attempts)
	errorsFound := make(chan error, attempts)
	var waitGroup sync.WaitGroup
	for index := range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			command := base
			command.EntryID = platformid.New()
			command.OccurredAt = now.Add(time.Duration(index) * time.Second)
			result, err := service.Grant(t.Context(), command)
			if err != nil {
				errorsFound <- err
				return
			}
			results <- result
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errorsFound)
	for err := range errorsFound {
		t.Errorf("concurrent Grant() error = %v", err)
	}
	created := 0
	var original ApplyResult
	for result := range results {
		if result.Created {
			created++
			original = result
		}
		if result.Balance != (Balance{Available: 4}) {
			t.Errorf("concurrent balance = %+v, want available 4", result.Balance)
		}
	}
	if created != 1 {
		t.Fatalf("created entries = %d, want 1", created)
	}

	conflict := base
	conflict.EntryID = platformid.New()
	conflict.Units = 5
	if _, err := service.Grant(t.Context(), conflict); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("Grant(conflict) error = %v, want %v", err, ErrIdempotencyConflict)
	}

	reserve, err := service.Reserve(t.Context(), OperationCommand{
		EntryID:        platformid.New(),
		AccountID:      fixture.accountID,
		StudentID:      fixture.studentID,
		IdempotencyKey: "reserve-session-1",
		ReferenceType:  "class_session",
		ReferenceID:    fixture.sessionID,
		Units:          1,
		Reason:         "session reserved",
		ActorID:        "system",
		OccurredAt:     now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if reserve.Balance != (Balance{Available: 3, Reserved: 1}) {
		t.Fatalf("Reserve() balance = %+v", reserve.Balance)
	}
	consumed, err := service.Consume(t.Context(), ConsumeCommand{
		OperationCommand: OperationCommand{
			EntryID:        platformid.New(),
			AccountID:      fixture.accountID,
			StudentID:      fixture.studentID,
			IdempotencyKey: "consume-session-1",
			ReferenceType:  "class_session",
			ReferenceID:    fixture.sessionID,
			Units:          1,
			Reason:         "session completed",
			ActorID:        "system",
			OccurredAt:     now.Add(2 * time.Minute),
		},
		Source: ConsumptionSourceReserved,
	})
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if consumed.Balance != (Balance{Available: 3, Consumed: 1}) {
		t.Fatalf("Consume() balance = %+v", consumed.Balance)
	}

	if _, err := service.Reserve(t.Context(), OperationCommand{
		EntryID:        platformid.New(),
		AccountID:      fixture.accountID,
		StudentID:      fixture.studentID,
		IdempotencyKey: "reserve-too-many",
		ReferenceType:  "external",
		ReferenceID:    "future-session",
		Units:          99,
		Reason:         "must not overdraw",
		ActorID:        "system",
		OccurredAt:     now.Add(3 * time.Minute),
	}); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("Reserve(overdraw) error = %v, want %v", err, ErrInsufficientBalance)
	}

	account, err := repository.Account(t.Context(), fixture.accountID)
	if err != nil {
		t.Fatalf("Account() error = %v", err)
	}
	if account.StudentID != fixture.studentID || account.EnrolmentID != fixture.enrolmentID {
		t.Fatalf("Account() = %+v", account)
	}
	balance, err := repository.Balance(t.Context(), fixture.accountID)
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if balance != consumed.Balance {
		t.Fatalf("Balance() = %+v, want %+v", balance, consumed.Balance)
	}

	firstPage, err := repository.EntryPage(t.Context(), fixture.accountID, PageRequest{Limit: 2})
	if err != nil {
		t.Fatalf("EntryPage(first) error = %v", err)
	}
	if len(firstPage.Items) != 2 || firstPage.NextCursor == nil {
		t.Fatalf("EntryPage(first) = %+v", firstPage)
	}
	secondPage, err := repository.EntryPage(t.Context(), fixture.accountID, PageRequest{
		Cursor: firstPage.NextCursor,
		Limit:  2,
	})
	if err != nil {
		t.Fatalf("EntryPage(second) error = %v", err)
	}
	if len(secondPage.Items) != 1 || secondPage.NextCursor != nil {
		t.Fatalf("EntryPage(second) = %+v", secondPage)
	}
	if secondPage.Items[0].ID != original.Entry.ID {
		t.Fatalf("oldest entry = %q, want %q", secondPage.Items[0].ID, original.Entry.ID)
	}

	var entryCount, auditCount int
	err = pool.QueryRow(t.Context(), `
		SELECT
			(SELECT count(*) FROM entitlement_entries WHERE account_id = $1::uuid),
			(SELECT count(*) FROM audit_events
			 WHERE object_type = 'entitlement_account' AND object_id = $1::text)`,
		fixture.accountID,
	).Scan(&entryCount, &auditCount)
	if err != nil {
		t.Fatalf("counting ledger evidence: %v", err)
	}
	if entryCount != 3 || auditCount != 3 {
		t.Fatalf("entry count = %d, audit count = %d, want 3 and 3", entryCount, auditCount)
	}
	if _, err := pool.Exec(t.Context(), `
		UPDATE entitlement_entries SET reason = 'tampered' WHERE id = $1::uuid`,
		original.Entry.ID,
	); err == nil {
		t.Fatal("append-only entitlement entry accepted an update")
	}
}

type entitlementFixture struct {
	actorID     string
	studentID   string
	enrolmentID string
	accountID   string
	sessionID   string
}

func createEntitlementFixture(t *testing.T, pool *pgxpool.Pool) entitlementFixture {
	t.Helper()
	fixture := entitlementFixture{
		actorID:     platformid.New(),
		studentID:   platformid.New(),
		enrolmentID: platformid.New(),
		accountID:   platformid.New(),
		sessionID:   platformid.New(),
	}
	studentUserID := platformid.New()
	batchID := platformid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)
	statements := []struct {
		query string
		args  []any
	}{
		{
			query: `INSERT INTO users
				(id, email, display_name, status, created_at, updated_at)
				VALUES ($1::uuid, $2, 'Finance Admin', 'active', $3, $3)`,
			args: []any{fixture.actorID, "finance-" + fixture.actorID + "@example.test", now},
		},
		{
			query: `INSERT INTO users
				(id, display_name, status, created_at, updated_at)
				VALUES ($1::uuid, 'Ledger Student', 'active', $2, $2)`,
			args: []any{studentUserID, now},
		},
		{
			query: `INSERT INTO student_profiles
				(id, user_id, preferred_name, age_band, country, time_zone,
				 locale, language, status, created_at, updated_at)
				VALUES ($1::uuid, $2::uuid, 'Student', '8-12', 'IN', 'Asia/Kolkata',
				 'en-IN', 'en', 'active', $3, $3)`,
			args: []any{fixture.studentID, studentUserID, now},
		},
		{
			query: `INSERT INTO batches
				(id, program_id, level_slug, format, source_time_zone,
				 target_capacity, hard_capacity, status, created_at, updated_at)
				VALUES ($1::uuid, '01900000-0000-7000-8000-000000000002'::uuid,
				 'beginner', 'one_to_one', 'Asia/Kolkata', 1, 1, 'active', $2, $2)`,
			args: []any{batchID, now},
		},
		{
			query: `INSERT INTO enrolments
				(id, student_profile_id, batch_id, status, policy_snapshot,
				 starts_at, created_at, updated_at)
				VALUES ($1::uuid, $2::uuid, $3::uuid, 'active', '{}'::jsonb,
				 $4, $4, $4)`,
			args: []any{fixture.enrolmentID, fixture.studentID, batchID, now},
		},
		{
			query: `INSERT INTO entitlement_accounts
				(id, enrolment_id, unit, created_at)
				VALUES ($1::uuid, $2::uuid, 'session_credit', $3)`,
			args: []any{fixture.accountID, fixture.enrolmentID, now},
		},
		{
			query: `INSERT INTO class_sessions
				(id, batch_id, kind, starts_at, ends_at, source_time_zone,
				 source_local_start, join_opens_at, status, created_at, updated_at)
				VALUES ($1::uuid, $2::uuid, 'one_to_one', $3, $4, 'Asia/Kolkata',
				 $5, $6, 'scheduled', $3, $3)`,
			args: []any{
				fixture.sessionID,
				batchID,
				now,
				now.Add(time.Hour),
				now.In(time.FixedZone("IST", 5*60*60+30*60)).Format("2006-01-02 15:04:05"),
				now.Add(-15 * time.Minute),
			},
		},
	}
	for index, statement := range statements {
		if _, err := pool.Exec(t.Context(), statement.query, statement.args...); err != nil {
			t.Fatalf("inserting entitlement fixture statement %d: %v", index, err)
		}
	}
	return fixture
}
