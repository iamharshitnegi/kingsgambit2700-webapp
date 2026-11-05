package classroom

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// Rules isolates the chess library from the authoritative classroom state
// machine. ApplyMove validates legality and returns canonical notation/state.
type Rules interface {
	ApplyMove(ctx context.Context, fen string, move string) (MoveResult, error)
	ValidatePosition(ctx context.Context, fen string, moves []string) error
}

type MoveResult struct {
	FEN            string
	NormalizedMove string
}

// Repository must commit the event and next state atomically only when the
// stored revision equals ExpectedRevision. Redis publication happens after a
// successful commit and is deliberately outside this interface.
type Repository interface {
	Create(ctx context.Context, state State) error
	State(ctx context.Context, sessionID string) (State, error)
	Commit(ctx context.Context, commit Commit) (State, error)
	Recovery(ctx context.Context, sessionID string, afterRevision uint64) (Recovery, error)
}

type Service struct {
	repository Repository
	rules      Rules
}

func NewService(repository Repository, rules Rules) (*Service, error) {
	if repository == nil || rules == nil {
		return nil, fmt.Errorf("%w: repository and rules are required", ErrInvalidBoard)
	}
	return &Service{repository: repository, rules: rules}, nil
}

func (s *Service) Create(ctx context.Context, params NewStateParams) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	state, err := NewState(params)
	if err != nil {
		return State{}, err
	}
	if err := s.rules.ValidatePosition(ctx, state.FEN, state.Moves); err != nil {
		return State{}, fmt.Errorf("validating initial position: %w", err)
	}
	if err := s.repository.Create(ctx, state); err != nil {
		return State{}, fmt.Errorf("creating board: %w", err)
	}
	return cloneState(state), nil
}

type MoveCommand struct {
	EventID          string
	SessionID        string
	Actor            Actor
	Move             string
	ExpectedRevision uint64
	OccurredAt       time.Time
}

type Accepted struct {
	State State
	Event Event
}

func (s *Service) SubmitMove(ctx context.Context, command MoveCommand) (Accepted, error) {
	if command.EventID == "" || command.Move == "" || command.OccurredAt.IsZero() {
		return Accepted{}, fmt.Errorf("%w: move metadata is required", ErrInvalidBoard)
	}
	current, err := s.loadExpected(ctx, command.SessionID, command.ExpectedRevision)
	if err != nil {
		return Accepted{}, err
	}
	if current.Status != BoardStatusOpen {
		return Accepted{}, ErrBoardClosed
	}
	if len(current.Moves) >= maxLessonMoves {
		return Accepted{}, fmt.Errorf("%w: load a fresh position after 512 moves", ErrInvalidBoard)
	}
	if !current.CanMove(command.Actor) {
		return Accepted{}, ErrMoveNotAuthorized
	}

	result, err := s.rules.ApplyMove(ctx, current.FEN, command.Move)
	if err != nil {
		return Accepted{}, fmt.Errorf("applying move: %w", err)
	}
	if result.FEN == "" || result.NormalizedMove == "" {
		return Accepted{}, fmt.Errorf("%w: rules returned an empty move result", ErrInvalidBoard)
	}

	next := cloneState(current)
	next.FEN = result.FEN
	next.Moves = append(next.Moves, result.NormalizedMove)
	next.Lesson.Positions = append(next.Lesson.Positions, result.FEN)
	next.Lesson.Annotations = []Annotation{}
	next.Revision++
	event := Event{
		ID:         command.EventID,
		SessionID:  current.SessionID,
		Revision:   next.Revision,
		Kind:       EventKindMoveApplied,
		ActorID:    command.Actor.ID,
		OccurredAt: command.OccurredAt,
		Move:       result.NormalizedMove,
		FEN:        result.FEN,
	}
	event.State = statePointer(next)
	committed, err := s.repository.Commit(ctx, Commit{
		ExpectedRevision: current.Revision,
		State:            next,
		Event:            event,
	})
	if err != nil {
		return Accepted{}, fmt.Errorf("committing move: %w", err)
	}
	return Accepted{State: committed, Event: event}, nil
}

type ModeCommand struct {
	EventID          string
	SessionID        string
	Actor            Actor
	Mode             Mode
	ActiveActorID    string
	ExpectedRevision uint64
	OccurredAt       time.Time
}

func (s *Service) SetMode(ctx context.Context, command ModeCommand) (Accepted, error) {
	if command.EventID == "" || !command.Mode.valid() || command.OccurredAt.IsZero() {
		return Accepted{}, fmt.Errorf("%w: mode metadata is required", ErrInvalidBoard)
	}
	current, err := s.loadExpected(ctx, command.SessionID, command.ExpectedRevision)
	if err != nil {
		return Accepted{}, err
	}
	if current.Status != BoardStatusOpen {
		return Accepted{}, ErrBoardClosed
	}
	if !command.Actor.canControl() {
		return Accepted{}, ErrControlNotAuthorized
	}

	next := cloneState(current)
	next.Mode = command.Mode
	next.ActiveActorID = command.ActiveActorID
	next.Revision++
	if err := next.Validate(); err != nil {
		return Accepted{}, err
	}
	event := Event{
		ID:            command.EventID,
		SessionID:     current.SessionID,
		Revision:      next.Revision,
		Kind:          EventKindModeChanged,
		ActorID:       command.Actor.ID,
		OccurredAt:    command.OccurredAt,
		Mode:          command.Mode,
		ActiveActorID: command.ActiveActorID,
	}
	event.State = statePointer(next)
	committed, err := s.repository.Commit(ctx, Commit{
		ExpectedRevision: current.Revision,
		State:            next,
		Event:            event,
	})
	if err != nil {
		return Accepted{}, fmt.Errorf("committing mode change: %w", err)
	}
	return Accepted{State: committed, Event: event}, nil
}

