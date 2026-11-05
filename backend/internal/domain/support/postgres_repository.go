package support

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const supportCaseObjectType = "support_case"

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, errors.New("support repository: pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Create(
	ctx context.Context,
	mutation CreateMutation,
) (Case, error) {
	if err := mutation.Case.Validate(); err != nil {
		return Case{}, err
	}
	if err := mutation.Event.Validate(); err != nil {
		return Case{}, err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Case{}, fmt.Errorf("support repository: beginning create: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO support_cases
			(id, family_id, opened_by, category, severity, status, subject,
			 assigned_to, opened_at, sla_due_at, resolved_at, updated_at, version)
		VALUES
			($1::uuid, $2::uuid, $3::uuid, $4, $5, $6, $7,
			 NULL, $8, $9, NULL, $8, 1)`,
		mutation.Case.ID,
		mutation.Case.FamilyID,
		mutation.Case.OpenedBy,
		mutation.Case.Category,
		mutation.Case.Severity,
		mutation.Case.Status,
		mutation.Case.Subject,
		mutation.Case.OpenedAt,
		mutation.Case.SLADueAt,
	)
	if err != nil {
		return Case{}, mapWriteError("inserting case", err)
	}
	if err := insertEvent(ctx, tx, mutation.Event); err != nil {
		return Case{}, err
	}
	if err := insertAudit(ctx, tx, auditMutation{
		ID:         mutation.Event.ID,
		ActorID:    mutation.Event.ActorID,
		Action:     "support.case.created",
		Case:       mutation.Case,
		After:      &mutation.Case,
		OccurredAt: mutation.Event.OccurredAt,
		RequestID:  mutation.RequestID,
	}); err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, mapWriteError("committing create", err)
	}
	return cloneCase(mutation.Case), nil
}

func (r *PostgresRepository) Summary(ctx context.Context, caseID string) (Case, error) {
	supportCase, err := loadSummary(ctx, r.pool, caseID, false)
	if err != nil {
		return Case{}, err
	}
	return supportCase, nil
}

func (r *PostgresRepository) Case(
	ctx context.Context,
	caseID string,
	includeInternal bool,
) (Case, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return Case{}, fmt.Errorf("support repository: beginning timeline read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	supportCase, err := loadSummary(ctx, tx, caseID, false)
	if err != nil {
		return Case{}, err
	}
	events, err := loadEvents(ctx, tx, caseID, includeInternal)
	if err != nil {
		return Case{}, err
	}
	supportCase.Events = events
	if err := supportCase.Validate(); err != nil {
		return Case{}, fmt.Errorf("support repository: validating timeline: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, fmt.Errorf("support repository: committing timeline read: %w", err)
	}
	return supportCase, nil
}

func (r *PostgresRepository) List(
	ctx context.Context,
	familyID string,
	options ListOptions,
) (Page, error) {
	var cursorTime any
	var cursorID any
	if options.Cursor != nil {
		cursorTime = options.Cursor.OpenedAt
		cursorID = options.Cursor.ID
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id::text, family_id::text, opened_by::text, category, severity,
		       status, subject, assigned_to::text, opened_at, sla_due_at,
		       resolved_at, updated_at, version
		FROM support_cases
		WHERE family_id = $1::uuid
		  AND ($2::timestamptz IS NULL OR (opened_at, id) < ($2, $3::uuid))
		ORDER BY opened_at DESC, id DESC
		LIMIT $4`,
		familyID,
		cursorTime,
		cursorID,
		options.Limit+1,
	)
	if err != nil {
		return Page{}, fmt.Errorf("support repository: listing cases: %w", err)
	}
	defer rows.Close()

	items := make([]Case, 0, options.Limit)
	for rows.Next() {
		supportCase, err := scanCase(rows)
		if err != nil {
			return Page{}, fmt.Errorf("support repository: scanning case list: %w", err)
		}
		items = append(items, supportCase)
	}
	if err := rows.Err(); err != nil {
		return Page{}, fmt.Errorf("support repository: iterating case list: %w", err)
	}

	page := Page{Items: items}
	if len(items) > options.Limit {
		last := items[options.Limit-1]
		page.Items = items[:options.Limit]
		page.NextCursor = &Cursor{OpenedAt: last.OpenedAt, ID: last.ID}
	}
	return page, nil
}

func (r *PostgresRepository) Assign(
	ctx context.Context,
	mutation AssignMutation,
) (Case, error) {
	return r.mutate(ctx, mutation.CaseID, mutation.ExpectedVersion, func(
		ctx context.Context,
		tx pgx.Tx,
		current Case,
	) error {
		result, err := tx.Exec(ctx, `
			UPDATE support_cases
			SET assigned_to = $1::uuid, updated_at = $2, version = version + 1
			WHERE id = $3::uuid AND version = $4`,
			mutation.AssigneeID,
			mutation.OccurredAt,
			mutation.CaseID,
			mutation.ExpectedVersion,
		)
		if err != nil {
			return mapWriteError("updating assignment", err)
		}
		if result.RowsAffected() != 1 {
			return ErrConflict
		}
		event := Event{
			ID:         mutation.EventID,
			CaseID:     mutation.CaseID,
			Type:       EventTypeAssigned,
			Visibility: VisibilityInternal,
			ActorID:    mutation.ActorID,
			Metadata: map[string]string{
				"previousAssigneeId": current.AssignedTo,
				"assigneeId":         mutation.AssigneeID,
			},
			OccurredAt: mutation.OccurredAt,
		}
		if err := insertEvent(ctx, tx, event); err != nil {
			return err
		}
		updated := current
		updated.AssignedTo = mutation.AssigneeID
		updated.UpdatedAt = mutation.OccurredAt
		updated.Version++
		return insertAudit(ctx, tx, auditMutation{
			ID:         mutation.EventID,
			ActorID:    mutation.ActorID,
			Action:     "support.case.assigned",
			Case:       updated,
			Before:     &current,
			After:      &updated,
			OccurredAt: mutation.OccurredAt,
			Reason:     mutation.Reason,
			RequestID:  mutation.RequestID,
		})
	})
}

func (r *PostgresRepository) ChangeStatus(
	ctx context.Context,
	mutation StatusMutation,
) (Case, error) {
	return r.mutate(ctx, mutation.CaseID, mutation.ExpectedVersion, func(
		ctx context.Context,
		tx pgx.Tx,
		current Case,
	) error {
		if current.Status != mutation.FromStatus {
			return ErrConflict
		}
		resolvedAt := resolutionTime(current, mutation.ToStatus, mutation.OccurredAt)
		result, err := tx.Exec(ctx, `
			UPDATE support_cases
			SET status = $1, resolved_at = $2, updated_at = $3, version = version + 1
			WHERE id = $4::uuid AND version = $5 AND status = $6`,
			mutation.ToStatus,
			resolvedAt,
			mutation.OccurredAt,
			mutation.CaseID,
			mutation.ExpectedVersion,
			mutation.FromStatus,
		)
		if err != nil {
			return mapWriteError("updating status", err)
		}
		if result.RowsAffected() != 1 {
			return ErrConflict
		}
		event := Event{
			ID:         mutation.EventID,
			CaseID:     mutation.CaseID,
			Type:       EventTypeStatusChanged,
			Visibility: VisibilityCustomerVisible,
			ActorID:    mutation.ActorID,
			Metadata: map[string]string{
				"fromStatus": string(mutation.FromStatus),
				"toStatus":   string(mutation.ToStatus),
			},
			OccurredAt: mutation.OccurredAt,
		}
		if err := insertEvent(ctx, tx, event); err != nil {
			return err
		}
		updated := current
		updated.Status = mutation.ToStatus
		updated.ResolvedAt = resolvedAt
		updated.UpdatedAt = mutation.OccurredAt
		updated.Version++
		return insertAudit(ctx, tx, auditMutation{
			ID:         mutation.EventID,
			ActorID:    mutation.ActorID,
			Action:     "support.case.status_changed",
			Case:       updated,
			Before:     &current,
			After:      &updated,
			OccurredAt: mutation.OccurredAt,
			Reason:     mutation.Reason,
			RequestID:  mutation.RequestID,
		})
	})
}

func (r *PostgresRepository) AppendEvent(
	ctx context.Context,
	mutation EventMutation,
) (Case, error) {
	if err := mutation.Event.Validate(); err != nil {
		return Case{}, err
	}
	return r.mutate(ctx, mutation.Event.CaseID, mutation.ExpectedVersion, func(
		ctx context.Context,
		tx pgx.Tx,
		current Case,
	) error {
		result, err := tx.Exec(ctx, `
			UPDATE support_cases
			SET updated_at = $1, version = version + 1
			WHERE id = $2::uuid AND version = $3`,
			mutation.Event.OccurredAt,
			mutation.Event.CaseID,
			mutation.ExpectedVersion,
		)
		if err != nil {
			return mapWriteError("updating event version", err)
		}
		if result.RowsAffected() != 1 {
			return ErrConflict
		}
		if err := insertEvent(ctx, tx, mutation.Event); err != nil {
			return err
		}
		updated := current
		updated.UpdatedAt = mutation.Event.OccurredAt
		updated.Version++
		return insertAudit(ctx, tx, auditMutation{
			ID:         mutation.Event.ID,
			ActorID:    mutation.Event.ActorID,
			Action:     "support.case.event_appended",
			Case:       updated,
			Before:     &current,
			After:      &updated,
			OccurredAt: mutation.Event.OccurredAt,
			RequestID:  mutation.RequestID,
			Metadata: map[string]string{
				"eventType":  string(mutation.Event.Type),
				"visibility": string(mutation.Event.Visibility),
			},
		})
	})
}

type mutationFunc func(context.Context, pgx.Tx, Case) error

func (r *PostgresRepository) mutate(
	ctx context.Context,
	caseID string,
	expectedVersion uint64,
	apply mutationFunc,
) (Case, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Case{}, fmt.Errorf("support repository: beginning mutation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	current, err := loadSummary(ctx, tx, caseID, true)
	if err != nil {
		return Case{}, err
	}
	if current.Version != expectedVersion {
		return Case{}, ErrConflict
	}
	if err := apply(ctx, tx, current); err != nil {
		return Case{}, err
	}
	updated, err := loadCaseWithEvents(ctx, tx, caseID, true)
	if err != nil {
		return Case{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Case{}, mapWriteError("committing mutation", err)
	}
	return updated, nil
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type rowScanner interface {
	Scan(dest ...any) error
}

func loadSummary(
	ctx context.Context,
	db queryer,
	caseID string,
	isLocked bool,
) (Case, error) {
	query := `
		SELECT id::text, family_id::text, opened_by::text, category, severity,
		       status, subject, assigned_to::text, opened_at, sla_due_at,
		       resolved_at, updated_at, version
		FROM support_cases
		WHERE id = $1::uuid`
	if isLocked {
		query += " FOR UPDATE"
	}
	supportCase, err := scanCase(db.QueryRow(ctx, query, caseID))
	if errors.Is(err, pgx.ErrNoRows) {
		return Case{}, ErrNotFound
	}
	if err != nil {
		return Case{}, fmt.Errorf("support repository: scanning case: %w", err)
	}
	return supportCase, nil
}

func scanCase(row rowScanner) (Case, error) {
	var supportCase Case
	var assignedTo *string
	var version int64
	err := row.Scan(
		&supportCase.ID,
		&supportCase.FamilyID,
		&supportCase.OpenedBy,
		&supportCase.Category,
		&supportCase.Severity,
		&supportCase.Status,
		&supportCase.Subject,
		&assignedTo,
		&supportCase.OpenedAt,
		&supportCase.SLADueAt,
		&supportCase.ResolvedAt,
		&supportCase.UpdatedAt,
		&version,
	)
	if err != nil {
		return Case{}, err
	}
	if version <= 0 {
		return Case{}, fmt.Errorf("%w: stored version is invalid", ErrInvalidCase)
	}
	if assignedTo != nil {
		supportCase.AssignedTo = *assignedTo
	}
	supportCase.Version = uint64(version)
	supportCase.Events = []Event{}
	if err := supportCase.Validate(); err != nil {
		return Case{}, err
	}
	return supportCase, nil
}

func loadCaseWithEvents(
	ctx context.Context,
	db queryer,
	caseID string,
	includeInternal bool,
) (Case, error) {
	supportCase, err := loadSummary(ctx, db, caseID, false)
	if err != nil {
		return Case{}, err
	}
	events, err := loadEvents(ctx, db, caseID, includeInternal)
	if err != nil {
		return Case{}, err
	}
	supportCase.Events = events
	if err := supportCase.Validate(); err != nil {
		return Case{}, err
	}
	return supportCase, nil
}

func loadEvents(
	ctx context.Context,
	db queryer,
	caseID string,
	includeInternal bool,
) ([]Event, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, support_case_id::text, event_type, visibility,
		       actor_user_id::text, COALESCE(body, ''), metadata, occurred_at
		FROM support_case_events
		WHERE support_case_id = $1::uuid
		  AND ($2 OR visibility = 'customer_visible')
		ORDER BY occurred_at, id`, caseID, includeInternal)
	if err != nil {
		return nil, fmt.Errorf("support repository: selecting timeline: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var event Event
		var metadata []byte
		if err := rows.Scan(
			&event.ID,
			&event.CaseID,
			&event.Type,
			&event.Visibility,
			&event.ActorID,
			&event.Body,
			&metadata,
			&event.OccurredAt,
		); err != nil {
			return nil, fmt.Errorf("support repository: scanning timeline: %w", err)
		}
		event.Metadata = map[string]string{}
		if err := json.Unmarshal(metadata, &event.Metadata); err != nil {
			return nil, fmt.Errorf("support repository: decoding event metadata: %w", err)
		}
		if err := event.Validate(); err != nil {
			return nil, fmt.Errorf("support repository: validating event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("support repository: iterating timeline: %w", err)
	}
	return events, nil
}

func insertEvent(ctx context.Context, tx pgx.Tx, event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("support repository: encoding event metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO support_case_events
			(id, support_case_id, event_type, actor_user_id, body, metadata,
			 occurred_at, visibility)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, NULLIF($5, ''), $6, $7, $8)`,
		event.ID,
		event.CaseID,
		event.Type,
		event.ActorID,
		event.Body,
		metadata,
		event.OccurredAt,
		event.Visibility,
	)
	if err != nil {
		return mapWriteError("inserting event", err)
	}
	return nil
}

type auditMutation struct {
	ID         string
	ActorID    string
	Action     string
	Case       Case
	Before     *Case
	After      *Case
	OccurredAt time.Time
	Reason     string
	RequestID  string
	Metadata   map[string]string
}

func insertAudit(ctx context.Context, tx pgx.Tx, mutation auditMutation) error {
	beforeDigest, err := caseDigest(mutation.Before)
	if err != nil {
		return fmt.Errorf("support repository: encoding before audit digest: %w", err)
	}
	afterDigest, err := caseDigest(mutation.After)
	if err != nil {
		return fmt.Errorf("support repository: encoding after audit digest: %w", err)
	}
	metadata := mutation.Metadata
	if metadata == nil {
		metadata = map[string]string{}
	}
	encodedMetadata, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("support repository: encoding audit metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, actor_id, action, object_type,
			 object_id, scope_id, before_digest, after_digest, reason_text,
			 request_id, metadata)
		VALUES
			($1::uuid, $2, 'user', $3::uuid, $4, $5, $6, $7::uuid,
			 $8, $9, NULLIF($10, ''), NULLIF($11, ''), $12)`,
		mutation.ID,
		mutation.OccurredAt,
		mutation.ActorID,
		mutation.Action,
		supportCaseObjectType,
		mutation.Case.ID,
		mutation.Case.FamilyID,
		beforeDigest,
		afterDigest,
		mutation.Reason,
		mutation.RequestID,
		encodedMetadata,
	)
	if err != nil {
		return mapWriteError("inserting audit event", err)
	}
	return nil
}

func caseDigest(supportCase *Case) ([]byte, error) {
	if supportCase == nil {
		return nil, nil
	}
	copy := cloneCase(*supportCase)
	copy.Events = []Event{}
	encoded, err := json.Marshal(copy)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	return digest[:], nil
}

func resolutionTime(current Case, status Status, occurredAt time.Time) *time.Time {
	if status != StatusResolved && status != StatusClosed {
		return nil
	}
	if current.ResolvedAt != nil {
		value := *current.ResolvedAt
		return &value
	}
	value := occurredAt
	return &value
}

func mapWriteError(action string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505", "40001", "40P01":
			return ErrConflict
		case "23503":
			return ErrNotFound
		}
	}
	return fmt.Errorf("support repository: %s: %w", action, err)
}
