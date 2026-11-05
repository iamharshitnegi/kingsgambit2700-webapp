//go:build integration

package ticket

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	authmodel "github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestService_IssueEnforcesSessionWindowAndAssignment(t *testing.T) {
	databaseURL := ticketTestDatabaseURL(t)
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	service := NewService(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	tests := []struct {
		name               string
		status             string
		joinOpensAt        time.Time
		endsAt             time.Time
		assigned           bool
		assignmentStartsAt time.Time
		assignmentEndsAt   *time.Time
		superAdmin         bool
		wantAllowed        bool
	}{
		{
			name:               "assigned user in an open join window",
			status:             "join_open",
			joinOpensAt:        now.Add(-10 * time.Minute),
			endsAt:             now.Add(time.Hour),
			assigned:           true,
			assignmentStartsAt: now.Add(-time.Hour),
			wantAllowed:        true,
		},
		{
			name:               "assigned user in a live session",
			status:             "live",
			joinOpensAt:        now.Add(-30 * time.Minute),
			endsAt:             now.Add(time.Hour),
			assigned:           true,
			assignmentStartsAt: now.Add(-time.Hour),
			wantAllowed:        true,
		},
		{
			name:               "scheduled class is joinable inside its window",
			status:             "scheduled",
			joinOpensAt:        now.Add(-10 * time.Minute),
			endsAt:             now.Add(time.Hour),
			assigned:           true,
			assignmentStartsAt: now.Add(-time.Hour),
			wantAllowed:        true,
		},
		{
			name:               "scheduled class join window has not opened",
			status:             "scheduled",
			joinOpensAt:        now.Add(10 * time.Minute),
			endsAt:             now.Add(time.Hour),
			assigned:           true,
			assignmentStartsAt: now.Add(-time.Hour),
		},
		{
			name:               "scheduled class has ended",
			status:             "scheduled",
			joinOpensAt:        now.Add(-2 * time.Hour),
			endsAt:             now.Add(-time.Minute),
			assigned:           true,
			assignmentStartsAt: now.Add(-2 * time.Hour),
		},
		{
			name:        "ordinary user is not assigned",
			status:      "join_open",
			joinOpensAt: now.Add(-10 * time.Minute),
			endsAt:      now.Add(time.Hour),
		},
		{
			name:               "assignment has not started",
			status:             "join_open",
			joinOpensAt:        now.Add(-10 * time.Minute),
			endsAt:             now.Add(time.Hour),
			assigned:           true,
			assignmentStartsAt: now.Add(time.Minute),
		},
		{
			name:               "assignment has ended",
			status:             "join_open",
			joinOpensAt:        now.Add(-10 * time.Minute),
			endsAt:             now.Add(time.Hour),
			assigned:           true,
			assignmentStartsAt: now.Add(-time.Hour),
			assignmentEndsAt:   timePointer(now.Add(-time.Minute)),
		},
		{
			name:        "super admin bypasses assignment scope",
			status:      "join_open",
			joinOpensAt: now.Add(-10 * time.Minute),
			endsAt:      now.Add(time.Hour),
			superAdmin:  true,
			wantAllowed: true,
		},
		{
			name:        "super admin cannot bypass session status",
			status:      "cancelled_academy",
			joinOpensAt: now.Add(-10 * time.Minute),
			endsAt:      now.Add(time.Hour),
			superAdmin:  true,
		},
		{
			name:        "super admin cannot bypass temporal window",
			status:      "join_open",
			joinOpensAt: now.Add(time.Minute),
			endsAt:      now.Add(time.Hour),
			superAdmin:  true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := seedTicketFixture(t, pool, ticketFixtureInput{
				Status:             test.status,
				JoinOpensAt:        test.joinOpensAt,
				EndsAt:             test.endsAt,
				Assigned:           test.assigned,
				AssignmentStartsAt: test.assignmentStartsAt,
				AssignmentEndsAt:   test.assignmentEndsAt,
				SuperAdmin:         test.superAdmin,
				Now:                now,
			})

			result, err := service.Issue(t.Context(), fixture.Principal, fixture.SessionID)
			if test.wantAllowed {
				if err != nil {
					t.Fatalf("Issue() error = %v", err)
				}
				if result.Ticket == "" || !result.ExpiresAt.After(time.Now().UTC()) {
					t.Fatalf("Issue() = %+v, want a live opaque ticket", result)
				}
				if err := service.Authorize(t.Context(), fixture.UserID, fixture.SessionID); err != nil {
					t.Fatalf("authorized classroom rejected: %v", err)
				}
				joined, err := service.Consume(t.Context(), result.Ticket, fixture.UserID)
				if err != nil || joined != fixture.SessionID {
					t.Fatalf("ticket consume: %q, %v", joined, err)
				}
				return
			}
			if !errors.Is(err, ErrForbidden) {
				t.Fatalf("Issue() error = %v, want %v", err, ErrForbidden)
			}
			assertTicketCount(t, pool, fixture.UserID, fixture.SessionID, 0)
			if err := service.Authorize(t.Context(), fixture.UserID, fixture.SessionID); !errors.Is(err, ErrForbidden) {
				t.Fatalf("classroom authorization allowed closed window: %v", err)
			}
		})
	}
}

