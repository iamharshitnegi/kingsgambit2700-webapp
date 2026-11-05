package classroom

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

const maxLessonMoves = 512
const maxCheckpoints = 8

// Annotation uses equal squares for a highlighted square and different squares
// for an arrow. Only the coach can replace the shared annotation collection.
type Annotation struct {
	StartSquare string `json:"startSquare"`
	EndSquare   string `json:"endSquare"`
	Color       string `json:"color"`
}

type Checkpoint struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	FEN         string       `json:"fen"`
	InitialFEN  string       `json:"initialFen"`
	Moves       []string     `json:"moves"`
	Positions   []string     `json:"positions"`
	Annotations []Annotation `json:"annotations"`
}

type LessonState struct {
	InitialFEN      string       `json:"initialFen"`
	Positions       []string     `json:"positions"`
	Annotations     []Annotation `json:"annotations"`
	Checkpoints     []Checkpoint `json:"checkpoints"`
	CountdownEndsAt *time.Time   `json:"countdownEndsAt,omitempty"`
	CountdownLabel  string       `json:"countdownLabel,omitempty"`
}

func newLesson(fen string) LessonState {
	return LessonState{InitialFEN: fen, Positions: []string{fen}, Annotations: []Annotation{}, Checkpoints: []Checkpoint{}}
}

func (s *State) normalizeLesson() {
	if len(s.Lesson.Positions) == 0 {
		s.Lesson.Positions = []string{s.FEN}
	}
	if s.Lesson.InitialFEN == "" && len(s.Moves) == 0 {
		s.Lesson.InitialFEN = s.FEN
	}
	if s.Lesson.Annotations == nil {
		s.Lesson.Annotations = []Annotation{}
	}
	if s.Lesson.Checkpoints == nil {
		s.Lesson.Checkpoints = []Checkpoint{}
	}
}

func cloneLesson(lesson LessonState) LessonState {
	lesson.Positions = slices.Clone(lesson.Positions)
	lesson.Annotations = slices.Clone(lesson.Annotations)
	lesson.Checkpoints = slices.Clone(lesson.Checkpoints)
	for i := range lesson.Checkpoints {
		checkpoint := &lesson.Checkpoints[i]
		checkpoint.Moves = slices.Clone(checkpoint.Moves)
		checkpoint.Positions = slices.Clone(checkpoint.Positions)
		checkpoint.Annotations = slices.Clone(checkpoint.Annotations)
	}
	if lesson.CountdownEndsAt != nil {
		deadline := *lesson.CountdownEndsAt
		lesson.CountdownEndsAt = &deadline
	}
	return lesson
}

func statePointer(state State) *State { cloned := cloneState(state); return &cloned }
func cloneEvent(event Event) Event {
	if event.State != nil {
		event.State = statePointer(*event.State)
	}
	return event
}

// InitialFEN makes imported history verifiable. Legacy v1 position loads did
// not provide a root, so only subsequently played moves can be undone/exported.
func (s *Service) positionLesson(ctx context.Context, initial, fen string, moves []string) (LessonState, error) {
	if len(moves) > maxLessonMoves {
		return LessonState{}, fmt.Errorf("%w: too many moves", ErrInvalidBoard)
	}
	lesson := newLesson(fen)
	if initial == "" {
		if len(moves) != 0 {
			lesson.InitialFEN = ""
		}
		return lesson, nil
	}
	if err := s.rules.ValidatePosition(ctx, initial, nil); err != nil {
		return LessonState{}, err
	}
	lesson.InitialFEN = initial
	lesson.Positions = []string{initial}
	current := initial
	for _, move := range moves {
		result, err := s.rules.ApplyMove(ctx, current, move)
		if err != nil {
			return LessonState{}, fmt.Errorf("replaying imported game: %w", err)
		}
		current = result.FEN
		lesson.Positions = append(lesson.Positions, current)
	}
	// Some chess libraries omit an uncapturable en-passant target. Compare the
	// meaningful board fields and clocks, allowing this harmless FEN difference.
	if !equivalentPosition(current, fen) {
		return LessonState{}, fmt.Errorf("%w: moves do not reach the supplied position", ErrInvalidBoard)
	}
	return lesson, nil
}

func equivalentPosition(a, b string) bool {
	if a == b {
		return true
	}
	af, bf := strings.Fields(a), strings.Fields(b)
	if len(af) != 6 || len(bf) != 6 {
		return false
	}
	for _, i := range []int{0, 1, 2, 4, 5} {
		if af[i] != bf[i] {
			return false
		}
	}
	// Only normalize an absent target against the other representation. The
	// sequence itself was replayed with the authoritative rules above.
	return af[3] == bf[3] || af[3] == "-" || bf[3] == "-"
}

