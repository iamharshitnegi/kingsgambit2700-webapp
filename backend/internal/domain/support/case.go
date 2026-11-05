package support

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalidCase = errors.New("support: invalid case")

type Category string

const (
	CategoryUnknown   Category = ""
	CategoryClass     Category = "class"
	CategoryLearning  Category = "learning"
	CategoryAccount   Category = "account"
	CategoryPolicy    Category = "policy"
	CategoryTechnical Category = "technical"
	CategoryOther     Category = "other"
)

func (c Category) Valid() bool {
	switch c {
	case CategoryClass, CategoryLearning, CategoryAccount, CategoryPolicy,
		CategoryTechnical, CategoryOther:
		return true
	default:
		return false
	}
}

type Severity string

const (
	SeverityUnknown  Severity = ""
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return true
	default:
		return false
	}
}

type Status string

const (
	StatusUnknown       Status = ""
	StatusOpen          Status = "open"
	StatusInvestigating Status = "investigating"
	StatusWaiting       Status = "waiting"
	StatusResolved      Status = "resolved"
	StatusClosed        Status = "closed"
)

func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusInvestigating, StatusWaiting, StatusResolved, StatusClosed:
		return true
	default:
		return false
	}
}

func CanTransition(from, to Status) bool {
	if !from.Valid() || !to.Valid() || from == to {
		return false
	}
	switch from {
	case StatusOpen:
		return to == StatusInvestigating || to == StatusWaiting || to == StatusResolved
	case StatusInvestigating:
		return to == StatusWaiting || to == StatusResolved
	case StatusWaiting:
		return to == StatusInvestigating || to == StatusResolved
	case StatusResolved:
		return to == StatusInvestigating || to == StatusClosed
	case StatusClosed:
		return to == StatusInvestigating
	default:
		return false
	}
}

type Visibility string

const (
	VisibilityUnknown         Visibility = ""
	VisibilityCustomerVisible Visibility = "customer_visible"
	VisibilityInternal        Visibility = "internal"
)

func (v Visibility) Valid() bool {
	return v == VisibilityCustomerVisible || v == VisibilityInternal
}

type EventType string

const (
	EventTypeUnknown       EventType = ""
	EventTypeOpened        EventType = "opened"
	EventTypeMessage       EventType = "message"
	EventTypeAssigned      EventType = "assigned"
	EventTypeStatusChanged EventType = "status_changed"
)

func (e EventType) Valid() bool {
	switch e {
	case EventTypeOpened, EventTypeMessage, EventTypeAssigned, EventTypeStatusChanged:
		return true
	default:
		return false
	}
}

type Event struct {
	ID         string
	CaseID     string
	Type       EventType
	Visibility Visibility
	ActorID    string
	Body       string
	Metadata   map[string]string
	OccurredAt time.Time
}

func (e Event) Validate() error {
	if e.ID == "" || e.CaseID == "" || e.ActorID == "" || e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: event identity, actor and time are required", ErrInvalidCase)
	}
	if !e.Type.Valid() || !e.Visibility.Valid() {
		return fmt.Errorf("%w: event type or visibility is invalid", ErrInvalidCase)
	}
	if utf8.RuneCountInString(e.Body) > 10000 {
		return fmt.Errorf("%w: event body is too long", ErrInvalidCase)
	}
	if e.Type == EventTypeMessage && strings.TrimSpace(e.Body) == "" {
		return fmt.Errorf("%w: message body is required", ErrInvalidCase)
	}
	if e.Metadata == nil {
		return fmt.Errorf("%w: event metadata must be initialized", ErrInvalidCase)
	}
	return nil
}

type Case struct {
	ID         string
	FamilyID   string
	OpenedBy   string
	Category   Category
	Severity   Severity
	Status     Status
	Subject    string
	AssignedTo string
	OpenedAt   time.Time
	SLADueAt   time.Time
	ResolvedAt *time.Time
	UpdatedAt  time.Time
	Version    uint64
	Events     []Event
}

