package support

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var (
	ErrNotFound         = errors.New("support: case not found")
	ErrConflict         = errors.New("support: case conflict")
	ErrPermissionDenied = errors.New("support: permission denied")
	ErrInvalidCommand   = errors.New("support: invalid command")
)

type Access struct {
	CanViewFamily bool
	CanManage     bool
}

type Authorizer interface {
	SupportAccess(ctx context.Context, actorID, familyID string) (Access, error)
	AuthorizeSupportAssignee(ctx context.Context, actorID, assigneeID string) error
}

type Cursor struct {
	OpenedAt time.Time
	ID       string
}

type ListOptions struct {
	Limit  int
	Cursor *Cursor
}

type Page struct {
	Items      []Case
	NextCursor *Cursor
}

type CreateMutation struct {
	Case      Case
	Event     Event
	RequestID string
}

type AssignMutation struct {
	CaseID          string
	EventID         string
	ExpectedVersion uint64
	ActorID         string
	AssigneeID      string
	Reason          string
	OccurredAt      time.Time
	RequestID       string
}

type StatusMutation struct {
	CaseID          string
	EventID         string
	ExpectedVersion uint64
	ActorID         string
	FromStatus      Status
	ToStatus        Status
	Reason          string
	OccurredAt      time.Time
	RequestID       string
}

type EventMutation struct {
	Event           Event
	ExpectedVersion uint64
	RequestID       string
}

type Repository interface {
	Create(ctx context.Context, mutation CreateMutation) (Case, error)
	Summary(ctx context.Context, caseID string) (Case, error)
	Case(ctx context.Context, caseID string, includeInternal bool) (Case, error)
	List(ctx context.Context, familyID string, options ListOptions) (Page, error)
	Assign(ctx context.Context, mutation AssignMutation) (Case, error)
	ChangeStatus(ctx context.Context, mutation StatusMutation) (Case, error)
	AppendEvent(ctx context.Context, mutation EventMutation) (Case, error)
}

type Config struct {
	LowSLA      time.Duration
	MediumSLA   time.Duration
	HighSLA     time.Duration
	CriticalSLA time.Duration
}

func DefaultConfig() Config {
	return Config{
		LowSLA:      72 * time.Hour,
		MediumSLA:   24 * time.Hour,
		HighSLA:     4 * time.Hour,
		CriticalSLA: time.Hour,
	}
}

type Service struct {
	repository Repository
	authorizer Authorizer
	config     Config
}

func NewService(repository Repository, authorizer Authorizer, config Config) (*Service, error) {
	if repository == nil || authorizer == nil {
		return nil, fmt.Errorf("%w: repository and authorizer are required", ErrInvalidCommand)
	}
	invalidSLA := config.LowSLA <= 0 || config.MediumSLA <= 0 ||
		config.HighSLA <= 0 || config.CriticalSLA <= 0
	if invalidSLA {
		return nil, fmt.Errorf("%w: positive sla durations are required", ErrInvalidCommand)
	}
	return &Service{repository: repository, authorizer: authorizer, config: config}, nil
}

type CreateCommand struct {
	CaseID     string
	EventID    string
	FamilyID   string
	Category   Category
	Severity   Severity
	Subject    string
	Message    string
	ActorID    string
	OccurredAt time.Time
	RequestID  string
}