type LessonCommand struct {
	EventID          string
	SessionID        string
	Actor            Actor
	ExpectedRevision uint64
	OccurredAt       time.Time
	Operation        string
	Annotations      []Annotation
	Title            string
	CheckpointID     string
	Seconds          int
	Label            string
}

var squarePattern = regexp.MustCompile(`^[a-h][1-8]$`)
var annotationColorPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Service) UpdateLesson(ctx context.Context, command LessonCommand) (Accepted, error) {
	if command.EventID == "" || command.OccurredAt.IsZero() {
		return Accepted{}, ErrInvalidBoard
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
	kind := EventKindLessonUpdated
	switch command.Operation {
	case "annotation":
		if command.Annotations == nil || len(command.Annotations) > 64 {
			return Accepted{}, ErrInvalidBoard
		}
		for _, annotation := range command.Annotations {
			if !squarePattern.MatchString(annotation.StartSquare) || !squarePattern.MatchString(annotation.EndSquare) || !annotationColorPattern.MatchString(annotation.Color) {
				return Accepted{}, ErrInvalidBoard
			}
		}
		next.Lesson.Annotations = slices.Clone(command.Annotations)
	case "undo":
		if len(next.Lesson.Positions) < 2 || len(next.Moves) == 0 {
			return Accepted{}, fmt.Errorf("%w: no move to undo", ErrInvalidBoard)
		}
		next.Lesson.Positions = next.Lesson.Positions[:len(next.Lesson.Positions)-1]
		next.FEN = next.Lesson.Positions[len(next.Lesson.Positions)-1]
		next.Moves = next.Moves[:len(next.Moves)-1]
		next.Lesson.Annotations = []Annotation{}
		kind = EventKindPositionLoaded
	case "checkpoint":
		title := strings.TrimSpace(command.Title)
		if title == "" || len([]rune(title)) > 100 || len(next.Lesson.Checkpoints) >= maxCheckpoints {
			return Accepted{}, ErrInvalidBoard
		}
		next.Lesson.Checkpoints = append(next.Lesson.Checkpoints, Checkpoint{ID: command.EventID, Title: title, FEN: next.FEN, InitialFEN: next.Lesson.InitialFEN, Moves: slices.Clone(next.Moves), Positions: slices.Clone(next.Lesson.Positions), Annotations: slices.Clone(next.Lesson.Annotations)})
	case "restoreCheckpoint", "deleteCheckpoint":
		index := slices.IndexFunc(next.Lesson.Checkpoints, func(checkpoint Checkpoint) bool { return checkpoint.ID == command.CheckpointID })
		if index < 0 {
			return Accepted{}, ErrInvalidBoard
		}
		if command.Operation == "deleteCheckpoint" {
			next.Lesson.Checkpoints = slices.Delete(next.Lesson.Checkpoints, index, index+1)
			break
		}
		checkpoint := next.Lesson.Checkpoints[index]
		next.FEN, next.Moves = checkpoint.FEN, slices.Clone(checkpoint.Moves)
		next.Lesson.InitialFEN = checkpoint.InitialFEN
		next.Lesson.Positions = slices.Clone(checkpoint.Positions)
		next.Lesson.Annotations = slices.Clone(checkpoint.Annotations)
		kind = EventKindPositionLoaded
	case "countdown":
		label := strings.TrimSpace(command.Label)
		if command.Seconds < 0 || command.Seconds > 10800 || len([]rune(label)) > 100 {
			return Accepted{}, ErrInvalidBoard
		}
		next.Lesson.CountdownEndsAt = nil
		next.Lesson.CountdownLabel = ""
		if command.Seconds > 0 {
			deadline := command.OccurredAt.Add(time.Duration(command.Seconds) * time.Second)
			next.Lesson.CountdownEndsAt = &deadline
			next.Lesson.CountdownLabel = label
		}
	default:
		return Accepted{}, ErrInvalidBoard
	}
	next.Revision++
	event := Event{ID: command.EventID, SessionID: next.SessionID, Revision: next.Revision, Kind: kind, ActorID: command.Actor.ID, OccurredAt: command.OccurredAt, State: statePointer(next)}
	if kind == EventKindPositionLoaded {
		event.FEN = next.FEN
	}
	committed, err := s.repository.Commit(ctx, Commit{ExpectedRevision: current.Revision, State: next, Event: event})
	if err != nil {
		return Accepted{}, fmt.Errorf("committing lesson tool: %w", err)
	}
	return Accepted{State: committed, Event: event}, nil
}
