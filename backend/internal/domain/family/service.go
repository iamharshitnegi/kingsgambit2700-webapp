package family

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"
)

var (
	ErrNotFound         = errors.New("family: relationship not found")
	ErrConflict         = errors.New("family: relationship conflict")
	ErrPermissionDenied = errors.New("family: permission denied")
	ErrInvalidCommand   = errors.New("family: invalid command")
)

type EventKind string

const (
	EventKindUnknown             EventKind = ""
	EventKindGranted             EventKind = "granted"
	EventKindPermissionsReplaced EventKind = "permissions_replaced"
	EventKindRevoked             EventKind = "revoked"
)

// Event is append-only evidence of a relationship or permission change.
type Event struct {
	ID                  string
	RelationshipID      string
	Revision            uint64
	Kind                EventKind
	ActorID             string
	OccurredAt          time.Time
	PreviousPermissions []Permission
	Permissions         []Permission
	Reason              string
}

// Repository must atomically persist a relationship revision and its event.
// expectedRevision is zero for creation and the current revision for updates.
type Repository interface {
	Relationship(ctx context.Context, relationshipID string) (Relationship, error)
	ActiveRelationship(
		ctx context.Context,
		familyID string,
		guardianID string,
		studentID string,
	) (Relationship, error)
	Save(
		ctx context.Context,
		relationship Relationship,
		event Event,
		expectedRevision uint64,
	) error
	Events(ctx context.Context, relationshipID string) ([]Event, error)
}

// Authorizer decides whether an actor may change guardian relationships for a
// family. Implementations may combine staff permissions with primary-guardian
// grants. Keeping this decision in the service prevents a transport from
// accidentally becoming the authorization boundary.
type Authorizer interface {
	AuthorizeRelationshipManagement(
		ctx context.Context,
		actorID string,
		familyID string,
	) error
}

type Service struct {
	repository Repository
	authorizer Authorizer
}

func NewService(repository Repository, authorizer Authorizer) (*Service, error) {
	if repository == nil || authorizer == nil {
		return nil, fmt.Errorf("%w: repository and authorizer are required", ErrInvalidCommand)
	}
	return &Service{repository: repository, authorizer: authorizer}, nil
}

type GrantCommand struct {
	RelationshipID string
	EventID        string
	FamilyID       string
	GuardianID     string
	StudentID      string
	Permissions    []Permission
	ActorID        string
	OccurredAt     time.Time
}

func (s *Service) Grant(ctx context.Context, command GrantCommand) (Relationship, error) {
	if err := ctx.Err(); err != nil {
		return Relationship{}, err
	}
	hasMissingIdentity := command.RelationshipID == "" || command.EventID == "" || command.FamilyID == "" ||
		command.GuardianID == "" || command.StudentID == "" || command.ActorID == "" ||
		command.OccurredAt.IsZero()
	if hasMissingIdentity {
		return Relationship{}, fmt.Errorf("%w: ids, actor and time are required", ErrInvalidCommand)
	}
	if err := s.authorizer.AuthorizeRelationshipManagement(
		ctx,
		command.ActorID,
		command.FamilyID,
	); err != nil {
		return Relationship{}, fmt.Errorf("authorizing relationship grant: %w", err)
	}

	_, err := s.repository.ActiveRelationship(
		ctx,
		command.FamilyID,
		command.GuardianID,
		command.StudentID,
	)
	if err == nil {
		return Relationship{}, fmt.Errorf("%w: active relationship already exists", ErrConflict)
	}
	if !errors.Is(err, ErrNotFound) {
		return Relationship{}, fmt.Errorf("finding active relationship: %w", err)
	}

	relationship, err := NewRelationship(NewRelationshipParams{
		ID:          command.RelationshipID,
		FamilyID:    command.FamilyID,
		GuardianID:  command.GuardianID,
		StudentID:   command.StudentID,
		Permissions: command.Permissions,
		GrantedBy:   command.ActorID,
		GrantedAt:   command.OccurredAt,
	})
	if err != nil {
		return Relationship{}, fmt.Errorf("creating relationship: %w", err)
	}

	event := Event{
		ID:             command.EventID,
		RelationshipID: relationship.ID(),
		Revision:       relationship.Revision(),
		Kind:           EventKindGranted,
		ActorID:        command.ActorID,
		OccurredAt:     command.OccurredAt,
		Permissions:    relationship.Permissions(),
	}
	if err := s.repository.Save(ctx, relationship, event, 0); err != nil {
		return Relationship{}, fmt.Errorf("saving relationship grant: %w", err)
	}

	return relationship, nil
}

type ReplacePermissionsCommand struct {
	FamilyID         string
	RelationshipID   string
	EventID          string
	ExpectedRevision uint64
	Permissions      []Permission
	ActorID          string
	Reason           string
	OccurredAt       time.Time
}

