package classroom

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

const standardFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func TestLessonToolsPersistAndRecover(t *testing.T) {
	repository, _ := NewMemoryRepository(20)
	service, _ := NewService(repository, ChessRules{})
	state, err := service.Create(t.Context(), NewStateParams{SessionID: "lesson", FEN: standardFEN, Mode: ModeDemonstration})
	if err != nil {
		t.Fatal(err)
	}
	coach := Actor{ID: "coach", Role: ActorRoleCoach}
	now := time.Now().UTC()
	index := 0
	tool := func(command LessonCommand) Accepted {
		t.Helper()
		index++
		command.EventID = fmt.Sprintf("event-%d", index)
		command.SessionID = state.SessionID
		command.Actor = coach
		command.ExpectedRevision = state.Revision
		command.OccurredAt = now
		result, err := service.UpdateLesson(t.Context(), command)
		if err != nil {
			t.Fatal(err)
		}
		state = result.State
		return result
	}
	tool(LessonCommand{Operation: "annotation", Annotations: []Annotation{{StartSquare: "e2", EndSquare: "e4", Color: "#b87628"}, {StartSquare: "d4", EndSquare: "d4", Color: "#334455"}}})
	checkpoint := tool(LessonCommand{Operation: "checkpoint", Title: "Before the opening"})
	result, err := service.SubmitMove(t.Context(), MoveCommand{EventID: "move", SessionID: state.SessionID, Actor: coach, Move: "e2e4", ExpectedRevision: state.Revision, OccurredAt: now})
	if err != nil {
		t.Fatal(err)
	}
	state = result.State
	if len(state.Lesson.Annotations) != 0 || len(state.Lesson.Positions) != 2 {
		t.Fatalf("move did not advance history and clear markings: %+v", state.Lesson)
	}
	tool(LessonCommand{Operation: "undo"})
	if state.FEN != standardFEN || len(state.Moves) != 0 {
		t.Fatalf("undo failed: %+v", state)
	}
	tool(LessonCommand{Operation: "countdown", Seconds: 180, Label: "Find a plan"})
	deadline := *state.Lesson.CountdownEndsAt
	tool(LessonCommand{Operation: "restoreCheckpoint", CheckpointID: checkpoint.Event.ID})
	if len(state.Lesson.Annotations) != 2 || !state.Lesson.CountdownEndsAt.Equal(deadline) {
		t.Fatal("checkpoint did not restore markings or changed running timer")
	}
	recovery, err := service.Reconnect(t.Context(), state.SessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(recovery.Events) != int(state.Revision) {
		t.Fatalf("missing recovery events: %+v", recovery)
	}
	for _, event := range recovery.Events {
		if event.State == nil || event.State.Revision != event.Revision {
			t.Fatal("event missing canonical recovery state")
		}
	}
	last := recovery.Events[len(recovery.Events)-1].State
	if !reflect.DeepEqual(*last, state) {
		t.Fatal("recovery state differs from committed lesson")
	}
	last.Lesson.Checkpoints[0].Annotations[0].Color = "#ffffff"
	actual, _ := repository.State(t.Context(), state.SessionID)
	if actual.Lesson.Checkpoints[0].Annotations[0].Color == "#ffffff" {
		t.Fatal("recovery mutation escaped repository")
	}
	tool(LessonCommand{Operation: "deleteCheckpoint", CheckpointID: checkpoint.Event.ID})
	tool(LessonCommand{Operation: "countdown", Seconds: 0})
	if len(state.Lesson.Checkpoints) != 0 || state.Lesson.CountdownEndsAt != nil {
		t.Fatal("lesson cleanup failed")
	}
}

func TestLessonToolsRejectStudentAndStaleOrInvalidCommands(t *testing.T) {
	repository, _ := NewMemoryRepository(20)
	service, _ := NewService(repository, ChessRules{})
	state, _ := service.Create(t.Context(), NewStateParams{SessionID: "lesson", FEN: standardFEN, Mode: ModeFreeAnalysis})
	for _, operation := range []string{"annotation", "undo", "checkpoint", "restoreCheckpoint", "deleteCheckpoint", "countdown"} {
		_, err := service.UpdateLesson(t.Context(), LessonCommand{EventID: operation, SessionID: state.SessionID, Actor: Actor{ID: "student", Role: ActorRoleStudent}, Operation: operation, OccurredAt: time.Now()})
		if !errors.Is(err, ErrControlNotAuthorized) {
			t.Fatalf("%s student error: %v", operation, err)
		}
	}
	for _, command := range []LessonCommand{
		{Operation: "annotation", Annotations: []Annotation{{StartSquare: "z9", EndSquare: "e4", Color: "#ffffff"}}},
		{Operation: "annotation", Annotations: []Annotation{{StartSquare: "e2", EndSquare: "e4", Color: "url(javascript:bad)"}}},
		{Operation: "countdown", Seconds: -1}, {Operation: "countdown", Seconds: 10801}, {Operation: "checkpoint", Title: " "}, {Operation: "undo"}, {Operation: "restoreCheckpoint", CheckpointID: "missing"},
	} {
		command.EventID = "invalid"
		command.SessionID = state.SessionID
		command.Actor = Actor{ID: "coach", Role: ActorRoleCoach}
		command.OccurredAt = time.Now()
		if _, err := service.UpdateLesson(t.Context(), command); !errors.Is(err, ErrInvalidBoard) {
			t.Fatalf("%+v = %v", command, err)
		}
	}
	_, err := service.UpdateLesson(t.Context(), LessonCommand{EventID: "stale", SessionID: state.SessionID, Actor: Actor{ID: "coach", Role: ActorRoleCoach}, Operation: "countdown", Seconds: 10, ExpectedRevision: 1, OccurredAt: time.Now()})
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale command = %v", err)
	}
	current, _ := repository.State(t.Context(), state.SessionID)
	if current.Revision != 0 {
		t.Fatal("rejected command mutated board")
	}
}

func TestImportedHistoryMustReplayAndCanBeUndone(t *testing.T) {
	repository, _ := NewMemoryRepository(20)
	service, _ := NewService(repository, ChessRules{})
	state, _ := service.Create(t.Context(), NewStateParams{SessionID: "lesson", FEN: standardFEN, Mode: ModeDemonstration})
	coach := Actor{ID: "coach", Role: ActorRoleCoach}
	now := time.Now()
	command := PositionCommand{EventID: "import", SessionID: state.SessionID, Actor: coach, InitialFEN: standardFEN, FEN: standardFEN, Moves: []string{"e2e4"}, OccurredAt: now}
	if _, err := service.LoadPosition(t.Context(), command); !errors.Is(err, ErrInvalidBoard) {
		t.Fatalf("mismatching final fen = %v", err)
	}
	command.Moves = []string{"e2e5"}
	if _, err := service.LoadPosition(t.Context(), command); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("illegal sequence = %v", err)
	}
	command.Moves = []string{"e2e4"}
	command.FEN = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1"
	loaded, err := service.LoadPosition(t.Context(), command)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.State.Lesson.Positions) != 2 {
		t.Fatal("imported history missing")
	}
	undone, err := service.UpdateLesson(t.Context(), LessonCommand{EventID: "undo", SessionID: state.SessionID, Actor: coach, ExpectedRevision: loaded.State.Revision, Operation: "undo", OccurredAt: now})
	if err != nil || undone.State.FEN != standardFEN {
		t.Fatalf("import undo = %+v %v", undone, err)
	}
}