type PositionCommand struct {
	EventID          string
	SessionID        string
	Actor            Actor
	FEN              string
	InitialFEN       string
	Moves            []string
	ExpectedRevision uint64
	OccurredAt       time.Time
}

func (s *Service) LoadPosition(ctx context.Context, command PositionCommand) (Accepted, error) {
	if command.EventID == "" || command.FEN == "" || command.OccurredAt.IsZero() {
		return Accepted{}, fmt.Errorf("%w: position metadata is required", ErrInvalidBoard)
	}
	current, err := s.loadExpected(ctx, command.SessionID, command.ExpectedRevision)
	if err != nil {
		return Accepted{}, err
	}
	if current.Status != BoardStatusOpen {
		return Accepted{}, ErrBoardClosed
	}
	if !command.Actor.canControl() {
		return Accepted{}, ErrControlNotAuthorized
	}
	moves := slices.Clone(command.Moves)
	if moves == nil {
		moves = []string{}
	}
	if err := s.rules.ValidatePosition(ctx, command.FEN, moves); err != nil {
		return Accepted{}, fmt.Errorf("validating position: %w", err)
	}

	next := cloneState(current)
	next.FEN = command.FEN
	next.Moves = moves
	lesson, err := s.positionLesson(ctx, command.InitialFEN, command.FEN, moves)
	if err != nil {
		return Accepted{}, err
	}
	lesson.Checkpoints = next.Lesson.Checkpoints
	lesson.CountdownEndsAt = next.Lesson.CountdownEndsAt
	lesson.CountdownLabel = next.Lesson.CountdownLabel
	next.Lesson = lesson
	next.FEN = lesson.Positions[len(lesson.Positions)-1]
	next.Revision++
	event := Event{
		ID:         command.EventID,
		SessionID:  current.SessionID,
		Revision:   next.Revision,
		Kind:       EventKindPositionLoaded,
		ActorID:    command.Actor.ID,
		OccurredAt: command.OccurredAt,
		FEN:        next.FEN,
	}
	event.State = statePointer(next)
	committed, err := s.repository.Commit(ctx, Commit{
		ExpectedRevision: current.Revision,
		State:            next,
		Event:            event,
	})
	if err != nil {
		return Accepted{}, fmt.Errorf("committing position: %w", err)
	}
	return Accepted{State: committed, Event: event}, nil
}

type CloseCommand struct {
	EventID          string
	SessionID        string
	Actor            Actor
	ExpectedRevision uint64
	OccurredAt       time.Time
}

func (s *Service) Close(ctx context.Context, command CloseCommand) (Accepted, error) {
	if command.EventID == "" || command.OccurredAt.IsZero() {
		return Accepted{}, fmt.Errorf("%w: close metadata is required", ErrInvalidBoard)
	}
	current, err := s.loadExpected(ctx, command.SessionID, command.ExpectedRevision)
	if err != nil {
		return Accepted{}, err
	}
	if current.Status != BoardStatusOpen {
		return Accepted{}, ErrBoardClosed
	}
	if !command.Actor.canControl() {
		return Accepted{}, ErrControlNotAuthorized
	}

	next := cloneState(current)
	next.Status = BoardStatusClosed
	next.Revision++
	event := Event{
		ID:         command.EventID,
		SessionID:  current.SessionID,
		Revision:   next.Revision,
		Kind:       EventKindBoardClosed,
		ActorID:    command.Actor.ID,
		OccurredAt: command.OccurredAt,
	}
	event.State = statePointer(next)
	committed, err := s.repository.Commit(ctx, Commit{
		ExpectedRevision: current.Revision,
		State:            next,
		Event:            event,
	})
	if err != nil {
		return Accepted{}, fmt.Errorf("committing board close: %w", err)
	}
	return Accepted{State: committed, Event: event}, nil
}

func (s *Service) Reconnect(
	ctx context.Context,
	sessionID string,
	afterRevision uint64,
) (Recovery, error) {
	recovery, err := s.repository.Recovery(ctx, sessionID, afterRevision)
	if err != nil {
		return Recovery{}, fmt.Errorf("recovering board: %w", err)
	}
	// Historical position events did not include their move list. A current
	// canonical snapshot avoids trying to reconstruct missing legacy history.
	for _, event := range recovery.Events {
		if event.Kind == EventKindPositionLoaded && event.State == nil {
			state, err := s.repository.State(ctx, sessionID)
			if err != nil {
				return Recovery{}, err
			}
			return Recovery{Snapshot: state, Events: []Event{}}, nil
		}
	}
	return recovery, nil
}

func (s *Service) loadExpected(
	ctx context.Context,
	sessionID string,
	expectedRevision uint64,
) (State, error) {
	if sessionID == "" {
		return State{}, fmt.Errorf("%w: session id is required", ErrInvalidBoard)
	}
	current, err := s.repository.State(ctx, sessionID)
	if err != nil {
		return State{}, fmt.Errorf("loading board: %w", err)
	}
	if current.Revision != expectedRevision {
		return State{}, &RevisionConflictError{
			Expected: expectedRevision,
			Current:  current.Revision,
		}
	}
	return current, nil
}
