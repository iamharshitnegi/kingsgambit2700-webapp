package classroom

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

const initialFEN = "rn1qkbnr/ppp1pppp/3p4/8/8/3P4/PPP1PPPP/RNBQKBNR w KQkq - 0 2"

func TestPrivateResponseNeverWritesStudentAnswerToSharedBoard(t *testing.T) {
	repository, err := NewMemoryRepository(DefaultSnapshotInterval)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repository, &fakeRules{})
	if err != nil {
		t.Fatal(err)
	}
	state, err := service.Create(t.Context(), NewStateParams{
		SessionID: "private-session", FEN: initialFEN, Mode: ModePrivateResponse, ActiveActorID: "student-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.SubmitMove(t.Context(), MoveCommand{
		EventID: "private-answer", SessionID: state.SessionID,
		Actor: Actor{ID: "student-1", Role: ActorRoleStudent},
		Move:  "e2e4", OccurredAt: time.Now(),
	})
	if !errors.Is(err, ErrMoveNotAuthorized) {
		t.Fatalf("private student move error = %v, want unauthorized", err)
	}
	current, err := repository.State(t.Context(), state.SessionID)
	if err != nil || current.Revision != 0 || len(current.Moves) != 0 {
		t.Fatalf("private answer changed shared state: %+v, %v", current, err)
	}
}

type fakeRules struct {
	calls int
}

func (r *fakeRules) ApplyMove(
	ctx context.Context,
	fen string,
	move string,
) (MoveResult, error) {
	if err := ctx.Err(); err != nil {
		return MoveResult{}, err
	}
	r.calls++
	if move == "illegal" {
		return MoveResult{}, ErrIllegalMove
	}
	return MoveResult{
		FEN:            fmt.Sprintf("%s after %s", fen, move),
		NormalizedMove: move,
	}, nil
}

func (r *fakeRules) ValidatePosition(ctx context.Context, fen string, moves []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fen == "invalid" {
		return ErrInvalidBoard
	}
	return nil
}

func TestService_AcceptsOnlyAuthorizedCurrentRevisionMoves(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository, err := NewMemoryRepository(DefaultSnapshotInterval)
	if err != nil {
		t.Fatalf("NewMemoryRepository() error = %v", err)
	}
	rules := &fakeRules{}
	service, err := NewService(repository, rules)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	state, err := service.Create(ctx, NewStateParams{
		SessionID: "session-1",
		FEN:       initialFEN,
		Mode:      ModeDemonstration,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)

	_, err = service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-student-denied",
		SessionID:        state.SessionID,
		Actor:            Actor{ID: "student-1", Role: ActorRoleStudent},
		Move:             "e2e4",
		ExpectedRevision: state.Revision,
		OccurredAt:       now,
	})
	if !errors.Is(err, ErrMoveNotAuthorized) {
		t.Fatalf("SubmitMove(student demonstration) error = %v, want %v", err, ErrMoveNotAuthorized)
	}
	if rules.calls != 0 {
		t.Fatalf("rules calls after unauthorized move = %d, want 0", rules.calls)
	}

	accepted, err := service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-1",
		SessionID:        state.SessionID,
		Actor:            Actor{ID: "coach-1", Role: ActorRoleCoach},
		Move:             "e2e4",
		ExpectedRevision: state.Revision,
		OccurredAt:       now,
	})
	if err != nil {
		t.Fatalf("SubmitMove(coach) error = %v", err)
	}
	if accepted.State.Revision != 1 || accepted.Event.Revision != 1 {
		t.Fatalf("accepted revisions = state %d, event %d; want 1", accepted.State.Revision, accepted.Event.Revision)
	}
	if !slices.Equal(accepted.State.Moves, []string{"e2e4"}) {
		t.Fatalf("accepted moves = %v, want [e2e4]", accepted.State.Moves)
	}

	_, err = service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-stale",
		SessionID:        state.SessionID,
		Actor:            Actor{ID: "coach-1", Role: ActorRoleCoach},
		Move:             "d7d5",
		ExpectedRevision: 0,
		OccurredAt:       now.Add(time.Second),
	})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("SubmitMove(stale) error = %v, want %v", err, ErrRevisionConflict)
	}
	if rules.calls != 1 {
		t.Fatalf("rules calls after stale move = %d, want 1", rules.calls)
	}
}

