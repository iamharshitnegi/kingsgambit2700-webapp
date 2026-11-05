package entitlement

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

const entitlementEntryColumns = `
	id::text, entry_type, available_delta, reserved_delta, consumed_delta,
	idempotency_key, request_digest, reference_type, reference_id, reason,
	actor_user_id::text, occurred_at`

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("entitlement repository: pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Account(
	ctx context.Context,
	accountID string,
) (Account, error) {
	return loadAccount(ctx, r.pool, accountID, false)
}

func (r *PostgresRepository) Apply(
	ctx context.Context,
	entry Entry,
) (ApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	if err := entry.Validate(); err != nil {
		return ApplyResult{}, err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ApplyResult{}, fmt.Errorf("entitlement repository: beginning apply: %w", err)
	}
	defer tx.Rollback(ctx)

	account, err := loadAccount(ctx, tx, entry.AccountID, true)
	if err != nil {
		return ApplyResult{}, err
	}
	if account.StudentID != entry.StudentID {
		return ApplyResult{}, ErrEntryConflict
	}

	existing, found, err := loadIdempotentEntry(
		ctx,
		tx,
		account,
		entry.IdempotencyKey,
	)
	if err != nil {
		return ApplyResult{}, err
	}
	if found {
		if subtle.ConstantTimeCompare(
			existing.Entry.RequestDigest[:],
			entry.RequestDigest[:],
		) != 1 {
			return ApplyResult{}, ErrIdempotencyConflict
		}
		existing.Created = false
		if err := tx.Commit(ctx); err != nil {
			return ApplyResult{}, fmt.Errorf("entitlement repository: committing replay: %w", err)
		}
		return existing, nil
	}

	current, err := loadBalance(ctx, tx, entry.AccountID)
	if err != nil {
		return ApplyResult{}, err
	}
	next, ok := current.apply(entry.Delta)
	if !ok {
		return ApplyResult{}, ErrBalanceOverflow
	}
	if !next.valid() {
		return ApplyResult{}, ErrInsufficientBalance
	}

	if err := insertEntry(ctx, tx, entry, next); err != nil {
		return ApplyResult{}, err
	}
	if err := insertAuditEvent(ctx, tx, account, entry, next); err != nil {
		return ApplyResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		if isEntitlementConflict(err) {
			return ApplyResult{}, ErrEntryConflict
		}
		return ApplyResult{}, fmt.Errorf("entitlement repository: committing apply: %w", err)
	}
	return ApplyResult{Entry: entry, Balance: next, Created: true}, nil
}

func (r *PostgresRepository) Balance(
	ctx context.Context,
	accountID string,
) (Balance, error) {
	if _, err := r.Account(ctx, accountID); err != nil {
		return Balance{}, err
	}
	return loadBalance(ctx, r.pool, accountID)
}

func (r *PostgresRepository) Entries(
	ctx context.Context,
	accountID string,
) ([]Entry, error) {
	account, err := r.Account(ctx, accountID)
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+entitlementEntryColumns+`
		FROM entitlement_entries
		WHERE account_id = $1::uuid
		ORDER BY occurred_at, id`, accountID)
	if err != nil {
		return nil, fmt.Errorf("entitlement repository: querying entries: %w", err)
	}
	defer rows.Close()
	entries, err := collectEntries(rows, account)
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *PostgresRepository) EntryPage(
	ctx context.Context,
	accountID string,
	request PageRequest,
) (EntryPage, error) {
	if request.Limit < 1 || request.Limit > 100 {
		return EntryPage{}, fmt.Errorf("%w: page limit must be between 1 and 100", ErrInvalidCommand)
	}
	account, err := r.Account(ctx, accountID)
	if err != nil {
		return EntryPage{}, err
	}

	var rows pgx.Rows
	if request.Cursor == nil {
		rows, err = r.pool.Query(ctx, `
			SELECT `+entitlementEntryColumns+`
			FROM entitlement_entries
			WHERE account_id = $1::uuid
			ORDER BY occurred_at DESC, id DESC
			LIMIT $2`, accountID, request.Limit+1)
	} else {
		rows, err = r.pool.Query(ctx, `
			SELECT `+entitlementEntryColumns+`
			FROM entitlement_entries
			WHERE account_id = $1::uuid
			  AND (occurred_at, id) < ($2, $3::uuid)
			ORDER BY occurred_at DESC, id DESC
			LIMIT $4`,
			accountID,
			request.Cursor.OccurredAt,
			request.Cursor.ID,
			request.Limit+1,
		)
	}
	if err != nil {
		return EntryPage{}, fmt.Errorf("entitlement repository: querying entry page: %w", err)
	}
	defer rows.Close()
	entries, err := collectEntries(rows, account)
	if err != nil {
		return EntryPage{}, err
	}

	page := EntryPage{Items: entries}
	if len(entries) <= request.Limit {
		return page, nil
	}
	page.Items = entries[:request.Limit]
	last := page.Items[len(page.Items)-1]
	page.NextCursor = &EntryCursor{OccurredAt: last.OccurredAt, ID: last.ID}
	return page, nil
}

type accountQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func loadAccount(
	ctx context.Context,
	database accountQuerier,
	accountID string,
	isLocked bool,
) (Account, error) {
	query := `
		SELECT account.id::text, account.enrolment_id::text,
		       enrolment.student_profile_id::text, account.unit, account.created_at
		FROM entitlement_accounts AS account
		JOIN enrolments AS enrolment ON enrolment.id = account.enrolment_id
		WHERE account.id = $1::uuid`
	if isLocked {
		query += " FOR UPDATE OF account"
	}
	var account Account
	err := database.QueryRow(ctx, query, accountID).Scan(
		&account.ID,
		&account.EnrolmentID,
		&account.StudentID,
		&account.Unit,
		&account.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, ErrAccountNotFound
	}
	if err != nil {
		return Account{}, fmt.Errorf("entitlement repository: loading account: %w", err)
	}
	return account, nil
}

func loadBalance(
	ctx context.Context,
	database accountQuerier,
	accountID string,
) (Balance, error) {
	var balance Balance
	err := database.QueryRow(ctx, `
		SELECT COALESCE(sum(available_delta), 0),
		       COALESCE(sum(reserved_delta), 0),
		       COALESCE(sum(consumed_delta), 0)
		FROM entitlement_entries
		WHERE account_id = $1::uuid`, accountID).Scan(
		&balance.Available,
		&balance.Reserved,
		&balance.Consumed,
	)
	if err != nil {
		return Balance{}, fmt.Errorf("entitlement repository: loading balance: %w", err)
	}
	return balance, nil
}

func loadIdempotentEntry(
	ctx context.Context,
	database accountQuerier,
	account Account,
	idempotencyKey string,
) (ApplyResult, bool, error) {
	row := database.QueryRow(ctx, `
		SELECT `+entitlementEntryColumns+`,
		       available_balance, reserved_balance, consumed_balance
		FROM entitlement_entries
		WHERE account_id = $1::uuid AND idempotency_key = $2`,
		account.ID,
		idempotencyKey,
	)
	entry, balance, err := scanAppliedEntry(row, account)
	if errors.Is(err, pgx.ErrNoRows) {
		return ApplyResult{}, false, nil
	}
	if err != nil {
		return ApplyResult{}, false, err
	}
	return ApplyResult{Entry: entry, Balance: balance, Created: false}, true, nil
}

func insertEntry(
	ctx context.Context,
	tx pgx.Tx,
	entry Entry,
	balance Balance,
) error {
	actorID, classSessionID, err := entryForeignKeys(entry)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO entitlement_entries
			(id, account_id, class_session_id, entry_type,
			 available_delta, reserved_delta, consumed_delta,
			 available_balance, reserved_balance, consumed_balance,
			 reason, idempotency_key, request_digest, reference_type,
			 reference_id, actor_user_id, occurred_at)
		VALUES
			($1::uuid, $2::uuid, $3::uuid, $4,
			 $5, $6, $7, $8, $9, $10,
			 $11, $12, $13, $14, $15, $16::uuid, $17)`,
		entry.ID,
		entry.AccountID,
		classSessionID,
		entry.Kind,
		entry.Delta.Available,
		entry.Delta.Reserved,
		entry.Delta.Consumed,
		balance.Available,
		balance.Reserved,
		balance.Consumed,
		entry.Reason,
		entry.IdempotencyKey,
		entry.RequestDigest[:],
		entry.ReferenceType,
		entry.ReferenceID,
		actorID,
		entry.OccurredAt,
	)
	if isEntitlementConflict(err) {
		return ErrEntryConflict
	}
	if err != nil {
		return fmt.Errorf("entitlement repository: inserting entry: %w", err)
	}
	return nil
}

func insertAuditEvent(
	ctx context.Context,
	tx pgx.Tx,
	account Account,
	entry Entry,
	balance Balance,
) error {
	actorID, _, err := entryForeignKeys(entry)
	if err != nil {
		return err
	}
	actorType := "system"
	if actorID != nil {
		actorType = "user"
	}
	metadata, err := json.Marshal(struct {
		EntryID       string    `json:"entryId"`
		Kind          EntryKind `json:"kind"`
		Delta         Delta     `json:"delta"`
		Balance       Balance   `json:"balance"`
		ReferenceType string    `json:"referenceType"`
		ReferenceID   string    `json:"referenceId"`
	}{
		EntryID:       entry.ID,
		Kind:          entry.Kind,
		Delta:         entry.Delta,
		Balance:       balance,
		ReferenceType: entry.ReferenceType,
		ReferenceID:   entry.ReferenceID,
	})
	if err != nil {
		return fmt.Errorf("entitlement repository: encoding audit metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, actor_id, action, object_type,
			 object_id, scope_id, after_digest, reason_text, idempotency_key, metadata)
		VALUES
			($1::uuid, $2, $3, $4::uuid, $5, 'entitlement_account',
			 $6, $7::uuid, $8, $9, $10, $11)`,
		entry.ID,
		entry.OccurredAt,
		actorType,
		actorID,
		"entitlement."+string(entry.Kind),
		account.ID,
		account.StudentID,
		entry.RequestDigest[:],
		entry.Reason,
		entry.IdempotencyKey,
		metadata,
	)
	if isEntitlementConflict(err) {
		return ErrEntryConflict
	}
	if err != nil {
		return fmt.Errorf("entitlement repository: inserting audit event: %w", err)
	}
	return nil
}

