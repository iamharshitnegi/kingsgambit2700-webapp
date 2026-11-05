package report

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound         = errors.New("report: not found")
	ErrConflict         = errors.New("report: conflict")
	ErrPermissionDenied = errors.New("report: permission denied")
	ErrInvalidCommand   = errors.New("report: invalid command")
)

// Repository persists the report aggregate with optimistic concurrency.
// expectedRevision is zero for create and the previously loaded revision for
// updates. Published versions may never be updated in place by an adapter.
type Repository interface {
	Report(ctx context.Context, reportID string) (Report, error)
	Save(ctx context.Context, report Report, expectedRevision uint64) error
}

type Action string

const (
	ActionUnknown         Action = ""
	ActionCreateDraft     Action = "create_draft"
	ActionEditDraft       Action = "edit_draft"
	ActionPublish         Action = "publish"
	ActionStartCorrection Action = "start_correction"
	ActionView            Action = "view"
	ActionAcknowledge     Action = "acknowledge"
)

// Authorizer evaluates report access against the actor, student and action.
// Typical implementations grant coaches draft access only to assigned students,
// publication only to academic managers, and view access through family scope.
type Authorizer interface {
	AuthorizeReport(
		ctx context.Context,
		actorID string,
		studentID string,
		action Action,
	) error
}

type Service struct {
	repository Repository
	authorizer Authorizer
}

func NewService(repository Repository, authorizer Authorizer) (*Service, error) {
	if repository == nil || authorizer == nil {
		return nil, fmt.Errorf("%w: repository and authorizer are required", ErrInvalidCommand)
	}
	return &Service{repository: repository, authorizer: authorizer}, nil
}

type CreateCommand struct {
	ReportID        string
	StudentID       string
	Year            int
	Month           time.Month
	VersionID       string
	RubricVersionID string
	Content         Content
	ActorID         string
	CreatedAt       time.Time
}

func (s *Service) CreateMonthly(ctx context.Context, command CreateCommand) (Report, error) {
	if err := ctx.Err(); err != nil {
		return Report{}, err
	}
	hasMissingIdentity := command.ReportID == "" || command.StudentID == "" ||
		command.VersionID == "" || command.RubricVersionID == "" ||
		command.ActorID == "" || command.CreatedAt.IsZero()
	if hasMissingIdentity {
		return Report{}, fmt.Errorf("%w: report identity and creation metadata are required", ErrInvalidCommand)
	}
	period, err := NewMonthlyPeriod(command.Year, command.Month)
	if err != nil {
		return Report{}, err
	}
	if err := s.authorizer.AuthorizeReport(
		ctx,
		command.ActorID,
		command.StudentID,
		ActionCreateDraft,
	); err != nil {
		return Report{}, fmt.Errorf("authorizing monthly report creation: %w", err)
	}
	report, err := New(NewParams{
		ID:              command.ReportID,
		StudentID:       command.StudentID,
		Period:          period,
		VersionID:       command.VersionID,
		RubricVersionID: command.RubricVersionID,
		Content:         command.Content,
		ActorID:         command.ActorID,
		CreatedAt:       command.CreatedAt,
	})
	if err != nil {
		return Report{}, fmt.Errorf("creating monthly report: %w", err)
	}
	if err := s.repository.Save(ctx, report, 0); err != nil {
		return Report{}, fmt.Errorf("saving monthly report: %w", err)
	}
	return report, nil
}

type EditCommand struct {
	ReportID         string
	VersionID        string
	ExpectedRevision uint64
	Content          Content
	ActorID          string
	UpdatedAt        time.Time
}

func (s *Service) EditDraft(ctx context.Context, command EditCommand) (Report, error) {
	current, err := s.loadAuthorized(
		ctx,
		command.ActorID,
		command.ReportID,
		command.ExpectedRevision,
		ActionEditDraft,
	)
	if err != nil {
		return Report{}, err
	}
	updated, err := current.EditDraft(command.VersionID, command.Content, command.ActorID, command.UpdatedAt)
	if err != nil {
		return Report{}, err
	}
	if err := s.repository.Save(ctx, updated, current.Revision()); err != nil {
		return Report{}, fmt.Errorf("saving report draft: %w", err)
	}
	return updated, nil
}