func TestService_AuthorizeRechecksClassAndAssignmentAfterJoin(t *testing.T) {
	pool := integrationtest.OpenMigratedSchema(t, ticketTestDatabaseURL(t))
	service := NewService(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	fixture := seedTicketFixture(t, pool, ticketFixtureInput{
		Status: "live", JoinOpensAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
		Assigned: true, AssignmentStartsAt: now.Add(-time.Hour), Now: now,
	})
	if err := service.Authorize(t.Context(), fixture.UserID, fixture.SessionID); err != nil {
		t.Fatalf("initial Authorize() = %v", err)
	}
	if _, err := pool.Exec(t.Context(), `UPDATE session_assignments SET ends_at = now() WHERE user_id = $1::uuid`, fixture.UserID); err != nil {
		t.Fatal(err)
	}
	if err := service.Authorize(t.Context(), fixture.UserID, fixture.SessionID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked assignment Authorize() = %v, want forbidden", err)
	}
}

func TestService_ConsumeRevalidatesEligibilityWithoutBurningTicket(t *testing.T) {
	databaseURL := ticketTestDatabaseURL(t)
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	service := NewService(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Run("cancelled after issuance", func(t *testing.T) {
		fixture := seedTicketFixture(t, pool, openAssignedTicketFixture(now))
		issued, err := service.Issue(t.Context(), fixture.Principal, fixture.SessionID)
		if err != nil {
			t.Fatalf("Issue() error = %v", err)
		}
		if _, err := pool.Exec(t.Context(), `
			UPDATE class_sessions
			SET status = 'cancelled_academy', updated_at = now()
			WHERE id = $1::uuid`, fixture.SessionID); err != nil {
			t.Fatalf("cancelling session: %v", err)
		}

		if _, err := service.Consume(t.Context(), issued.Ticket, fixture.UserID); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Consume(cancelled) error = %v, want %v", err, ErrInvalid)
		}
		assertTicketUnconsumed(t, pool, fixture.UserID, fixture.SessionID)

		if _, err := pool.Exec(t.Context(), `
			UPDATE class_sessions
			SET status = 'live', updated_at = now()
			WHERE id = $1::uuid`, fixture.SessionID); err != nil {
			t.Fatalf("reopening session: %v", err)
		}
		gotSessionID, err := service.Consume(t.Context(), issued.Ticket, fixture.UserID)
		if err != nil {
			t.Fatalf("Consume(reopened) error = %v", err)
		}
		if gotSessionID != fixture.SessionID {
			t.Fatalf("Consume(reopened) session = %q, want %q", gotSessionID, fixture.SessionID)
		}
	})

	t.Run("ended after issuance", func(t *testing.T) {
		fixture := seedTicketFixture(t, pool, openAssignedTicketFixture(now))
		issued, err := service.Issue(t.Context(), fixture.Principal, fixture.SessionID)
		if err != nil {
			t.Fatalf("Issue() error = %v", err)
		}
		if _, err := pool.Exec(t.Context(), `
			UPDATE class_sessions
			SET ends_at = now() - INTERVAL '1 second', updated_at = now()
			WHERE id = $1::uuid`, fixture.SessionID); err != nil {
			t.Fatalf("ending session: %v", err)
		}

		if _, err := service.Consume(t.Context(), issued.Ticket, fixture.UserID); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Consume(ended) error = %v, want %v", err, ErrInvalid)
		}
		assertTicketUnconsumed(t, pool, fixture.UserID, fixture.SessionID)
	})

	t.Run("assignment revoked after issuance", func(t *testing.T) {
		fixture := seedTicketFixture(t, pool, openAssignedTicketFixture(now))
		issued, err := service.Issue(t.Context(), fixture.Principal, fixture.SessionID)
		if err != nil {
			t.Fatalf("Issue() error = %v", err)
		}
		if _, err := pool.Exec(t.Context(), `
			UPDATE session_assignments
			SET ends_at = now() - INTERVAL '1 second'
			WHERE class_session_id = $1::uuid AND user_id = $2::uuid`,
			fixture.SessionID,
			fixture.UserID,
		); err != nil {
			t.Fatalf("ending assignment: %v", err)
		}

		if _, err := service.Consume(t.Context(), issued.Ticket, fixture.UserID); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Consume(unassigned) error = %v, want %v", err, ErrInvalid)
		}
		assertTicketUnconsumed(t, pool, fixture.UserID, fixture.SessionID)
	})
}

