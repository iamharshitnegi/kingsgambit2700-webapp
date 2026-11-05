package scheduling

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrDecisionNotFound = errors.New("scheduling: decision not found")
	ErrDecisionConflict = errors.New("scheduling: decision conflict")
)

type CancellationRecord struct {
	ID               string
	IdempotencyKey   string
	ActorID          string
	Input            CancellationInput
	Decision         CancellationDecision
	MakeUpObligation *MakeUpObligation
	CreatedAt        time.Time
}

type MakeUpStatus string

const (
	MakeUpStatusUnknown   MakeUpStatus = ""
	MakeUpStatusOpen      MakeUpStatus = "open"
	MakeUpStatusFulfilled MakeUpStatus = "fulfilled"
	MakeUpStatusCancelled MakeUpStatus = "cancelled"
)

// MakeUpObligation is the academy's durable responsibility to replace a
// cancelled class. Creating the cancellation record and this obligation is one
// repository operation so an affected family cannot be silently omitted.
type MakeUpObligation struct {
	ID              string
	SessionID       string
	StudentID       string
	PolicyVersionID string
	Status          MakeUpStatus
	Reason          ReasonCode
	CreatedBy       string
	CreatedAt       time.Time
}

type CompletionRecord struct {
	ID             string
	IdempotencyKey string
	Input          CompletionInput
	Decision       CompletionDecision
	CreatedAt      time.Time
}

type StoreCancellationResult struct {
	Record  CancellationRecord
	Created bool
}

type StoreCompletionResult struct {
	Record  CompletionRecord
	Created bool
}

// Repository stores policy decisions as immutable facts. Implementations must
// enforce idempotency keys atomically and reject reuse for different inputs.
type Repository interface {
	StoreCancellation(
		ctx context.Context,
		record CancellationRecord,
	) (StoreCancellationResult, error)
	StoreCompletion(
		ctx context.Context,
		record CompletionRecord,
	) (StoreCompletionResult, error)
	Cancellation(ctx context.Context, id string) (CancellationRecord, error)
	Completion(ctx context.Context, id string) (CompletionRecord, error)
	MakeUpObligation(ctx context.Context, id string) (MakeUpObligation, error)
}

type Service struct {
	policy     Policy
	repository Repository
}

func NewService(policy Policy, repository Repository) (*Service, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if repository == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrInvalidPolicy)
	}
	return &Service{policy: policy, repository: repository}, nil
}

type CancellationCommand struct {
	ID                 string
	IdempotencyKey     string
	ActorID            string
	MakeUpObligationID string
	Input              CancellationInput
	CreatedAt          time.Time
}

func (s *Service) DecideCancellation(
	ctx context.Context,
	command CancellationCommand,
) (StoreCancellationResult, error) {
	if err := ctx.Err(); err != nil {
		return StoreCancellationResult{}, err
	}
	if command.ID == "" ||
		command.IdempotencyKey == "" ||
		command.ActorID == "" ||
		command.CreatedAt.IsZero() {
		return StoreCancellationResult{}, fmt.Errorf("%w: decision metadata is required", ErrInvalidRequest)
	}

	decision, err := s.policy.EvaluateCancellation(command.Input)
	if err != nil {
		return StoreCancellationResult{}, err
	}
	record := CancellationRecord{
		ID:             command.ID,
		IdempotencyKey: command.IdempotencyKey,
		ActorID:        command.ActorID,
		Input:          command.Input,
		Decision:       decision,
		CreatedAt:      command.CreatedAt,
	}
	if decision.MakeUpAction == MakeUpActionCreate {
		if command.MakeUpObligationID == "" {
			return StoreCancellationResult{}, fmt.Errorf("%w: make-up obligation id is required", ErrInvalidRequest)
		}
		record.MakeUpObligation = &MakeUpObligation{
			ID:              command.MakeUpObligationID,
			SessionID:       command.Input.SessionID,
			StudentID:       command.Input.StudentID,
			PolicyVersionID: decision.PolicyVersionID,
			Status:          MakeUpStatusOpen,
			Reason:          decision.Reason,
			CreatedBy:       command.ActorID,
			CreatedAt:       command.CreatedAt,
		}
	}
	result, err := s.repository.StoreCancellation(ctx, record)
	if err != nil {
		return StoreCancellationResult{}, fmt.Errorf("storing cancellation decision: %w", err)
	}
	return result, nil
}

type CompletionCommand struct {
	ID             string
	IdempotencyKey string
	Input          CompletionInput
	CreatedAt      time.Time
}

func (s *Service) DecideCompletion(
	ctx context.Context,
	command CompletionCommand,
) (StoreCompletionResult, error) {
	if err := ctx.Err(); err != nil {
		return StoreCompletionResult{}, err
	}
	if command.ID == "" || command.IdempotencyKey == "" || command.CreatedAt.IsZero() {
		return StoreCompletionResult{}, fmt.Errorf("%w: decision metadata is required", ErrInvalidRequest)
	}

	decision, err := s.policy.EvaluateCompletion(command.Input)
	if err != nil {
		return StoreCompletionResult{}, err
	}
	record := CompletionRecord{
		ID:             command.ID,
		IdempotencyKey: command.IdempotencyKey,
		Input:          command.Input,
		Decision:       decision,
		CreatedAt:      command.CreatedAt,
	}
	result, err := s.repository.StoreCompletion(ctx, record)
	if err != nil {
		return StoreCompletionResult{}, fmt.Errorf("storing completion decision: %w", err)
	}
	return result, nil
}