func (s *Service) Create(ctx context.Context, command CreateCommand) (Case, error) {
	if err := ctx.Err(); err != nil {
		return Case{}, err
	}
	if err := validateCreateCommand(command); err != nil {
		return Case{}, err
	}
	access, err := s.authorizer.SupportAccess(ctx, command.ActorID, command.FamilyID)
	if err != nil {
		return Case{}, fmt.Errorf("authorizing support case creation: %w", err)
	}
	if !access.CanViewFamily {
		return Case{}, ErrPermissionDenied
	}

	event := Event{
		ID:         command.EventID,
		CaseID:     command.CaseID,
		Type:       EventTypeOpened,
		Visibility: VisibilityCustomerVisible,
		ActorID:    command.ActorID,
		Body:       strings.TrimSpace(command.Message),
		Metadata:   map[string]string{},
		OccurredAt: command.OccurredAt.UTC(),
	}
	supportCase, err := New(NewParams{
		ID:           command.CaseID,
		FamilyID:     command.FamilyID,
		OpenedBy:     command.ActorID,
		Category:     command.Category,
		Severity:     command.Severity,
		Subject:      command.Subject,
		OpenedAt:     command.OccurredAt.UTC(),
		SLADueAt:     command.OccurredAt.UTC().Add(s.sla(command.Severity)),
		InitialEvent: event,
	})
	if err != nil {
		return Case{}, fmt.Errorf("creating support case: %w", err)
	}
	created, err := s.repository.Create(ctx, CreateMutation{
		Case:      supportCase,
		Event:     event,
		RequestID: strings.TrimSpace(command.RequestID),
	})
	if err != nil {
		return Case{}, fmt.Errorf("saving support case: %w", err)
	}
	return created.CustomerView(), nil
}

type ListCommand struct {
	FamilyID string
	ActorID  string
	Options  ListOptions
}

func (s *Service) List(ctx context.Context, command ListCommand) (Page, error) {
	if err := validateListCommand(command); err != nil {
		return Page{}, err
	}
	access, err := s.authorizer.SupportAccess(ctx, command.ActorID, command.FamilyID)
	if err != nil {
		return Page{}, fmt.Errorf("authorizing support case list: %w", err)
	}
	if !access.CanViewFamily && !access.CanManage {
		return Page{}, ErrPermissionDenied
	}
	page, err := s.repository.List(ctx, command.FamilyID, command.Options)
	if err != nil {
		return Page{}, fmt.Errorf("listing support cases: %w", err)
	}
	if page.Items == nil {
		page.Items = []Case{}
	}
	for index := range page.Items {
		page.Items[index].Events = []Event{}
	}
	return page, nil
}

func (s *Service) View(ctx context.Context, actorID, caseID string) (Case, error) {
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(caseID) {
		return Case{}, fmt.Errorf("%w: actor and case ids must be UUIDv4 or UUIDv7", ErrInvalidCommand)
	}
	summary, access, err := s.caseAccess(ctx, actorID, caseID)
	if err != nil {
		return Case{}, err
	}
	loaded, err := s.repository.Case(ctx, summary.ID, access.CanManage)
	if err != nil {
		return Case{}, fmt.Errorf("loading support case timeline: %w", err)
	}
	if !access.CanManage {
		loaded = loaded.CustomerView()
	}
	return loaded, nil
}

type AssignCommand struct {
	CaseID          string
	EventID         string
	ExpectedVersion uint64
	AssigneeID      string
	ActorID         string
	Reason          string
	OccurredAt      time.Time
	RequestID       string
}

func (s *Service) Assign(ctx context.Context, command AssignCommand) (Case, error) {
	if err := validateAssignCommand(command); err != nil {
		return Case{}, err
	}
	current, access, err := s.caseAccess(ctx, command.ActorID, command.CaseID)
	if err != nil {
		return Case{}, err
	}
	if !access.CanManage {
		return Case{}, ErrPermissionDenied
	}
	if current.Version != command.ExpectedVersion {
		return Case{}, versionConflict(command.ExpectedVersion, current.Version)
	}
	if err := s.authorizer.AuthorizeSupportAssignee(
		ctx,
		command.ActorID,
		command.AssigneeID,
	); err != nil {
		return Case{}, fmt.Errorf("authorizing support assignee: %w", err)
	}
	updated, err := s.repository.Assign(ctx, AssignMutation{
		CaseID:          command.CaseID,
		EventID:         command.EventID,
		ExpectedVersion: command.ExpectedVersion,
		ActorID:         command.ActorID,
		AssigneeID:      command.AssigneeID,
		Reason:          strings.TrimSpace(command.Reason),
		OccurredAt:      command.OccurredAt.UTC(),
		RequestID:       strings.TrimSpace(command.RequestID),
	})
	if err != nil {
		return Case{}, fmt.Errorf("assigning support case: %w", err)
	}
	return updated, nil
}

