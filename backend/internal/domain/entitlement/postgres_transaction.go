package entitlement

import (
	"context"
	"crypto/subtle"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ApplyInTx composes a ledger operation with the owning scheduling transaction.
// The caller owns commit/rollback and must authorize the business operation.
func ApplyInTx(ctx context.Context, tx pgx.Tx, entry Entry) (ApplyResult, error) {
	digest, err := requestDigest(entry)
	if err != nil {
		return ApplyResult{}, err
	}
	entry.RequestDigest = digest
	if err := entry.Validate(); err != nil {
		return ApplyResult{}, err
	}
	account, err := loadAccount(ctx, tx, entry.AccountID, true)
	if err != nil {
		return ApplyResult{}, err
	}
	if account.StudentID != entry.StudentID {
		return ApplyResult{}, ErrEntryConflict
	}
	existing, found, err := loadIdempotentEntry(ctx, tx, account, entry.IdempotencyKey)
	if err != nil {
		return ApplyResult{}, err
	}
	if found {
		if subtle.ConstantTimeCompare(existing.Entry.RequestDigest[:], digest[:]) != 1 {
			return ApplyResult{}, ErrIdempotencyConflict
		}
		return existing, nil
	}
	balance, err := loadBalance(ctx, tx, entry.AccountID)
	if err != nil {
		return ApplyResult{}, err
	}
	next, ok := balance.apply(entry.Delta)
	if !ok {
		return ApplyResult{}, ErrBalanceOverflow
	}
	if !next.valid() {
		return ApplyResult{}, ErrInsufficientBalance
	}
	if err := insertEntry(ctx, tx, entry, next); err != nil {
		return ApplyResult{}, fmt.Errorf("inserting transaction ledger entry: %w", err)
	}
	if err := insertAuditEvent(ctx, tx, account, entry, next); err != nil {
		return ApplyResult{}, fmt.Errorf("auditing transaction ledger entry: %w", err)
	}
	return ApplyResult{Entry: entry, Balance: next, Created: true}, nil
}
