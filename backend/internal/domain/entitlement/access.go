package entitlement

import (
	"context"
	"errors"
	"fmt"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var ErrPermissionDenied = errors.New("entitlement: permission denied")

type Action string

const (
	ActionUnknown Action = ""
	ActionView    Action = "view"
	ActionGrant   Action = "grant"
)

type Authorizer interface {
	AuthorizeEntitlement(
		ctx context.Context,
		actorID string,
		studentID string,
		action Action,
	) error
}

type Option func(*Service) error

func WithAuthorizer(authorizer Authorizer) Option {
	return func(service *Service) error {
		if authorizer == nil {
			return fmt.Errorf("%w: authorizer is required", ErrInvalidCommand)
		}
		service.authorizer = authorizer
		return nil
	}
}

type Account struct {
	ID          string
	EnrolmentID string
	StudentID   string
	Unit        string
	CreatedAt   time.Time
}

type AccountBalance struct {
	Account Account
	Balance Balance
}

type EntryCursor struct {
	OccurredAt time.Time
	ID         string
}

type PageRequest struct {
	Cursor *EntryCursor
	Limit  int
}

type EntryPage struct {
	Items      []Entry
	NextCursor *EntryCursor
}

type HistoryCommand struct {
	ActorID   string
	AccountID string
	Cursor    *EntryCursor
	Limit     int
}

type GrantCommand struct {
	EntryID        string
	AccountID      string
	IdempotencyKey string
	ReferenceID    string
	Units          int64
	Reason         string
	ActorID        string
	OccurredAt     time.Time
}

func (s *Service) ViewBalance(
	ctx context.Context,
	actorID string,
	accountID string,
) (AccountBalance, error) {
	account, err := s.authorizedAccount(ctx, actorID, accountID, ActionView)
	if err != nil {
		return AccountBalance{}, err
	}
	balance, err := s.repository.Balance(ctx, account.ID)
	if err != nil {
		return AccountBalance{}, fmt.Errorf("loading entitlement balance: %w", err)
	}
	return AccountBalance{Account: account, Balance: balance}, nil
}

func (s *Service) History(ctx context.Context, command HistoryCommand) (EntryPage, error) {
	if command.Limit == 0 {
		command.Limit = 50
	}
	if command.Limit < 1 || command.Limit > 100 {
		return EntryPage{}, fmt.Errorf("%w: history limit must be between 1 and 100", ErrInvalidCommand)
	}
	if command.Cursor != nil &&
		(!platformid.ValidReference(command.Cursor.ID) || command.Cursor.OccurredAt.IsZero()) {
		return EntryPage{}, fmt.Errorf("%w: history cursor is invalid", ErrInvalidCommand)
	}
	account, err := s.authorizedAccount(
		ctx,
		command.ActorID,
		command.AccountID,
		ActionView,
	)
	if err != nil {
		return EntryPage{}, err
	}
	page, err := s.repository.EntryPage(ctx, account.ID, PageRequest{
		Cursor: command.Cursor,
		Limit:  command.Limit,
	})
	if err != nil {
		return EntryPage{}, fmt.Errorf("loading entitlement history: %w", err)
	}
	return page, nil
}

func (s *Service) GrantForActor(
	ctx context.Context,
	command GrantCommand,
) (ApplyResult, error) {
	if !platformid.Valid(command.EntryID) {
		return ApplyResult{}, fmt.Errorf("%w: entry id must be a uuidv7", ErrInvalidCommand)
	}
	account, err := s.authorizedAccount(
		ctx,
		command.ActorID,
		command.AccountID,
		ActionGrant,
	)
	if err != nil {
		return ApplyResult{}, err
	}
	return s.Grant(ctx, OperationCommand{
		EntryID:        command.EntryID,
		AccountID:      account.ID,
		StudentID:      account.StudentID,
		IdempotencyKey: command.IdempotencyKey,
		ReferenceType:  "manual_grant",
		ReferenceID:    command.ReferenceID,
		Units:          command.Units,
		Reason:         command.Reason,
		ActorID:        command.ActorID,
		OccurredAt:     command.OccurredAt,
	})
}

func (s *Service) authorizedAccount(
	ctx context.Context,
	actorID string,
	accountID string,
	action Action,
) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(accountID) {
		return Account{}, fmt.Errorf("%w: actor and account must be UUIDv4 or UUIDv7 values", ErrInvalidCommand)
	}
	account, err := s.repository.Account(ctx, accountID)
	if err != nil {
		return Account{}, fmt.Errorf("loading entitlement account: %w", err)
	}
	if s.authorizer == nil {
		return Account{}, ErrPermissionDenied
	}
	if err := s.authorizer.AuthorizeEntitlement(
		ctx,
		actorID,
		account.StudentID,
		action,
	); err != nil {
		return Account{}, fmt.Errorf("authorizing entitlement %s: %w", action, err)
	}
	return account, nil
}