type ChangeStatusCommand struct {
	CaseID          string
	EventID         string
	ExpectedVersion uint64
	Status          Status
	ActorID         string
	Reason          string
	OccurredAt      time.Time
	RequestID       string
}

func (s *Service) ChangeStatus(ctx context.Context, command ChangeStatusCommand) (Case, error) {
	if err := validateStatusCommand(command); err != nil {
		return Case{}, err
	}
	current, access, err := s.caseAccess(ctx, command.ActorID, command.CaseID)
	if err != nil {
		return Case{}, err
	}
	if !access.CanManage {
		return Case{}, ErrPermissionDenied
	}
	if current.Version != command.ExpectedVersion {
		return Case{}, versionConflict(command.ExpectedVersion, current.Version)
	}
	if !CanTransition(current.Status, command.Status) {
		return Case{}, fmt.Errorf(
			"%w: cannot transition from %q to %q",
			ErrInvalidCommand,
			current.Status,
			command.Status,
		)
	}
	updated, err := s.repository.ChangeStatus(ctx, StatusMutation{
		CaseID:          command.CaseID,
		EventID:         command.EventID,
		ExpectedVersion: command.ExpectedVersion,
		ActorID:         command.ActorID,
		FromStatus:      current.Status,
		ToStatus:        command.Status,
		Reason:          strings.TrimSpace(command.Reason),
		OccurredAt:      command.OccurredAt.UTC(),
		RequestID:       strings.TrimSpace(command.RequestID),
	})
	if err != nil {
		return Case{}, fmt.Errorf("changing support case status: %w", err)
	}
	return updated, nil
}

type AppendEventCommand struct {
	CaseID          string
	EventID         string
	ExpectedVersion uint64
	Visibility      Visibility
	Body            string
	ActorID         string
	OccurredAt      time.Time
	RequestID       string
}

func (s *Service) AppendEvent(ctx context.Context, command AppendEventCommand) (Case, error) {
	if err := validateAppendEventCommand(command); err != nil {
		return Case{}, err
	}
	current, access, err := s.caseAccess(ctx, command.ActorID, command.CaseID)
	if err != nil {
		return Case{}, err
	}
	if !access.CanViewFamily && !access.CanManage {
		return Case{}, ErrPermissionDenied
	}
	if command.Visibility == VisibilityInternal && !access.CanManage {
		return Case{}, ErrPermissionDenied
	}
	if current.Version != command.ExpectedVersion {
		return Case{}, versionConflict(command.ExpectedVersion, current.Version)
	}
	event := Event{
		ID:         command.EventID,
		CaseID:     command.CaseID,
		Type:       EventTypeMessage,
		Visibility: command.Visibility,
		ActorID:    command.ActorID,
		Body:       strings.TrimSpace(command.Body),
		Metadata:   map[string]string{},
		OccurredAt: command.OccurredAt.UTC(),
	}
	if err := event.Validate(); err != nil {
		return Case{}, err
	}
	updated, err := s.repository.AppendEvent(ctx, EventMutation{
		Event:           event,
		ExpectedVersion: command.ExpectedVersion,
		RequestID:       strings.TrimSpace(command.RequestID),
	})
	if err != nil {
		return Case{}, fmt.Errorf("appending support case event: %w", err)
	}
	if !access.CanManage {
		updated = updated.CustomerView()
	}
	return updated, nil
}

func (s *Service) caseAccess(
	ctx context.Context,
	actorID string,
	caseID string,
) (Case, Access, error) {
	current, err := s.repository.Summary(ctx, caseID)
	if err != nil {
		return Case{}, Access{}, fmt.Errorf("loading support case: %w", err)
	}
	access, err := s.authorizer.SupportAccess(ctx, actorID, current.FamilyID)
	if err != nil {
		return Case{}, Access{}, fmt.Errorf("authorizing support case access: %w", err)
	}
	if !access.CanViewFamily && !access.CanManage {
		return Case{}, Access{}, ErrPermissionDenied
	}
	return current, access, nil
}

