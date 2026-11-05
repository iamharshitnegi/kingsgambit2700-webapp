package entitlement

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	ErrAccountNotFound     = errors.New("entitlement: account not found")
	ErrEntryConflict       = errors.New("entitlement: entry conflict")
	ErrIdempotencyConflict = errors.New("entitlement: idempotency conflict")
	ErrInsufficientBalance = errors.New("entitlement: insufficient balance")
	ErrBalanceOverflow     = errors.New("entitlement: balance overflow")
	ErrInvalidCommand      = errors.New("entitlement: invalid command")
)

type ApplyResult struct {
	Entry   Entry
	Balance Balance
	Created bool
}

// Repository is append-only. Apply must atomically enforce account-scoped
// idempotency, entry-id uniqueness and nonnegative resulting balances.
type Repository interface {
	Account(ctx context.Context, accountID string) (Account, error)
	Apply(ctx context.Context, entry Entry) (ApplyResult, error)
	Balance(ctx context.Context, accountID string) (Balance, error)
	Entries(ctx context.Context, accountID string) ([]Entry, error)
	EntryPage(ctx context.Context, accountID string, request PageRequest) (EntryPage, error)
}

type Service struct {
	repository Repository
	authorizer Authorizer
}

func NewService(repository Repository, options ...Option) (*Service, error) {
	if repository == nil {
		return nil, fmt.Errorf("%w: repository is required", ErrInvalidCommand)
	}
	service := &Service{repository: repository}
	for _, option := range options {
		if option == nil {
			return nil, fmt.Errorf("%w: service option is required", ErrInvalidCommand)
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
}

type OperationCommand struct {
	EntryID        string
	AccountID      string
	StudentID      string
	IdempotencyKey string
	ReferenceType  string
	ReferenceID    string
	Units          int64
	Reason         string
	ActorID        string
	OccurredAt     time.Time
}

func (s *Service) Purchase(ctx context.Context, command OperationCommand) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindPurchased, Delta{Available: command.Units})
}

func (s *Service) Grant(ctx context.Context, command OperationCommand) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindGranted, Delta{Available: command.Units})
}

func (s *Service) Reserve(ctx context.Context, command OperationCommand) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindReserved, Delta{
		Available: -command.Units,
		Reserved:  command.Units,
	})
}

type ConsumptionSource string

const (
	ConsumptionSourceUnknown   ConsumptionSource = ""
	ConsumptionSourceAvailable ConsumptionSource = "available"
	ConsumptionSourceReserved  ConsumptionSource = "reserved"
)

type ConsumeCommand struct {
	OperationCommand
	Source ConsumptionSource
}

func (s *Service) Consume(ctx context.Context, command ConsumeCommand) (ApplyResult, error) {
	var delta Delta
	switch command.Source {
	case ConsumptionSourceAvailable:
		delta = Delta{Available: -command.Units, Consumed: command.Units}
	case ConsumptionSourceReserved:
		delta = Delta{Reserved: -command.Units, Consumed: command.Units}
	default:
		return ApplyResult{}, fmt.Errorf("%w: consumption source is required", ErrInvalidCommand)
	}
	return s.applyUnits(ctx, command.OperationCommand, EntryKindConsumed, delta)
}

func (s *Service) Release(ctx context.Context, command OperationCommand) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindReleased, Delta{
		Available: command.Units,
		Reserved:  -command.Units,
	})
}

func (s *Service) ReverseConsumption(
	ctx context.Context,
	command OperationCommand,
) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindReversed, Delta{
		Available: command.Units,
		Consumed:  -command.Units,
	})
}

func (s *Service) Expire(ctx context.Context, command OperationCommand) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindExpired, Delta{Available: -command.Units})
}

func (s *Service) Refund(ctx context.Context, command OperationCommand) (ApplyResult, error) {
	return s.applyUnits(ctx, command, EntryKindRefunded, Delta{Available: -command.Units})
}

type AdjustmentCommand struct {
	EntryID        string
	AccountID      string
	StudentID      string
	IdempotencyKey string
	ReferenceType  string
	ReferenceID    string
	Delta          Delta
	Reason         string
	ActorID        string
	OccurredAt     time.Time
}

func (s *Service) Adjust(ctx context.Context, command AdjustmentCommand) (ApplyResult, error) {
	entry := Entry{
		ID:             command.EntryID,
		AccountID:      command.AccountID,
		StudentID:      command.StudentID,
		Kind:           EntryKindAdjusted,
		Delta:          command.Delta,
		IdempotencyKey: command.IdempotencyKey,
		ReferenceType:  command.ReferenceType,
		ReferenceID:    command.ReferenceID,
		Reason:         command.Reason,
		ActorID:        command.ActorID,
		OccurredAt:     command.OccurredAt,
	}
	return s.apply(ctx, entry)
}

func (s *Service) applyUnits(
	ctx context.Context,
	command OperationCommand,
	kind EntryKind,
	delta Delta,
) (ApplyResult, error) {
	if command.Units <= 0 {
		return ApplyResult{}, fmt.Errorf("%w: units must be positive", ErrInvalidCommand)
	}
	entry := Entry{
		ID:             command.EntryID,
		AccountID:      command.AccountID,
		StudentID:      command.StudentID,
		Kind:           kind,
		Delta:          delta,
		IdempotencyKey: command.IdempotencyKey,
		ReferenceType:  command.ReferenceType,
		ReferenceID:    command.ReferenceID,
		Reason:         command.Reason,
		ActorID:        command.ActorID,
		OccurredAt:     command.OccurredAt,
	}
	return s.apply(ctx, entry)
}

func (s *Service) apply(ctx context.Context, entry Entry) (ApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	if entry.ReferenceType == "" {
		entry.ReferenceType = "external"
	}
	digest, err := requestDigest(entry)
	if err != nil {
		return ApplyResult{}, err
	}
	entry.RequestDigest = digest
	if err := entry.Validate(); err != nil {
		return ApplyResult{}, err
	}

	result, err := s.repository.Apply(ctx, entry)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("applying ledger entry: %w", err)
	}
	return result, nil
}

func requestDigest(entry Entry) ([32]byte, error) {
	payload, err := json.Marshal(struct {
		AccountID     string    `json:"accountId"`
		StudentID     string    `json:"studentId"`
		Kind          EntryKind `json:"kind"`
		Delta         Delta     `json:"delta"`
		ReferenceType string    `json:"referenceType"`
		ReferenceID   string    `json:"referenceId"`
		Reason        string    `json:"reason"`
		ActorID       string    `json:"actorId"`
	}{
		AccountID:     entry.AccountID,
		StudentID:     entry.StudentID,
		Kind:          entry.Kind,
		Delta:         entry.Delta,
		ReferenceType: entry.ReferenceType,
		ReferenceID:   entry.ReferenceID,
		Reason:        entry.Reason,
		ActorID:       entry.ActorID,
	})
	if err != nil {
		return [32]byte{}, fmt.Errorf("entitlement: encoding request digest: %w", err)
	}
	return sha256.Sum256(payload), nil
}