func TestService_StudentTurnAndIllegalMove(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository, err := NewMemoryRepository(DefaultSnapshotInterval)
	if err != nil {
		t.Fatalf("NewMemoryRepository() error = %v", err)
	}
	rules := &fakeRules{}
	service, err := NewService(repository, rules)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	state, err := service.Create(ctx, NewStateParams{
		SessionID: "session-1",
		FEN:       initialFEN,
		Mode:      ModeDemonstration,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	mode, err := service.SetMode(ctx, ModeCommand{
		EventID:          "event-mode",
		SessionID:        state.SessionID,
		Actor:            Actor{ID: "coach-1", Role: ActorRoleCoach},
		Mode:             ModeStudentTurn,
		ActiveActorID:    "student-1",
		ExpectedRevision: 0,
		OccurredAt:       now,
	})
	if err != nil {
		t.Fatalf("SetMode() error = %v", err)
	}

	_, err = service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-wrong-student",
		SessionID:        state.SessionID,
		Actor:            Actor{ID: "student-2", Role: ActorRoleStudent},
		Move:             "e2e4",
		ExpectedRevision: mode.State.Revision,
		OccurredAt:       now.Add(time.Second),
	})
	if !errors.Is(err, ErrMoveNotAuthorized) {
		t.Fatalf("SubmitMove(wrong student) error = %v, want %v", err, ErrMoveNotAuthorized)
	}

	_, err = service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-illegal",
		SessionID:        state.SessionID,
		Actor:            Actor{ID: "student-1", Role: ActorRoleStudent},
		Move:             "illegal",
		ExpectedRevision: mode.State.Revision,
		OccurredAt:       now.Add(2 * time.Second),
	})
	if !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("SubmitMove(illegal) error = %v, want %v", err, ErrIllegalMove)
	}
	current, err := repository.State(ctx, state.SessionID)
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if current.Revision != mode.State.Revision {
		t.Fatalf("revision after illegal move = %d, want %d", current.Revision, mode.State.Revision)
	}
}

func TestService_ReconnectReturnsSnapshotAndSubsequentEvents(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository, err := NewMemoryRepository(2)
	if err != nil {
		t.Fatalf("NewMemoryRepository() error = %v", err)
	}
	service, err := NewService(repository, &fakeRules{})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	state, err := service.Create(ctx, NewStateParams{
		SessionID: "session-1",
		FEN:       initialFEN,
		Mode:      ModeDemonstration,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	actor := Actor{ID: "coach-1", Role: ActorRoleCoach}
	first, err := service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-1",
		SessionID:        state.SessionID,
		Actor:            actor,
		Move:             "e2e4",
		ExpectedRevision: 0,
		OccurredAt:       now,
	})
	if err != nil {
		t.Fatalf("SubmitMove(first) error = %v", err)
	}
	second, err := service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-2",
		SessionID:        state.SessionID,
		Actor:            actor,
		Move:             "d7d5",
		ExpectedRevision: first.State.Revision,
		OccurredAt:       now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("SubmitMove(second) error = %v", err)
	}
	_, err = service.SubmitMove(ctx, MoveCommand{
		EventID:          "event-3",
		SessionID:        state.SessionID,
		Actor:            actor,
		Move:             "e4d5",
		ExpectedRevision: second.State.Revision,
		OccurredAt:       now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("SubmitMove(third) error = %v", err)
	}

	recovery, err := service.Reconnect(ctx, state.SessionID, 1)
	if err != nil {
		t.Fatalf("Reconnect() error = %v", err)
	}
	if recovery.Snapshot.Revision != 2 {
		t.Fatalf("snapshot revision = %d, want 2", recovery.Snapshot.Revision)
	}
	if len(recovery.Events) != 1 || recovery.Events[0].Revision != 3 {
		t.Fatalf("recovery events = %+v, want revision 3", recovery.Events)
	}

	recovery.Snapshot.Moves[0] = "mutated"
	current, err := repository.State(ctx, state.SessionID)
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if current.Moves[0] == "mutated" {
		t.Fatal("recovery snapshot exposed repository state for mutation")
	}
}

