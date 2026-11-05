package classroom

import (
	"context"
	"fmt"
	"sync"
)

const DefaultSnapshotInterval uint64 = 20

type boardStream struct {
	state     State
	events    []Event
	snapshots []State
}

type MemoryRepository struct {
	mu               sync.RWMutex
	snapshotInterval uint64
	streams          map[string]*boardStream
	eventIDs         map[string]struct{}
}

func NewMemoryRepository(snapshotInterval uint64) (*MemoryRepository, error) {
	if snapshotInterval == 0 {
		return nil, fmt.Errorf("%w: snapshot interval must be positive", ErrInvalidBoard)
	}
	return &MemoryRepository{
		snapshotInterval: snapshotInterval,
		streams:          make(map[string]*boardStream),
		eventIDs:         make(map[string]struct{}),
	}, nil
}

var _ Repository = (*MemoryRepository)(nil)

func (r *MemoryRepository) Create(ctx context.Context, state State) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := state.Validate(); err != nil {
		return err
	}
	if state.Revision != 0 {
		return fmt.Errorf("%w: initial revision must be zero", ErrInvalidBoard)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.streams[state.SessionID]; exists {
		return ErrBoardExists
	}
	initial := cloneState(state)
	r.streams[state.SessionID] = &boardStream{
		state:     initial,
		events:    []Event{},
		snapshots: []State{cloneState(initial)},
	}
	return nil
}

func (r *MemoryRepository) State(ctx context.Context, sessionID string) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	stream, exists := r.streams[sessionID]
	if !exists {
		return State{}, ErrBoardNotFound
	}
	return cloneState(stream.state), nil
}

func (r *MemoryRepository) Commit(ctx context.Context, commit Commit) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	if err := commit.State.Validate(); err != nil {
		return State{}, err
	}
	if err := commit.Event.validate(commit.State); err != nil {
		return State{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	stream, exists := r.streams[commit.State.SessionID]
	if !exists {
		return State{}, ErrBoardNotFound
	}
	if stream.state.Revision != commit.ExpectedRevision {
		return State{}, &RevisionConflictError{
			Expected: commit.ExpectedRevision,
			Current:  stream.state.Revision,
		}
	}
	if commit.State.Revision != commit.ExpectedRevision+1 ||
		commit.Event.Revision != commit.State.Revision {
		return State{}, fmt.Errorf("%w: revision must advance by one", ErrRevisionConflict)
	}
	if err := validateTransition(stream.state, commit.State, commit.Event); err != nil {
		return State{}, err
	}
	if _, exists := r.eventIDs[commit.Event.ID]; exists {
		return State{}, ErrEventConflict
	}

	stream.state = cloneState(commit.State)
	stream.events = append(stream.events, cloneEvent(commit.Event))
	r.eventIDs[commit.Event.ID] = struct{}{}
	if commit.State.Revision%r.snapshotInterval == 0 {
		stream.snapshots = append(stream.snapshots, cloneState(commit.State))
	}
	return cloneState(stream.state), nil
}

func (r *MemoryRepository) Recovery(
	ctx context.Context,
	sessionID string,
	afterRevision uint64,
) (Recovery, error) {
	if err := ctx.Err(); err != nil {
		return Recovery{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	stream, exists := r.streams[sessionID]
	if !exists {
		return Recovery{}, ErrBoardNotFound
	}
	if afterRevision > stream.state.Revision {
		return Recovery{}, &RevisionConflictError{
			Expected: afterRevision,
			Current:  stream.state.Revision,
		}
	}

	snapshot := stream.snapshots[len(stream.snapshots)-1]
	events := make([]Event, 0, len(stream.events))
	for _, event := range stream.events {
		if event.Revision > snapshot.Revision {
			events = append(events, cloneEvent(event))
		}
	}
	return Recovery{Snapshot: cloneState(snapshot), Events: events}, nil
}