func (s *Service) ReplacePermissions(
	ctx context.Context,
	command ReplacePermissionsCommand,
) (Relationship, error) {
	if err := validateChangeCommand(
		command.FamilyID,
		command.RelationshipID,
		command.EventID,
		command.ActorID,
		command.Reason,
		command.OccurredAt,
		command.ExpectedRevision,
	); err != nil {
		return Relationship{}, err
	}
	if err := s.authorizer.AuthorizeRelationshipManagement(
		ctx,
		command.ActorID,
		command.FamilyID,
	); err != nil {
		return Relationship{}, fmt.Errorf("authorizing permission change: %w", err)
	}

	current, err := s.repository.Relationship(ctx, command.RelationshipID)
	if err != nil {
		return Relationship{}, fmt.Errorf("loading relationship: %w", err)
	}
	if current.FamilyID() != command.FamilyID {
		return Relationship{}, ErrNotFound
	}
	if current.Revision() != command.ExpectedRevision {
		return Relationship{}, fmt.Errorf(
			"%w: expected revision %d, current %d",
			ErrConflict,
			command.ExpectedRevision,
			current.Revision(),
		)
	}

	updated, err := current.WithPermissions(command.Permissions)
	if err != nil {
		return Relationship{}, fmt.Errorf("replacing permissions: %w", err)
	}
	event := Event{
		ID:                  command.EventID,
		RelationshipID:      updated.ID(),
		Revision:            updated.Revision(),
		Kind:                EventKindPermissionsReplaced,
		ActorID:             command.ActorID,
		OccurredAt:          command.OccurredAt,
		PreviousPermissions: current.Permissions(),
		Permissions:         updated.Permissions(),
		Reason:              command.Reason,
	}
	if err := s.repository.Save(ctx, updated, event, current.Revision()); err != nil {
		return Relationship{}, fmt.Errorf("saving permission change: %w", err)
	}

	return updated, nil
}

type RevokeCommand struct {
	FamilyID         string
	RelationshipID   string
	EventID          string
	ExpectedRevision uint64
	ActorID          string
	Reason           string
	OccurredAt       time.Time
}

func (s *Service) Revoke(ctx context.Context, command RevokeCommand) (Relationship, error) {
	if err := validateChangeCommand(
		command.FamilyID,
		command.RelationshipID,
		command.EventID,
		command.ActorID,
		command.Reason,
		command.OccurredAt,
		command.ExpectedRevision,
	); err != nil {
		return Relationship{}, err
	}
	if err := s.authorizer.AuthorizeRelationshipManagement(
		ctx,
		command.ActorID,
		command.FamilyID,
	); err != nil {
		return Relationship{}, fmt.Errorf("authorizing relationship revocation: %w", err)
	}

	current, err := s.repository.Relationship(ctx, command.RelationshipID)
	if err != nil {
		return Relationship{}, fmt.Errorf("loading relationship: %w", err)
	}
	if current.FamilyID() != command.FamilyID {
		return Relationship{}, ErrNotFound
	}
	if current.Revision() != command.ExpectedRevision {
		return Relationship{}, fmt.Errorf(
			"%w: expected revision %d, current %d",
			ErrConflict,
			command.ExpectedRevision,
			current.Revision(),
		)
	}

	updated, err := current.Revoke(command.ActorID, command.Reason, command.OccurredAt)
	if err != nil {
		return Relationship{}, fmt.Errorf("revoking relationship: %w", err)
	}
	event := Event{
		ID:                  command.EventID,
		RelationshipID:      updated.ID(),
		Revision:            updated.Revision(),
		Kind:                EventKindRevoked,
		ActorID:             command.ActorID,
		OccurredAt:          command.OccurredAt,
		PreviousPermissions: current.Permissions(),
		Permissions:         updated.Permissions(),
		Reason:              command.Reason,
	}
	if err := s.repository.Save(ctx, updated, event, current.Revision()); err != nil {
		return Relationship{}, fmt.Errorf("saving relationship revocation: %w", err)
	}

	return updated, nil
}

func (s *Service) Authorize(
	ctx context.Context,
	familyID string,
	guardianID string,
	studentID string,
	permission Permission,
) error {
	if !permission.valid() {
		return fmt.Errorf("%w: unsupported permission %q", ErrInvalidCommand, permission)
	}

	relationship, err := s.repository.ActiveRelationship(ctx, familyID, guardianID, studentID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPermissionDenied
		}
		return fmt.Errorf("loading active relationship: %w", err)
	}
	if !relationship.Allows(permission) {
		return ErrPermissionDenied
	}

	return nil
}

func validateChangeCommand(
	familyID string,
	relationshipID string,
	eventID string,
	actorID string,
	reason string,
	occurredAt time.Time,
	expectedRevision uint64,
) error {
	if familyID == "" || relationshipID == "" || eventID == "" || actorID == "" || reason == "" {
		return fmt.Errorf("%w: ids, actor and reason are required", ErrInvalidCommand)
	}
	if occurredAt.IsZero() || expectedRevision == 0 {
		return fmt.Errorf("%w: time and expected revision are required", ErrInvalidCommand)
	}
	return nil
}

func cloneEvent(event Event) Event {
	cloned := event
	cloned.PreviousPermissions = slices.Clone(event.PreviousPermissions)
	cloned.Permissions = slices.Clone(event.Permissions)
	return cloned
}
