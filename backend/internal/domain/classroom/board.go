package classroom

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"time"
)

var (
	ErrInvalidBoard         = errors.New("classroom: invalid board")
	ErrBoardNotFound        = errors.New("classroom: board not found")
	ErrBoardExists          = errors.New("classroom: board already exists")
	ErrBoardClosed          = errors.New("classroom: board closed")
	ErrMoveNotAuthorized    = errors.New("classroom: move not authorized")
	ErrControlNotAuthorized = errors.New("classroom: control not authorized")
	ErrIllegalMove          = errors.New("classroom: illegal move")
	ErrRevisionConflict     = errors.New("classroom: revision conflict")
	ErrEventConflict        = errors.New("classroom: event conflict")
)

type RevisionConflictError struct {
	Expected uint64
	Current  uint64
}

func (e *RevisionConflictError) Error() string {
	return fmt.Sprintf(
		"classroom: expected revision %d, current revision %d",
		e.Expected,
		e.Current,
	)
}

func (e *RevisionConflictError) Unwrap() error {
	return ErrRevisionConflict
}

type Mode string

const (
	ModeUnknown         Mode = ""
	ModeDemonstration   Mode = "demonstration"
	ModeFreeAnalysis    Mode = "free_analysis"
	ModeStudentTurn     Mode = "student_turn"
	ModePrivateResponse Mode = "private_response"
	ModeGameReview      Mode = "game_review"
	ModeEngineReview    Mode = "engine_review"
)

func (m Mode) valid() bool {
	switch m {
	case ModeDemonstration,
		ModeFreeAnalysis,
		ModeStudentTurn,
		ModePrivateResponse,
		ModeGameReview,
		ModeEngineReview:
		return true
	default:
		return false
	}
}

type BoardStatus string

const (
	BoardStatusUnknown BoardStatus = ""
	BoardStatusOpen    BoardStatus = "open"
	BoardStatusClosed  BoardStatus = "closed"
)

type ActorRole string

const (
	ActorRoleUnknown         ActorRole = ""
	ActorRoleStudent         ActorRole = "student"
	ActorRoleCoach           ActorRole = "coach"
	ActorRoleAcademicManager ActorRole = "academic_manager"
	ActorRoleOperationsAdmin ActorRole = "operations_admin"
	ActorRoleSuperAdmin      ActorRole = "super_admin"
)

type Actor struct {
	ID   string
	Role ActorRole
}

func (a Actor) canControl() bool {
	switch a.Role {
	case ActorRoleCoach,
		ActorRoleAcademicManager,
		ActorRoleOperationsAdmin,
		ActorRoleSuperAdmin:
		return a.ID != ""
	default:
		return false
	}
}

// State is the server-authoritative board state. Revision increases exactly
// once for every accepted event.
type State struct {
	SessionID     string      `json:"sessionId"`
	FEN           string      `json:"fen"`
	Moves         []string    `json:"moves"`
	Mode          Mode        `json:"mode"`
	ActiveActorID string      `json:"activeActorId,omitempty"`
	Revision      uint64      `json:"revision"`
	Status        BoardStatus `json:"status"`
	Lesson        LessonState `json:"lesson"`
}

type NewStateParams struct {
	SessionID     string
	FEN           string
	Mode          Mode
	ActiveActorID string
}

func NewState(params NewStateParams) (State, error) {
	state := State{
		SessionID:     params.SessionID,
		FEN:           params.FEN,
		Moves:         []string{},
		Mode:          params.Mode,
		ActiveActorID: params.ActiveActorID,
		Status:        BoardStatusOpen,
		Lesson:        newLesson(params.FEN),
	}
	if err := state.Validate(); err != nil {
		return State{}, err
	}
	return state, nil
}

func (s State) Validate() error {
	if s.SessionID == "" || s.FEN == "" || !s.Mode.valid() {
		return fmt.Errorf("%w: session, fen and mode are required", ErrInvalidBoard)
	}
	if s.Moves == nil {
		return fmt.Errorf("%w: moves must be initialized", ErrInvalidBoard)
	}
	if s.Status != BoardStatusOpen && s.Status != BoardStatusClosed {
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidBoard, s.Status)
	}
	requiresActiveActor := s.Mode == ModeStudentTurn || s.Mode == ModePrivateResponse
	if requiresActiveActor && s.ActiveActorID == "" {
		return fmt.Errorf("%w: mode %q requires an active actor", ErrInvalidBoard, s.Mode)
	}
	return nil
}

func (s State) CanMove(actor Actor) bool {
	if s.Status != BoardStatusOpen || actor.ID == "" {
		return false
	}
	if actor.canControl() {
		return true
	}
	if actor.Role != ActorRoleStudent {
		return false
	}

	switch s.Mode {
	case ModeFreeAnalysis:
		return true
	case ModeStudentTurn:
		return s.ActiveActorID == actor.ID
	case ModePrivateResponse:
		// Shared-board events are broadcast to the whole room. Private answers
		// require their own recipient-scoped workflow before students may submit.
		return false
	default:
		return false
	}
}

func cloneState(state State) State {
	cloned := state
	cloned.Moves = slices.Clone(state.Moves)
	cloned.Lesson = cloneLesson(state.Lesson)
	cloned.normalizeLesson()
	if cloned.Moves == nil {
		cloned.Moves = []string{}
	}
	return cloned
}

