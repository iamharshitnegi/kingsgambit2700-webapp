// Package learning owns assignments, teaching resources and guardian-visible learning records.
package learning

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrDenied   = errors.New("learning: permission denied")
	ErrNotFound = errors.New("learning: record not found")
	ErrConflict = errors.New("learning: record changed or transition unavailable")
)

type ValidationError struct{ Detail string }

func (e ValidationError) Error() string { return e.Detail }

type Puzzle struct {
	Title     string     `json:"title"`
	FEN       string     `json:"fen"`
	Solutions [][]string `json:"solutions,omitempty"`
}
type PuzzleProgress struct {
	Moves        []string  `json:"moves"`
	Incorrect    int       `json:"incorrect"`
	Hints        int       `json:"hints"`
	Seconds      int       `json:"seconds"`
	Solved       bool      `json:"solved"`
	Revealed     bool      `json:"revealed"`
	Hint         string    `json:"hint,omitempty"`
	LastActionAt time.Time `json:"lastActionAt"`
}
type Metrics struct {
	InProgress     int      `json:"inProgress"`
	Completed      int      `json:"completed"`
	NotSolved      int      `json:"notSolved"`
	NotAttempted   int      `json:"notAttempted"`
	FirstTry       int      `json:"firstTry"`
	Incorrect      int      `json:"incorrect"`
	Hints          int      `json:"hints"`
	Reveals        int      `json:"reveals"`
	Seconds        int      `json:"seconds"`
	Accuracy       *float64 `json:"accuracy"`
	AverageSeconds *float64 `json:"averageSeconds"`
}
type Attempt struct {
	Files    []File           `json:"files"`
	ID       string           `json:"id"`
	Number   int              `json:"number"`
	Status   string           `json:"status"`
	Answer   string           `json:"answer"`
	Feedback string           `json:"feedback"`
	Puzzles  []PuzzleProgress `json:"puzzles"`
	Metrics  Metrics          `json:"metrics"`
}
type Assignment struct {
	Resources   []AssignedResource `json:"resources"`
	ID          string             `json:"id"`
	StudentID   string             `json:"studentId"`
	ClassID     string             `json:"classId,omitempty"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Kind        string             `json:"kind"`
	Status      string             `json:"status"`
	Version     int                `json:"version"`
	DueAt       *time.Time         `json:"dueAt"`
	Puzzles     []Puzzle           `json:"puzzles"`
	Attempts    []Attempt          `json:"attempts"`
}
type AssignedResource struct {
	ID       string `json:"id"`
	Version  int    `json:"version"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Content  string `json:"content"`
	FileID   string `json:"fileId,omitempty"`
	Filename string `json:"filename,omitempty"`
}
type Student struct {
	LevelSlug      string `json:"levelSlug"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	CanManage      bool   `json:"canManage"`
	CanSubmit      bool   `json:"canSubmit"`
	CanCommunicate bool   `json:"canCommunicate"`
}
type Placement struct {
	ID                  string    `json:"id"`
	StudentID           string    `json:"studentId"`
	CurriculumVersionID string    `json:"curriculumVersionId"`
	CurriculumVersion   string    `json:"curriculumVersion"`
	LevelSlug           string    `json:"levelSlug"`
	PreviousLevelSlug   string    `json:"previousLevelSlug"`
	AssessmentID        string    `json:"assessmentId"`
	Reason              string    `json:"reason"`
	PlacedBy            string    `json:"placedBy"`
	CreatedAt           time.Time `json:"createdAt"`
}
type Resource struct {
	FileID     string   `json:"fileId,omitempty"`
	Filename   string   `json:"filename,omitempty"`
	MediaType  string   `json:"mediaType,omitempty"`
	ID         string   `json:"id"`
	OwnerID    string   `json:"ownerId"`
	Title      string   `json:"title"`
	Kind       string   `json:"kind"`
	Visibility string   `json:"visibility"`
	Status     string   `json:"status"`
	LevelSlug  string   `json:"levelSlug"`
	Tags       []string `json:"tags"`
	Content    string   `json:"content"`
	Version    int      `json:"version"`
}
type Game struct {
	ID        string    `json:"id"`
	StudentID string    `json:"studentId"`
	Title     string    `json:"title"`
	Source    string    `json:"source"`
	PGN       string    `json:"pgn"`
	Review    string    `json:"review"`
	Shared    bool      `json:"shared"`
	PlayedAt  time.Time `json:"playedAt"`
	Version   int       `json:"version"`
}
type Message struct {
	ID        string    `json:"id"`
	ActorID   string    `json:"actorId"`
	ActorName string    `json:"actorName"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
}
type Thread struct {
	ID        string    `json:"id"`
	StudentID string    `json:"studentId,omitempty"`
	BatchID   string    `json:"batchId,omitempty"`
	Title     string    `json:"title"`
	Unread    int       `json:"unread"`
	Messages  []Message `json:"messages"`
	CanSend   bool      `json:"canSend"`
}
type Notification struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Href      string     `json:"href"`
	CreatedAt time.Time  `json:"createdAt"`
	ReadAt    *time.Time `json:"readAt"`
}
type Rubric struct {
	ID         string   `json:"id"`
	LevelSlug  string   `json:"levelSlug"`
	Title      string   `json:"title"`
	Version    int      `json:"version"`
	Dimensions []string `json:"dimensions"`
}
type Rating struct {
	Dimension string `json:"dimension"`
	Rating    string `json:"rating"`
	Evidence  string `json:"evidence"`
}
type AssessmentContent struct {
	Learned      string   `json:"learned"`
	Strengths    string   `json:"strengths"`
	FocusNext    string   `json:"focusNext"`
	PracticePlan string   `json:"practicePlan"`
	CoachComment string   `json:"coachComment"`
	Ratings      []Rating `json:"ratings"`
	Eligible     bool     `json:"eligible"`
}
type Assessment struct {
	ReportID        string            `json:"reportId"`
	SourceVersionID string            `json:"sourceVersionId,omitempty"`
	History         []AssessmentEvent `json:"history"`
	ID              string            `json:"id"`
	StudentID       string            `json:"studentId"`
	RubricID        string            `json:"rubricId"`
	Title           string            `json:"title"`
	Period          string            `json:"period"`
	Content         AssessmentContent `json:"content"`
	Status          string            `json:"status"`
	Version         int               `json:"version"`
	PublishedAt     *time.Time        `json:"publishedAt"`
	AcknowledgedAt  *time.Time        `json:"acknowledgedAt"`
}
type AssessmentEvent struct {
	Action  string          `json:"action"`
	ActorID string          `json:"actorId"`
	At      time.Time       `json:"at"`
	Content json.RawMessage `json:"content"`
}
type Certificate struct {
	FileID       string    `json:"fileId,omitempty"`
	ID           string    `json:"id"`
	StudentID    string    `json:"studentId"`
	AssessmentID string    `json:"assessmentId"`
	LevelSlug    string    `json:"levelSlug"`
	Status       string    `json:"status"`
	IssuedAt     time.Time `json:"issuedAt"`
	Reason       string    `json:"reason"`
}
type Workspace struct {
	Pagination         Pagination     `json:"pagination"`
	Placements         []Placement    `json:"placements"`
	CanPlace           bool           `json:"canPlace"`
	Students           []Student      `json:"students"`
	Assignments        []Assignment   `json:"assignments"`
	Resources          []Resource     `json:"resources"`
	Games              []Game         `json:"games"`
	Threads            []Thread       `json:"threads"`
	Notifications      []Notification `json:"notifications"`
	Rubrics            []Rubric       `json:"rubrics"`
	Assessments        []Assessment   `json:"assessments"`
	Certificates       []Certificate  `json:"certificates"`
	CanPublish         bool           `json:"canPublish"`
	CanCreateResources bool           `json:"canCreateResources"`
}