type NewParams struct {
	ID           string
	FamilyID     string
	OpenedBy     string
	Category     Category
	Severity     Severity
	Subject      string
	OpenedAt     time.Time
	SLADueAt     time.Time
	InitialEvent Event
}

func New(params NewParams) (Case, error) {
	supportCase := Case{
		ID:        params.ID,
		FamilyID:  params.FamilyID,
		OpenedBy:  params.OpenedBy,
		Category:  params.Category,
		Severity:  params.Severity,
		Status:    StatusOpen,
		Subject:   strings.TrimSpace(params.Subject),
		OpenedAt:  params.OpenedAt,
		SLADueAt:  params.SLADueAt,
		UpdatedAt: params.OpenedAt,
		Version:   1,
		Events:    []Event{cloneEvent(params.InitialEvent)},
	}
	if err := supportCase.Validate(); err != nil {
		return Case{}, err
	}
	if len(supportCase.Events) != 1 || supportCase.Events[0].Type != EventTypeOpened ||
		supportCase.Events[0].Visibility != VisibilityCustomerVisible {
		return Case{}, fmt.Errorf("%w: initial customer-visible opened event is required", ErrInvalidCase)
	}
	return supportCase, nil
}

func (c Case) Validate() error {
	if c.ID == "" || c.FamilyID == "" || c.OpenedBy == "" || c.Version == 0 {
		return fmt.Errorf("%w: case identity and version are required", ErrInvalidCase)
	}
	if !c.Category.Valid() || !c.Severity.Valid() || !c.Status.Valid() {
		return fmt.Errorf("%w: category, severity or status is invalid", ErrInvalidCase)
	}
	subjectLength := utf8.RuneCountInString(strings.TrimSpace(c.Subject))
	if subjectLength == 0 || subjectLength > 200 {
		return fmt.Errorf("%w: subject must contain 1 to 200 characters", ErrInvalidCase)
	}
	invalidTimes := c.OpenedAt.IsZero() || c.SLADueAt.Before(c.OpenedAt) ||
		c.UpdatedAt.Before(c.OpenedAt)
	if invalidTimes {
		return fmt.Errorf("%w: case timestamps are invalid", ErrInvalidCase)
	}
	isTerminal := c.Status == StatusResolved || c.Status == StatusClosed
	if isTerminal != (c.ResolvedAt != nil) {
		return fmt.Errorf("%w: resolution timestamp does not match status", ErrInvalidCase)
	}
	if c.ResolvedAt != nil && c.ResolvedAt.Before(c.OpenedAt) {
		return fmt.Errorf("%w: resolution precedes opening", ErrInvalidCase)
	}
	if c.Events == nil {
		return fmt.Errorf("%w: events must be initialized", ErrInvalidCase)
	}
	for _, event := range c.Events {
		if event.CaseID != c.ID {
			return fmt.Errorf("%w: event belongs to another case", ErrInvalidCase)
		}
		if err := event.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (c Case) CustomerView() Case {
	visible := make([]Event, 0, len(c.Events))
	for _, event := range c.Events {
		if event.Visibility == VisibilityCustomerVisible {
			visible = append(visible, cloneEvent(event))
		}
	}
	c.Events = visible
	return c
}

func cloneCase(supportCase Case) Case {
	cloned := supportCase
	cloned.Events = make([]Event, 0, len(supportCase.Events))
	for _, event := range supportCase.Events {
		cloned.Events = append(cloned.Events, cloneEvent(event))
	}
	if supportCase.ResolvedAt != nil {
		value := *supportCase.ResolvedAt
		cloned.ResolvedAt = &value
	}
	return cloned
}

func cloneEvent(event Event) Event {
	cloned := event
	cloned.Metadata = make(map[string]string, len(event.Metadata))
	for key, value := range event.Metadata {
		cloned.Metadata[key] = value
	}
	return cloned
}