func TestMemoryRepository_CommitUsesCompareAndSwap(t *testing.T) {
	t.Parallel()

	repository, err := NewMemoryRepository(DefaultSnapshotInterval)
	if err != nil {
		t.Fatalf("NewMemoryRepository() error = %v", err)
	}
	state, err := NewState(NewStateParams{
		SessionID: "session-1",
		FEN:       initialFEN,
		Mode:      ModeDemonstration,
	})
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	ctx := context.Background()
	if err := repository.Create(ctx, state); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	next := cloneState(state)
	next.Revision = 1
	next.FEN = initialFEN + " changed"
	event := Event{
		ID:         "event-1",
		SessionID:  state.SessionID,
		Revision:   1,
		Kind:       EventKindPositionLoaded,
		ActorID:    "coach-1",
		OccurredAt: time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC),
		FEN:        next.FEN,
	}
	if _, err := repository.Commit(ctx, Commit{ExpectedRevision: 0, State: next, Event: event}); err != nil {
		t.Fatalf("Commit(first) error = %v", err)
	}
	event.ID = "event-2"
	_, err = repository.Commit(ctx, Commit{ExpectedRevision: 0, State: next, Event: event})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("Commit(stale) error = %v, want %v", err, ErrRevisionConflict)
	}
}

func TestMemoryRepository_RejectsInconsistentEventTransition(t *testing.T) {
	t.Parallel()

	repository, err := NewMemoryRepository(DefaultSnapshotInterval)
	if err != nil {
		t.Fatalf("NewMemoryRepository() error = %v", err)
	}
	state, err := NewState(NewStateParams{
		SessionID: "session-1",
		FEN:       initialFEN,
		Mode:      ModeDemonstration,
	})
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	ctx := context.Background()
	if err := repository.Create(ctx, state); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	next := cloneState(state)
	next.Revision = 1
	next.FEN = initialFEN + " changed without matching event"
	event := Event{
		ID:         "event-1",
		SessionID:  state.SessionID,
		Revision:   1,
		Kind:       EventKindModeChanged,
		ActorID:    "coach-1",
		OccurredAt: time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC),
		Mode:       ModeDemonstration,
	}
	_, err = repository.Commit(ctx, Commit{ExpectedRevision: 0, State: next, Event: event})
	if !errors.Is(err, ErrInvalidBoard) {
		t.Fatalf("Commit(inconsistent) error = %v, want %v", err, ErrInvalidBoard)
	}
}

func TestMemoryRepository_ConcurrentCommitsAcceptOneRevision(t *testing.T) {
	t.Parallel()

	repository, err := NewMemoryRepository(DefaultSnapshotInterval)
	if err != nil {
		t.Fatalf("NewMemoryRepository() error = %v", err)
	}
	state, err := NewState(NewStateParams{
		SessionID: "session-1",
		FEN:       initialFEN,
		Mode:      ModeDemonstration,
	})
	if err != nil {
		t.Fatalf("NewState() error = %v", err)
	}
	ctx := context.Background()
	if err := repository.Create(ctx, state); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	commits := make([]Commit, 0, 2)
	for index, move := range []string{"e2e4", "d2d4"} {
		next := cloneState(state)
		next.Revision = 1
		next.FEN = fmt.Sprintf("%s after %s", initialFEN, move)
		next.Moves = append(next.Moves, move)
		commits = append(commits, Commit{
			ExpectedRevision: 0,
			State:            next,
			Event: Event{
				ID:         fmt.Sprintf("event-%d", index),
				SessionID:  state.SessionID,
				Revision:   1,
				Kind:       EventKindMoveApplied,
				ActorID:    "coach-1",
				OccurredAt: time.Date(2026, time.August, 27, 10, 0, index, 0, time.UTC),
				Move:       move,
				FEN:        next.FEN,
			},
		})
	}

	errorsFound := make(chan error, len(commits))
	var waitGroup sync.WaitGroup
	for _, commit := range commits {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := repository.Commit(ctx, commit)
			errorsFound <- err
		}()
	}
	waitGroup.Wait()
	close(errorsFound)

	accepted := 0
	conflicted := 0
	for err := range errorsFound {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, ErrRevisionConflict):
			conflicted++
		default:
			t.Fatalf("Commit() concurrent error = %v", err)
		}
	}
	if accepted != 1 || conflicted != 1 {
		t.Fatalf("concurrent commits accepted = %d, conflicted = %d; want 1 and 1", accepted, conflicted)
	}
	current, err := repository.State(ctx, state.SessionID)
	if err != nil {
		t.Fatalf("State() error = %v", err)
	}
	if current.Revision != 1 || len(current.Moves) != 1 {
		t.Fatalf("State() after concurrent commit = %+v, want one accepted revision", current)
	}
}
