package entitlement

import (
	"context"
	"crypto/subtle"
	"sort"
	"sync"
)

type operationKey struct {
	accountID      string
	idempotencyKey string
}

type MemoryRepository struct {
	mu         sync.RWMutex
	entries    map[string][]Entry
	balances   map[string]Balance
	accounts   map[string]Account
	operations map[operationKey]ApplyResult
	entryIDs   map[string]struct{}
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		entries:    make(map[string][]Entry),
		balances:   make(map[string]Balance),
		accounts:   make(map[string]Account),
		operations: make(map[operationKey]ApplyResult),
		entryIDs:   make(map[string]struct{}),
	}
}

var _ Repository = (*MemoryRepository)(nil)

func (r *MemoryRepository) Account(ctx context.Context, accountID string) (Account, error) {
	if err := ctx.Err(); err != nil {
		return Account{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	account, exists := r.accounts[accountID]
	if !exists {
		return Account{}, ErrAccountNotFound
	}
	return account, nil
}

func (r *MemoryRepository) Apply(ctx context.Context, entry Entry) (ApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	if err := entry.Validate(); err != nil {
		return ApplyResult{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := operationKey{accountID: entry.AccountID, idempotencyKey: entry.IdempotencyKey}
	if existing, exists := r.operations[key]; exists {
		if !sameOperation(existing.Entry, entry) {
			return ApplyResult{}, ErrIdempotencyConflict
		}
		existing.Created = false
		return existing, nil
	}
	if _, exists := r.entryIDs[entry.ID]; exists {
		return ApplyResult{}, ErrEntryConflict
	}
	if account, exists := r.accounts[entry.AccountID]; exists && account.StudentID != entry.StudentID {
		return ApplyResult{}, ErrEntryConflict
	}

	current := r.balances[entry.AccountID]
	next, ok := current.apply(entry.Delta)
	if !ok {
		return ApplyResult{}, ErrBalanceOverflow
	}
	if !next.valid() {
		return ApplyResult{}, ErrInsufficientBalance
	}

	result := ApplyResult{Entry: entry, Balance: next, Created: true}
	r.entries[entry.AccountID] = append(r.entries[entry.AccountID], entry)
	r.balances[entry.AccountID] = next
	if _, exists := r.accounts[entry.AccountID]; !exists {
		r.accounts[entry.AccountID] = Account{
			ID:        entry.AccountID,
			StudentID: entry.StudentID,
			Unit:      "session_credit",
			CreatedAt: entry.OccurredAt,
		}
	}
	r.operations[key] = result
	r.entryIDs[entry.ID] = struct{}{}
	return result, nil
}

func (r *MemoryRepository) Balance(ctx context.Context, accountID string) (Balance, error) {
	if err := ctx.Err(); err != nil {
		return Balance{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	balance, exists := r.balances[accountID]
	if !exists {
		return Balance{}, ErrAccountNotFound
	}
	return balance, nil
}

func (r *MemoryRepository) Entries(ctx context.Context, accountID string) ([]Entry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	entries, exists := r.entries[accountID]
	if !exists {
		return nil, ErrAccountNotFound
	}
	return append([]Entry{}, entries...), nil
}

func (r *MemoryRepository) EntryPage(
	ctx context.Context,
	accountID string,
	request PageRequest,
) (EntryPage, error) {
	if err := ctx.Err(); err != nil {
		return EntryPage{}, err
	}
	if request.Limit < 1 {
		return EntryPage{}, ErrInvalidCommand
	}

	r.mu.RLock()
	entries, exists := r.entries[accountID]
	r.mu.RUnlock()
	if !exists {
		return EntryPage{}, ErrAccountNotFound
	}

	ordered := append([]Entry{}, entries...)
	sort.Slice(ordered, func(left, right int) bool {
		if ordered[left].OccurredAt.Equal(ordered[right].OccurredAt) {
			return ordered[left].ID > ordered[right].ID
		}
		return ordered[left].OccurredAt.After(ordered[right].OccurredAt)
	})
	if request.Cursor != nil {
		filtered := make([]Entry, 0, len(ordered))
		for _, entry := range ordered {
			isBeforeTime := entry.OccurredAt.Before(request.Cursor.OccurredAt)
			isBeforeID := entry.OccurredAt.Equal(request.Cursor.OccurredAt) &&
				entry.ID < request.Cursor.ID
			if isBeforeTime || isBeforeID {
				filtered = append(filtered, entry)
			}
		}
		ordered = filtered
	}

	page := EntryPage{Items: []Entry{}}
	if len(ordered) <= request.Limit {
		page.Items = ordered
		return page, nil
	}
	page.Items = ordered[:request.Limit]
	last := page.Items[len(page.Items)-1]
	page.NextCursor = &EntryCursor{OccurredAt: last.OccurredAt, ID: last.ID}
	return page, nil
}

func sameOperation(left, right Entry) bool {
	return subtle.ConstantTimeCompare(left.RequestDigest[:], right.RequestDigest[:]) == 1
}
