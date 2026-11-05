package family

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestService_RelationshipLifecycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := NewMemoryRepository()
	service, err := NewService(repository, allowRelationshipManagement{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)

	relationship, err := service.Grant(ctx, GrantCommand{
		RelationshipID: "relationship-1",
		EventID:        "event-1",
		FamilyID:       "family-1",
		GuardianID:     "guardian-1",
		StudentID:      "student-1",
		Permissions: []Permission{
			PermissionViewProgress,
			PermissionViewSchedule,
		},
		ActorID:    "guardian-1",
		OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	if relationship.Revision() != 1 {
		t.Fatalf("Grant() revision = %d, want 1", relationship.Revision())
	}
	if err := service.Authorize(
		ctx,
		"family-1",
		"guardian-1",
		"student-1",
		PermissionViewProgress,
	); err != nil {
		t.Fatalf("Authorize(view progress) error = %v", err)
	}
	if err := service.Authorize(
		ctx,
		"family-1",
		"guardian-1",
		"student-1",
		PermissionManageBilling,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize(manage billing) error = %v, want %v", err, ErrPermissionDenied)
	}

	updated, err := service.ReplacePermissions(ctx, ReplacePermissionsCommand{
		FamilyID:         "family-1",
		RelationshipID:   relationship.ID(),
		EventID:          "event-2",
		ExpectedRevision: relationship.Revision(),
		Permissions:      []Permission{PermissionManageSchedule},
		ActorID:          "guardian-1",
		Reason:           "secondary guardian schedule access",
		OccurredAt:       now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("ReplacePermissions() error = %v", err)
	}
	if err := service.Authorize(
		ctx,
		"family-1",
		"guardian-1",
		"student-1",
		PermissionViewProgress,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize(removed permission) error = %v, want %v", err, ErrPermissionDenied)
	}

	revoked, err := service.Revoke(ctx, RevokeCommand{
		FamilyID:         "family-1",
		RelationshipID:   updated.ID(),
		EventID:          "event-3",
		ExpectedRevision: updated.Revision(),
		ActorID:          "guardian-1",
		Reason:           "guardian access removed",
		OccurredAt:       now.Add(2 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if revoked.IsActive() {
		t.Fatal("Revoke() relationship remains active")
	}
	if err := service.Authorize(
		ctx,
		"family-1",
		"guardian-1",
		"student-1",
		PermissionManageSchedule,
	); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Authorize(after revoke) error = %v, want %v", err, ErrPermissionDenied)
	}

	events, err := repository.Events(ctx, relationship.ID())
	if err != nil {
		t.Fatalf("Events() error = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("Events() length = %d, want 3", len(events))
	}
	if events[0].Kind != EventKindGranted ||
		events[1].Kind != EventKindPermissionsReplaced ||
		events[2].Kind != EventKindRevoked {
		t.Fatalf("Events() kinds = %q, %q, %q", events[0].Kind, events[1].Kind, events[2].Kind)
	}
}

func TestService_RejectsStalePermissionChange(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := NewMemoryRepository()
	service, err := NewService(repository, allowRelationshipManagement{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	relationship, err := service.Grant(ctx, GrantCommand{
		RelationshipID: "relationship-1",
		EventID:        "event-1",
		FamilyID:       "family-1",
		GuardianID:     "guardian-1",
		StudentID:      "student-1",
		Permissions:    []Permission{PermissionViewProfile},
		ActorID:        "guardian-1",
		OccurredAt:     now,
	})
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	_, err = service.ReplacePermissions(ctx, ReplacePermissionsCommand{
		FamilyID:         "family-1",
		RelationshipID:   relationship.ID(),
		EventID:          "event-2",
		ExpectedRevision: relationship.Revision() + 1,
		Permissions:      []Permission{PermissionViewProgress},
		ActorID:          "guardian-1",
		Reason:           "stale browser request",
		OccurredAt:       now.Add(time.Minute),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("ReplacePermissions() error = %v, want %v", err, ErrConflict)
	}
}

func TestService_RejectsUnauthorizedRelationshipManagement(t *testing.T) {
	t.Parallel()

	service, err := NewService(NewMemoryRepository(), denyRelationshipManagement{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.Grant(t.Context(), GrantCommand{
		RelationshipID: "relationship-1",
		EventID:        "event-1",
		FamilyID:       "family-1",
		GuardianID:     "guardian-1",
		StudentID:      "student-1",
		Permissions:    []Permission{PermissionViewProfile},
		ActorID:        "guardian-1",
		OccurredAt:     time.Now().UTC(),
	})
	if !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("Grant() error = %v, want %v", err, ErrPermissionDenied)
	}
}

type allowRelationshipManagement struct{}

func (allowRelationshipManagement) AuthorizeRelationshipManagement(
	context.Context,
	string,
	string,
) error {
	return nil
}

type denyRelationshipManagement struct{}

func (denyRelationshipManagement) AuthorizeRelationshipManagement(
	context.Context,
	string,
	string,
) error {
	return ErrPermissionDenied
}

func TestRelationship_PermissionsAreDefensivelyCopied(t *testing.T) {
	t.Parallel()

	permissions := []Permission{PermissionViewProfile}
	relationship, err := NewRelationship(NewRelationshipParams{
		ID:          "relationship-1",
		FamilyID:    "family-1",
		GuardianID:  "guardian-1",
		StudentID:   "student-1",
		Permissions: permissions,
		GrantedBy:   "guardian-1",
		GrantedAt:   time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("NewRelationship() error = %v", err)
	}

	permissions[0] = PermissionManageBilling
	returned := relationship.Permissions()
	returned[0] = PermissionManageBilling
	if !relationship.Allows(PermissionViewProfile) {
		t.Fatal("external slice mutation changed relationship permissions")
	}
}
