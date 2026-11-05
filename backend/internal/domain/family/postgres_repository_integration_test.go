//go:build integration

package family

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresRepository_RelationshipLifecycle(t *testing.T) {
	databaseURL := os.Getenv("KG_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)

	guardianID := platformid.New()
	studentUserID := platformid.New()
	studentID := platformid.New()
	familyID := platformid.New()
	relationshipID := platformid.New()
	eventIDs := []string{platformid.New(), platformid.New(), platformid.New()}
	now := time.Now().UTC().Truncate(time.Microsecond)
	seedFamilyRepositoryTest(
		t,
		pool,
		guardianID,
		studentUserID,
		studentID,
		familyID,
		now,
	)
	repository, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatalf("NewPostgresRepository() error = %v", err)
	}
	service, err := NewService(repository, integrationRelationshipAuthorizer{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	relationship, err := service.Grant(t.Context(), GrantCommand{
		RelationshipID: relationshipID,
		EventID:        eventIDs[0],
		FamilyID:       familyID,
		GuardianID:     guardianID,
		StudentID:      studentID,
		Permissions: []Permission{
			PermissionViewSchedule,
			PermissionViewProgress,
		},
		ActorID:    guardianID,
		OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	relationship, err = service.ReplacePermissions(t.Context(), ReplacePermissionsCommand{
		FamilyID:         familyID,
		RelationshipID:   relationship.ID(),
		EventID:          eventIDs[1],
		ExpectedRevision: relationship.Revision(),
		Permissions:      []Permission{PermissionViewProgress},
		ActorID:          guardianID,
		Reason:           "integration permission change",
		OccurredAt:       now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ReplacePermissions() error = %v", err)
	}
	loaded, err := repository.Relationship(t.Context(), relationshipID)
	if err != nil {
		t.Fatalf("Relationship() error = %v", err)
	}
	if loaded.Revision() != 2 || !loaded.Allows(PermissionViewProgress) {
		t.Fatalf("Relationship() = %+v", loaded.Record())
	}

	relationship, err = service.Revoke(t.Context(), RevokeCommand{
		FamilyID:         familyID,
		RelationshipID:   relationship.ID(),
		EventID:          eventIDs[2],
		ExpectedRevision: relationship.Revision(),
		ActorID:          guardianID,
		Reason:           "integration revocation",
		OccurredAt:       now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if relationship.IsActive() || relationship.Revision() != 3 {
		t.Fatalf("Revoke() = %+v", relationship.Record())
	}
	events, err := repository.Events(t.Context(), relationshipID)
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(events) != 3 || events[2].Reason != "integration revocation" {
		t.Fatalf("Events() = %+v", events)
	}
}

type integrationRelationshipAuthorizer struct{}

func (integrationRelationshipAuthorizer) AuthorizeRelationshipManagement(
	context.Context,
	string,
	string,
) error {
	return nil
}

func seedFamilyRepositoryTest(
	t *testing.T,
	pool *pgxpool.Pool,
	guardianID string,
	studentUserID string,
	studentID string,
	familyID string,
	now time.Time,
) {
	t.Helper()
	_, err := pool.Exec(t.Context(), `
		INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES
			($1::uuid, $2, 'Integration Guardian', 'active', $5, $5),
			($3::uuid, $4, 'Integration Student', 'active', $5, $5)`,
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
		INSERT INTO families (id, display_name, created_at, updated_at)
		VALUES ($1::uuid, 'Integration Family', $2, $2)`,
		familyID,
		now,
	)
	if err != nil {
		t.Fatalf("inserting family: %v", err)
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