type ticketFixtureInput struct {
	LegacyIDs          bool
	Status             string
	JoinOpensAt        time.Time
	EndsAt             time.Time
	Assigned           bool
	AssignmentStartsAt time.Time
	AssignmentEndsAt   *time.Time
	SuperAdmin         bool
	Now                time.Time
}

type ticketFixture struct {
	UserID    string
	SessionID string
	Principal authmodel.Principal
}

func openAssignedTicketFixture(now time.Time) ticketFixtureInput {
	return ticketFixtureInput{
		Status:             "join_open",
		JoinOpensAt:        now.Add(-10 * time.Minute),
		EndsAt:             now.Add(time.Hour),
		Assigned:           true,
		AssignmentStartsAt: now.Add(-time.Hour),
		Now:                now,
	}
}

func seedTicketFixture(
	t *testing.T,
	pool *pgxpool.Pool,
	input ticketFixtureInput,
) ticketFixture {
	t.Helper()
	userID := platformid.New()
	sessionID := platformid.New()
	if input.LegacyIDs {
		userID = userID[:14] + "4" + userID[15:]
		sessionID = sessionID[:14] + "4" + sessionID[15:]
	}
	roleGrantID := platformid.New()
	startsAt := input.EndsAt.Add(-time.Hour)
	if input.EndsAt.After(input.Now) {
		startsAt = input.Now.Add(-30 * time.Minute)
	}

	role := authmodel.RoleStudent
	if input.SuperAdmin {
		role = authmodel.RoleSuperAdmin
	}
	_, err := pool.Exec(t.Context(), `
		INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES ($1::uuid, $2, 'Ticket Test User', 'active', $3, $3)`,
		userID,
		userID+"@example.invalid",
		input.Now.Add(-2*time.Hour),
	)
	if err != nil {
		t.Fatalf("seeding ticket user: %v", err)
	}
	_, err = pool.Exec(t.Context(), `
		INSERT INTO role_grants (id, user_id, role, granted_at)
		VALUES ($1::uuid, $2::uuid, $3, $4)`,
		roleGrantID,
		userID,
		role,
		input.Now.Add(-2*time.Hour),
	)
	if err != nil {
		t.Fatalf("seeding ticket role: %v", err)
	}
	_, err = pool.Exec(t.Context(), `
		INSERT INTO class_sessions
			(id, kind, starts_at, ends_at, source_time_zone, source_local_start,
			 join_opens_at, status, created_at, updated_at)
		VALUES
			($1::uuid, 'regular', $2::timestamptz, $3::timestamptz, 'UTC',
			 $2::timestamptz AT TIME ZONE 'UTC', $4::timestamptz, $5,
			 $6::timestamptz, $6::timestamptz)`,
		sessionID,
		startsAt,
		input.EndsAt,
		input.JoinOpensAt,
		input.Status,
		input.Now.Add(-2*time.Hour),
	)
	if err != nil {
		t.Fatalf("seeding ticket session: %v", err)
	}

	if input.Assigned {
		_, err = pool.Exec(t.Context(), `
			INSERT INTO session_assignments
				(id, class_session_id, user_id, role, starts_at, ends_at)
			VALUES ($1::uuid, $2::uuid, $3::uuid, 'student', $4, $5)`,
			platformid.New(),
			sessionID,
			userID,
			input.AssignmentStartsAt,
			input.AssignmentEndsAt,
		)
		if err != nil {
			t.Fatalf("seeding session assignment: %v", err)
		}
	}

	return ticketFixture{
		UserID:    userID,
		SessionID: sessionID,
		Principal: authmodel.Principal{User: authmodel.User{
			ID:     userID,
			Roles:  []authmodel.Role{role},
			Status: "active",
		}},
	}
}