func (s *Service) sla(severity Severity) time.Duration {
	switch severity {
	case SeverityCritical:
		return s.config.CriticalSLA
	case SeverityHigh:
		return s.config.HighSLA
	case SeverityMedium:
		return s.config.MediumSLA
	default:
		return s.config.LowSLA
	}
}

func validateCreateCommand(command CreateCommand) error {
	validIDs := platformid.Valid(command.CaseID) && platformid.Valid(command.EventID) &&
		platformid.ValidReference(command.FamilyID) && platformid.ValidReference(command.ActorID)
	if !validIDs || command.OccurredAt.IsZero() {
		return fmt.Errorf("%w: new case/event IDs must be UUIDv7, existing references must be UUIDv4 or UUIDv7, and time is required", ErrInvalidCommand)
	}
	if !command.Category.Valid() || !command.Severity.Valid() {
		return fmt.Errorf("%w: category or severity is invalid", ErrInvalidCommand)
	}
	if strings.TrimSpace(command.Subject) == "" || strings.TrimSpace(command.Message) == "" {
		return fmt.Errorf("%w: subject and message are required", ErrInvalidCommand)
	}
	return nil
}

func validateListCommand(command ListCommand) error {
	if !platformid.ValidReference(command.FamilyID) || !platformid.ValidReference(command.ActorID) {
		return fmt.Errorf("%w: actor and family ids must be UUIDv4 or UUIDv7", ErrInvalidCommand)
	}
	if command.Options.Limit < 1 || command.Options.Limit > 100 {
		return fmt.Errorf("%w: list limit must be between 1 and 100", ErrInvalidCommand)
	}
	if command.Options.Cursor != nil {
		if command.Options.Cursor.OpenedAt.IsZero() || !platformid.ValidReference(command.Options.Cursor.ID) {
			return fmt.Errorf("%w: list cursor is invalid", ErrInvalidCommand)
		}
	}
	return nil
}

func validateAssignCommand(command AssignCommand) error {
	validIDs := platformid.ValidReference(command.CaseID) && platformid.Valid(command.EventID) &&
		platformid.ValidReference(command.AssigneeID) && platformid.ValidReference(command.ActorID)
	if !validIDs || command.ExpectedVersion == 0 || command.OccurredAt.IsZero() {
		return fmt.Errorf("%w: assignment identity, version and time are required", ErrInvalidCommand)
	}
	if strings.TrimSpace(command.Reason) == "" {
		return fmt.Errorf("%w: assignment reason is required", ErrInvalidCommand)
	}
	return nil
}

func validateStatusCommand(command ChangeStatusCommand) error {
	validIDs := platformid.ValidReference(command.CaseID) && platformid.Valid(command.EventID) &&
		platformid.ValidReference(command.ActorID)
	if !validIDs || command.ExpectedVersion == 0 || command.OccurredAt.IsZero() {
		return fmt.Errorf("%w: status identity, version and time are required", ErrInvalidCommand)
	}
	if !command.Status.Valid() || strings.TrimSpace(command.Reason) == "" {
		return fmt.Errorf("%w: status and reason are required", ErrInvalidCommand)
	}
	return nil
}

func validateAppendEventCommand(command AppendEventCommand) error {
	validIDs := platformid.ValidReference(command.CaseID) && platformid.Valid(command.EventID) &&
		platformid.ValidReference(command.ActorID)
	if !validIDs || command.ExpectedVersion == 0 || command.OccurredAt.IsZero() {
		return fmt.Errorf("%w: event identity, version and time are required", ErrInvalidCommand)
	}
	if !command.Visibility.Valid() || strings.TrimSpace(command.Body) == "" {
		return fmt.Errorf("%w: visibility and message body are required", ErrInvalidCommand)
	}
	return nil
}

func versionConflict(expected, current uint64) error {
	return fmt.Errorf("%w: expected version %d, current %d", ErrConflict, expected, current)
}