// Command is the typed input for POST /learning/actions. Each action validates its required fields.
type Command struct {
	Action            string            `json:"action"`
	ID                string            `json:"id"`
	Version           int               `json:"version"`
	StudentID         string            `json:"studentId"`
	StudentIDs        []string          `json:"studentIds"`
	ResourceIDs       []string          `json:"resourceIds"`
	AssessmentID      string            `json:"assessmentId"`
	BatchID           string            `json:"batchId"`
	ClassID           string            `json:"classId"`
	Title             string            `json:"title"`
	Description       string            `json:"description"`
	Kind              string            `json:"kind"`
	Status            string            `json:"status"`
	DueAt             *time.Time        `json:"dueAt"`
	Puzzles           []Puzzle          `json:"puzzles"`
	Answer            string            `json:"answer"`
	Feedback          string            `json:"feedback"`
	PuzzleIndex       int               `json:"puzzleIndex"`
	Move              string            `json:"move"`
	ElapsedSeconds    int               `json:"elapsedSeconds"`
	Visibility        string            `json:"visibility"`
	LevelSlug         string            `json:"levelSlug"`
	Tags              []string          `json:"tags"`
	Content           string            `json:"content"`
	PGN               string            `json:"pgn"`
	Source            string            `json:"source"`
	Shared            bool              `json:"shared"`
	PlayedAt          *time.Time        `json:"playedAt"`
	Body              string            `json:"body"`
	RubricID          string            `json:"rubricId"`
	Period            string            `json:"period"`
	AssessmentContent AssessmentContent `json:"assessmentContent"`
}