type EventKind string

const (
	EventKindUnknown        EventKind = ""
	EventKindMoveApplied    EventKind = "move_applied"
	EventKindModeChanged    EventKind = "mode_changed"
	EventKindPositionLoaded EventKind = "position_loaded"
	EventKindBoardClosed    EventKind = "board_closed"
	EventKindLessonUpdated  EventKind = "lesson_updated"
)

type Event struct {
	ID            string    `json:"id"`
	SessionID     string    `json:"sessionId"`
	Revision      uint64    `json:"revision"`
	Kind          EventKind `json:"kind"`
	ActorID       string    `json:"actorId"`
	OccurredAt    time.Time `json:"occurredAt"`
	Move          string    `json:"move,omitempty"`
	FEN           string    `json:"fen,omitempty"`
	Mode          Mode      `json:"mode,omitempty"`
	ActiveActorID string    `json:"activeActorId,omitempty"`
	State         *State    `json:"state,omitempty"`
}

func (e Event) validate(state State) error {
	if e.ID == "" || e.SessionID == "" || e.ActorID == "" || e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: event metadata is required", ErrInvalidBoard)
	}
	if e.SessionID != state.SessionID || e.Revision != state.Revision {
		return fmt.Errorf("%w: event does not match state revision", ErrInvalidBoard)
	}
	if e.State != nil && !reflect.DeepEqual(cloneState(*e.State), cloneState(state)) {
		return fmt.Errorf("%w: event recovery state differs from committed state", ErrInvalidBoard)
	}
	switch e.Kind {
	case EventKindMoveApplied:
		if e.Move == "" || e.FEN == "" {
			return fmt.Errorf("%w: move event is incomplete", ErrInvalidBoard)
		}
	case EventKindModeChanged:
		if !e.Mode.valid() {
			return fmt.Errorf("%w: mode event is incomplete", ErrInvalidBoard)
		}
	case EventKindPositionLoaded:
		if e.FEN == "" {
			return fmt.Errorf("%w: position event is incomplete", ErrInvalidBoard)
		}
	case EventKindBoardClosed:
	case EventKindLessonUpdated:
		if e.State == nil {
			return fmt.Errorf("%w: lesson event needs resulting state", ErrInvalidBoard)
		}
	default:
		return fmt.Errorf("%w: unsupported event kind %q", ErrInvalidBoard, e.Kind)
	}
	return nil
}

func validateTransition(previous, next State, event Event) error {
	if previous.SessionID != next.SessionID || previous.SessionID != event.SessionID {
		return fmt.Errorf("%w: session changed during transition", ErrInvalidBoard)
	}
	if previous.Status != BoardStatusOpen {
		return ErrBoardClosed
	}
	if next.Revision != previous.Revision+1 || event.Revision != next.Revision {
		return fmt.Errorf("%w: revision must advance by one", ErrRevisionConflict)
	}

	switch event.Kind {
	case EventKindMoveApplied:
		isAppend := len(next.Moves) == len(previous.Moves)+1 &&
			slices.Equal(previous.Moves, next.Moves[:len(previous.Moves)])
		if !isAppend ||
			next.Moves[len(next.Moves)-1] != event.Move ||
			next.FEN != event.FEN ||
			next.Mode != previous.Mode ||
			next.ActiveActorID != previous.ActiveActorID ||
			next.Status != previous.Status {
			return fmt.Errorf("%w: invalid move transition", ErrInvalidBoard)
		}

	case EventKindModeChanged:
		if next.FEN != previous.FEN ||
			!slices.Equal(next.Moves, previous.Moves) ||
			next.Status != previous.Status ||
			next.Mode != event.Mode ||
			next.ActiveActorID != event.ActiveActorID {
			return fmt.Errorf("%w: invalid mode transition", ErrInvalidBoard)
		}

	case EventKindPositionLoaded:
		if next.FEN != event.FEN ||
			next.Mode != previous.Mode ||
			next.ActiveActorID != previous.ActiveActorID ||
			next.Status != previous.Status {
			return fmt.Errorf("%w: invalid position transition", ErrInvalidBoard)
		}

	case EventKindLessonUpdated:
		if next.FEN != previous.FEN || !slices.Equal(next.Moves, previous.Moves) || next.Mode != previous.Mode || next.ActiveActorID != previous.ActiveActorID || next.Status != previous.Status {
			return fmt.Errorf("%w: invalid lesson transition", ErrInvalidBoard)
		}
	case EventKindBoardClosed:
		if next.FEN != previous.FEN ||
			!slices.Equal(next.Moves, previous.Moves) ||
			next.Mode != previous.Mode ||
			next.ActiveActorID != previous.ActiveActorID ||
			next.Status != BoardStatusClosed {
			return fmt.Errorf("%w: invalid close transition", ErrInvalidBoard)
		}

	default:
		return fmt.Errorf("%w: unsupported event kind %q", ErrInvalidBoard, event.Kind)
	}

	return nil
}

type Commit struct {
	ExpectedRevision uint64
	State            State
	Event            Event
}

type Recovery struct {
	Snapshot State   `json:"snapshot"`
	Events   []Event `json:"events"`
}