type PublishCommand struct {
	ReportID         string
	VersionID        string
	ExpectedRevision uint64
	ActorID          string
	PublishedAt      time.Time
}

func (s *Service) Publish(ctx context.Context, command PublishCommand) (Report, error) {
	current, err := s.loadAuthorized(
		ctx,
		command.ActorID,
		command.ReportID,
		command.ExpectedRevision,
		ActionPublish,
	)
	if err != nil {
		return Report{}, err
	}
	updated, err := current.Publish(command.VersionID, command.ActorID, command.PublishedAt)
	if err != nil {
		return Report{}, err
	}
	if err := s.repository.Save(ctx, updated, current.Revision()); err != nil {
		return Report{}, fmt.Errorf("saving published report: %w", err)
	}
	return updated, nil
}

type CorrectCommand struct {
	ReportID         string
	SourceVersionID  string
	NewVersionID     string
	ExpectedRevision uint64
	ActorID          string
	CreatedAt        time.Time
}

func (s *Service) StartCorrection(ctx context.Context, command CorrectCommand) (Report, error) {
	current, err := s.loadAuthorized(
		ctx,
		command.ActorID,
		command.ReportID,
		command.ExpectedRevision,
		ActionStartCorrection,
	)
	if err != nil {
		return Report{}, err
	}
	updated, err := current.StartCorrection(
		command.NewVersionID,
		command.SourceVersionID,
		command.ActorID,
		command.CreatedAt,
	)
	if err != nil {
		return Report{}, err
	}
	if err := s.repository.Save(ctx, updated, current.Revision()); err != nil {
		return Report{}, fmt.Errorf("saving report correction: %w", err)
	}
	return updated, nil
}

type ReceiptCommand struct {
	ReportID         string
	VersionID        string
	ViewerID         string
	ExpectedRevision uint64
	OccurredAt       time.Time
}

func (s *Service) RecordView(ctx context.Context, command ReceiptCommand) (Report, error) {
	current, err := s.loadAuthorized(
		ctx,
		command.ViewerID,
		command.ReportID,
		command.ExpectedRevision,
		ActionView,
	)
	if err != nil {
		return Report{}, err
	}
	updated, changed, err := current.RecordView(command.VersionID, command.ViewerID, command.OccurredAt)
	if err != nil {
		return Report{}, err
	}
	if !changed {
		return updated, nil
	}
	if err := s.repository.Save(ctx, updated, current.Revision()); err != nil {
		return Report{}, fmt.Errorf("saving report view: %w", err)
	}
	return updated, nil
}

func (s *Service) Acknowledge(ctx context.Context, command ReceiptCommand) (Report, error) {
	current, err := s.loadAuthorized(
		ctx,
		command.ViewerID,
		command.ReportID,
		command.ExpectedRevision,
		ActionAcknowledge,
	)
	if err != nil {
		return Report{}, err
	}
	updated, changed, err := current.Acknowledge(command.VersionID, command.ViewerID, command.OccurredAt)
	if err != nil {
		return Report{}, err
	}
	if !changed {
		return updated, nil
	}
	if err := s.repository.Save(ctx, updated, current.Revision()); err != nil {
		return Report{}, fmt.Errorf("saving report acknowledgement: %w", err)
	}
	return updated, nil
}

func (s *Service) loadAuthorized(
	ctx context.Context,
	actorID string,
	reportID string,
	expectedRevision uint64,
	action Action,
) (Report, error) {
	if actorID == "" || reportID == "" || expectedRevision == 0 || action == ActionUnknown {
		return Report{}, fmt.Errorf(
			"%w: actor, report id, expected revision and action are required",
			ErrInvalidCommand,
		)
	}
	current, err := s.repository.Report(ctx, reportID)
	if err != nil {
		return Report{}, fmt.Errorf("loading report: %w", err)
	}
	if err := s.authorizer.AuthorizeReport(
		ctx,
		actorID,
		current.StudentID(),
		action,
	); err != nil {
		return Report{}, fmt.Errorf("authorizing report action: %w", err)
	}
	if current.Revision() != expectedRevision {
		return Report{}, fmt.Errorf(
			"%w: expected revision %d, current %d",
			ErrConflict,
			expectedRevision,
			current.Revision(),
		)
	}
	return current, nil
}