func assertTicketCount(
	t *testing.T,
	pool *pgxpool.Pool,
	userID string,
	sessionID string,
	want int,
) {
	t.Helper()
	var count int
	if err := pool.QueryRow(t.Context(), `
		SELECT COUNT(*)
		FROM realtime_join_tickets
		WHERE user_id = $1::uuid AND class_session_id = $2::uuid`,
		userID,
		sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("counting join tickets: %v", err)
	}
	if count != want {
		t.Fatalf("join ticket count = %d, want %d", count, want)
	}
}

func assertTicketUnconsumed(
	t *testing.T,
	pool *pgxpool.Pool,
	userID string,
	sessionID string,
) {
	t.Helper()
	var unconsumed bool
	if err := pool.QueryRow(t.Context(), `
		SELECT consumed_at IS NULL
		FROM realtime_join_tickets
		WHERE user_id = $1::uuid AND class_session_id = $2::uuid`,
		userID,
		sessionID,
	).Scan(&unconsumed); err != nil {
		t.Fatalf("loading join ticket consumption state: %v", err)
	}
	if !unconsumed {
		t.Fatal("rejected join ticket was consumed")
	}
}

func ticketTestDatabaseURL(t *testing.T) string {
	t.Helper()
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("KG_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL and KG_DATABASE_URL are not set")
	}
	return databaseURL
}

func timePointer(value time.Time) *time.Time { return &value }

func TestService_LegacyAccountAndClassJoin(t *testing.T) {
	pool := integrationtest.OpenMigratedSchema(t, ticketTestDatabaseURL(t))
	service := NewService(pool)
	input := openAssignedTicketFixture(time.Now().UTC().Truncate(time.Microsecond))
	input.LegacyIDs = true
	f := seedTicketFixture(t, pool, input)
	result, err := service.Issue(t.Context(), f.Principal, f.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err = service.Authorize(t.Context(), f.UserID, f.SessionID); err != nil {
		t.Fatal(err)
	}
	joined, err := service.Consume(t.Context(), result.Ticket, f.UserID)
	if err != nil || joined != f.SessionID {
		t.Fatalf("legacy classroom join: %q, %v", joined, err)
	}
	if _, err = pool.Exec(t.Context(), `UPDATE session_assignments SET ends_at=now()-interval '1 second' WHERE class_session_id=$1::uuid AND user_id=$2::uuid`, f.SessionID, f.UserID); err != nil {
		t.Fatal(err)
	}
	if err = service.Authorize(t.Context(), f.UserID, f.SessionID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked legacy assignment allowed: %v", err)
	}
}
