package family

import (
	"context"
	"fmt"
	"sync"
)

// MemoryRepository is a concurrency-safe repository suitable for unit tests
// and local adapters. Save applies optimistic concurrency and appends the audit
// event in the same critical section.
type MemoryRepository struct {
	mu            sync.RWMutex
	relationships map[string]Relationship
	events        map[string][]Event
	eventIDs      map[string]struct{}
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		relationships: make(map[string]Relationship),
		events:        make(map[string][]Event),
		eventIDs:      make(map[string]struct{}),
	}
}

var _ Repository = (*MemoryRepository)(nil)

func (r *MemoryRepository) Relationship(
	ctx context.Context,
	relationshipID string,
) (Relationship, error) {
	if err := ctx.Err(); err != nil {
		return Relationship{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	relationship, ok := r.relationships[relationshipID]
	if !ok {
		return Relationship{}, ErrNotFound
	}
	return relationship.clone(), nil
}

func (r *MemoryRepository) ActiveRelationship(
	ctx context.Context,
	familyID string,
	guardianID string,
	studentID string,
) (Relationship, error) {
	if err := ctx.Err(); err != nil {
		return Relationship{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, relationship := range r.relationships {
		isPair := relationship.FamilyID() == familyID &&
			relationship.GuardianID() == guardianID &&
			relationship.StudentID() == studentID
		if isPair && relationship.IsActive() {
			return relationship.clone(), nil
		}
	}
	return Relationship{}, ErrNotFound
}

func (r *MemoryRepository) Save(
	ctx context.Context,
	relationship Relationship,
	event Event,
	expectedRevision uint64,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := relationship.validate(); err != nil {
		return err
	}
	if err := validateEvent(relationship, event); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	current, exists := r.relationships[relationship.ID()]
	if expectedRevision == 0 {
		if exists || relationship.Revision() != 1 {
			return ErrConflict
		}
		for _, candidate := range r.relationships {
			isSamePair := candidate.FamilyID() == relationship.FamilyID() &&
				candidate.GuardianID() == relationship.GuardianID() &&
				candidate.StudentID() == relationship.StudentID()
			if isSamePair && candidate.IsActive() {
				return ErrConflict
			}
		}
	} else {
		if !exists || current.Revision() != expectedRevision {
			return ErrConflict
		}
		if relationship.Revision() != expectedRevision+1 {
			return fmt.Errorf("%w: revision must advance by one", ErrConflict)
		}
	}
	if _, exists := r.eventIDs[event.ID]; exists {
		return fmt.Errorf("%w: event id already exists", ErrConflict)
	}

	r.relationships[relationship.ID()] = relationship.clone()
	r.events[relationship.ID()] = append(r.events[relationship.ID()], cloneEvent(event))
	r.eventIDs[event.ID] = struct{}{}
	return nil
}

func (r *MemoryRepository) Events(ctx context.Context, relationshipID string) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, exists := r.relationships[relationshipID]; !exists {
		return nil, ErrNotFound
	}
	events := make([]Event, 0, len(r.events[relationshipID]))
	for _, event := range r.events[relationshipID] {
		events = append(events, cloneEvent(event))
	}
	return events, nil
}

func validateEvent(relationship Relationship, event Event) error {
	if event.ID == "" || event.ActorID == "" || event.OccurredAt.IsZero() {
		return fmt.Errorf("%w: event metadata is required", ErrInvalidCommand)
	}
	if event.RelationshipID != relationship.ID() || event.Revision != relationship.Revision() {
		return fmt.Errorf("%w: event does not match relationship revision", ErrInvalidCommand)
	}

	switch event.Kind {
	case EventKindGranted, EventKindPermissionsReplaced, EventKindRevoked:
		return nil
	default:
		return fmt.Errorf("%w: unsupported event kind %q", ErrInvalidCommand, event.Kind)
	}
}