func entryForeignKeys(entry Entry) (actorID any, classSessionID any, err error) {
	switch {
	case platformid.ValidReference(entry.ActorID):
		actorID = entry.ActorID
	case entry.ActorID != "system":
		return nil, nil, fmt.Errorf("%w: actor must be a UUIDv4, UUIDv7 or system", ErrInvalidEntry)
	}
	if entry.ReferenceType == "class_session" {
		if !platformid.ValidReference(entry.ReferenceID) {
			return nil, nil, fmt.Errorf("%w: class session reference must be a UUIDv4 or UUIDv7", ErrInvalidEntry)
		}
		classSessionID = entry.ReferenceID
	}
	return actorID, classSessionID, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntry(scanner rowScanner, account Account) (Entry, error) {
	var entry Entry
	var digest []byte
	var actorID *string
	entry.AccountID = account.ID
	entry.StudentID = account.StudentID
	err := scanner.Scan(
		&entry.ID,
		&entry.Kind,
		&entry.Delta.Available,
		&entry.Delta.Reserved,
		&entry.Delta.Consumed,
		&entry.IdempotencyKey,
		&digest,
		&entry.ReferenceType,
		&entry.ReferenceID,
		&entry.Reason,
		&actorID,
		&entry.OccurredAt,
	)
	if err != nil {
		return Entry{}, err
	}
	if len(digest) != len(entry.RequestDigest) {
		return Entry{}, fmt.Errorf("entitlement repository: invalid request digest length %d", len(digest))
	}
	copy(entry.RequestDigest[:], digest)
	entry.ActorID = "system"
	if actorID != nil {
		entry.ActorID = *actorID
	}
	return entry, nil
}

func scanAppliedEntry(
	scanner rowScanner,
	account Account,
) (Entry, Balance, error) {
	var entry Entry
	var digest []byte
	var actorID *string
	var balance Balance
	entry.AccountID = account.ID
	entry.StudentID = account.StudentID
	err := scanner.Scan(
		&entry.ID,
		&entry.Kind,
		&entry.Delta.Available,
		&entry.Delta.Reserved,
		&entry.Delta.Consumed,
		&entry.IdempotencyKey,
		&digest,
		&entry.ReferenceType,
		&entry.ReferenceID,
		&entry.Reason,
		&actorID,
		&entry.OccurredAt,
		&balance.Available,
		&balance.Reserved,
		&balance.Consumed,
	)
	if err != nil {
		return Entry{}, Balance{}, err
	}
	if len(digest) != len(entry.RequestDigest) {
		return Entry{}, Balance{}, fmt.Errorf(
			"entitlement repository: invalid request digest length %d",
			len(digest),
		)
	}
	copy(entry.RequestDigest[:], digest)
	entry.ActorID = "system"
	if actorID != nil {
		entry.ActorID = *actorID
	}
	return entry, balance, nil
}

func collectEntries(rows pgx.Rows, account Account) ([]Entry, error) {
	entries := []Entry{}
	for rows.Next() {
		entry, err := scanEntry(rows, account)
		if err != nil {
			return nil, fmt.Errorf("entitlement repository: scanning entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("entitlement repository: iterating entries: %w", err)
	}
	return entries, nil
}

func isEntitlementConflict(err error) bool {
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		return false
	}
	return postgresError.Code == "23505" || postgresError.Code == "40001"
}
