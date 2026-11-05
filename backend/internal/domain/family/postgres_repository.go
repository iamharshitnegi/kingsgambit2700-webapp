package family

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

const (
	relationshipObjectType = "guardian_student_relationship"
	actionGranted          = "family.relationship.granted"
	actionPermissions      = "family.relationship.permissions_replaced"
	actionRevoked          = "family.relationship.revoked"
)

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, fmt.Errorf("family repository: pool is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

var _ Repository = (*PostgresRepository)(nil)

func (r *PostgresRepository) Relationship(
	ctx context.Context,
	relationshipID string,
) (Relationship, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return Relationship{}, fmt.Errorf("family repository: beginning read: %w", err)
	}
	defer tx.Rollback(ctx)

	relationship, err := loadRelationship(ctx, tx, relationshipID, false)
	if err != nil {
		return Relationship{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Relationship{}, fmt.Errorf("family repository: committing read: %w", err)
	}
	return relationship, nil
}

func (r *PostgresRepository) ActiveRelationship(
	ctx context.Context,
	familyID string,
	guardianID string,
	studentID string,
) (Relationship, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return Relationship{}, fmt.Errorf("family repository: beginning active read: %w", err)
	}
	defer tx.Rollback(ctx)

	var relationshipID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM guardian_student_relationships
		WHERE family_id = $1::uuid
		  AND guardian_user_id = $2::uuid
		  AND student_profile_id = $3::uuid
		  AND status = 'active'`,
		familyID,
		guardianID,
		studentID,
	).Scan(&relationshipID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Relationship{}, ErrNotFound
	}
	if err != nil {
		return Relationship{}, fmt.Errorf("family repository: selecting active relationship: %w", err)
	}

	relationship, err := loadRelationship(ctx, tx, relationshipID, false)
	if err != nil {
		return Relationship{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Relationship{}, fmt.Errorf("family repository: committing active read: %w", err)
	}
	return relationship, nil
}

func (r *PostgresRepository) Save(
	ctx context.Context,
	relationship Relationship,
	event Event,
	expectedRevision uint64,
) error {
	if err := relationship.validate(); err != nil {
		return err
	}
	if err := validateEvent(relationship, event); err != nil {
		return err
	}

	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return fmt.Errorf("family repository: beginning save: %w", err)
	}
	defer tx.Rollback(ctx)

	if expectedRevision == 0 {
		err = insertRelationship(ctx, tx, relationship)
	} else {
		err = updateRelationship(ctx, tx, relationship, expectedRevision)
	}
	if err != nil {
		return err
	}
	if err := insertRelationshipEvent(ctx, tx, relationship, event); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		if isPostgresConflict(err) {
			return ErrConflict
		}
		return fmt.Errorf("family repository: committing save: %w", err)
	}
	return nil
}

func (r *PostgresRepository) Events(
	ctx context.Context,
	relationshipID string,
) ([]Event, error) {
	events, err := loadEvents(ctx, r.pool, relationshipID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, ErrNotFound
	}
	return events, nil
}

type relationshipRow struct {
	id          string
	familyID    string
	guardianID  string
	studentID   string
	permissions []string
	status      string
	createdAt   time.Time
	revokedAt   *time.Time
}

type queryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func loadRelationship(
	ctx context.Context,
	db queryer,
	relationshipID string,
	isLocked bool,
) (Relationship, error) {
	query := `
		SELECT id::text, family_id::text, guardian_user_id::text,
		       student_profile_id::text, permissions, status, created_at, revoked_at
		FROM guardian_student_relationships
		WHERE id = $1::uuid`
	if isLocked {
		query += " FOR UPDATE"
	}

	var row relationshipRow
	err := db.QueryRow(ctx, query, relationshipID).Scan(
		&row.id,
		&row.familyID,
		&row.guardianID,
		&row.studentID,
		&row.permissions,
		&row.status,
		&row.createdAt,
		&row.revokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Relationship{}, ErrNotFound
	}
	if err != nil {
		return Relationship{}, fmt.Errorf("family repository: scanning relationship: %w", err)
	}

	events, err := loadEvents(ctx, db, relationshipID)
	if err != nil {
		return Relationship{}, err
	}
	return restoreRelationship(row, events)
}

func restoreRelationship(row relationshipRow, events []Event) (Relationship, error) {
	if len(events) == 0 || events[0].Kind != EventKindGranted {
		return Relationship{}, fmt.Errorf(
			"family repository: restoring relationship: %w: grant audit event is missing",
			ErrInvalidRelationship,
		)
	}
	for index, event := range events {
		expectedRevision := uint64(index + 1)
		if event.Revision != expectedRevision {
			return Relationship{}, fmt.Errorf(
				"family repository: restoring relationship: %w: non-sequential audit revision",
				ErrInvalidRelationship,
			)
		}
	}

	permissions := make([]Permission, 0, len(row.permissions))
	for _, permission := range row.permissions {
		permissions = append(permissions, Permission(permission))
	}
	record := RelationshipRecord{
		ID:          row.id,
		FamilyID:    row.familyID,
		GuardianID:  row.guardianID,
		StudentID:   row.studentID,
		Permissions: permissions,
		Status:      RelationshipStatus(row.status),
		GrantedBy:   events[0].ActorID,
		GrantedAt:   events[0].OccurredAt,
		Revision:    uint64(len(events)),
	}
	if record.Status == RelationshipStatusRevoked {
		last := events[len(events)-1]
		if last.Kind != EventKindRevoked || row.revokedAt == nil {
			return Relationship{}, fmt.Errorf(
				"family repository: restoring relationship: %w: revoke audit event is missing",
				ErrInvalidRelationship,
			)
		}
		record.RevokedBy = last.ActorID
		record.RevokedAt = row.revokedAt
		record.RevokeReason = last.Reason
	}

	relationship, err := RestoreRelationship(record)
	if err != nil {
		return Relationship{}, fmt.Errorf("family repository: restoring relationship: %w", err)
	}
	return relationship, nil
}

type relationshipEventMetadata struct {
	Revision            uint64       `json:"revision"`
	PreviousPermissions []Permission `json:"previousPermissions"`
	Permissions         []Permission `json:"permissions"`
}

func loadEvents(
	ctx context.Context,
	db queryer,
	relationshipID string,
) ([]Event, error) {
	rows, err := db.Query(ctx, `
		SELECT id::text, action, COALESCE(actor_id::text, ''), occurred_at,
		       COALESCE(reason_text, ''), metadata
		FROM audit_events
		WHERE object_type = $1 AND object_id = $2
		  AND action IN ($3, $4, $5)
		ORDER BY (metadata->>'revision')::bigint`,
		relationshipObjectType,
		relationshipID,
		actionGranted,
		actionPermissions,
		actionRevoked,
	)
	if err != nil {
		return nil, fmt.Errorf("family repository: selecting relationship events: %w", err)
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var event Event
		var action string
		var rawMetadata []byte
		if err := rows.Scan(
			&event.ID,
			&action,
			&event.ActorID,
			&event.OccurredAt,
			&event.Reason,
			&rawMetadata,
		); err != nil {
			return nil, fmt.Errorf("family repository: scanning relationship event: %w", err)
		}

		var metadata relationshipEventMetadata
		if err := json.Unmarshal(rawMetadata, &metadata); err != nil {
			return nil, fmt.Errorf("family repository: decoding relationship event: %w", err)
		}
		event.RelationshipID = relationshipID
		event.Revision = metadata.Revision
		event.PreviousPermissions = metadata.PreviousPermissions
		event.Permissions = metadata.Permissions
		event.Kind, err = eventKind(action)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("family repository: iterating relationship events: %w", err)
	}
	return events, nil
}

func eventKind(action string) (EventKind, error) {
	switch action {
	case actionGranted:
		return EventKindGranted, nil
	case actionPermissions:
		return EventKindPermissionsReplaced, nil
	case actionRevoked:
		return EventKindRevoked, nil
	default:
		return EventKindUnknown, fmt.Errorf(
			"family repository: unsupported relationship action %q",
			action,
		)
	}
}

func insertRelationship(
	ctx context.Context,
	tx pgx.Tx,
	relationship Relationship,
) error {
	if relationship.Revision() != 1 || !relationship.IsActive() {
		return ErrConflict
	}
	record := relationship.Record()
	permissions := permissionStrings(record.Permissions)
	_, err := tx.Exec(ctx, `
		INSERT INTO guardian_student_relationships
			(id, family_id, guardian_user_id, student_profile_id, permissions,
			 status, created_at, revoked_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid, $5, 'active', $6, NULL)`,
		record.ID,
		record.FamilyID,
		record.GuardianID,
		record.StudentID,
		permissions,
		record.GrantedAt,
	)
	if isPostgresConflict(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("family repository: inserting relationship: %w", err)
	}
	return nil
}

func updateRelationship(
	ctx context.Context,
	tx pgx.Tx,
	relationship Relationship,
	expectedRevision uint64,
) error {
	current, err := loadRelationship(ctx, tx, relationship.ID(), true)
	if err != nil {
		return err
	}
	if current.Revision() != expectedRevision || relationship.Revision() != expectedRevision+1 {
		return ErrConflict
	}
	if !sameRelationshipIdentity(current, relationship) {
		return fmt.Errorf("family repository: relationship identity cannot change: %w", ErrConflict)
	}

	record := relationship.Record()
	_, err = tx.Exec(ctx, `
		UPDATE guardian_student_relationships
		SET permissions = $2, status = $3, revoked_at = $4
		WHERE id = $1::uuid`,
		record.ID,
		permissionStrings(record.Permissions),
		record.Status,
		record.RevokedAt,
	)
	if err != nil {
		return fmt.Errorf("family repository: updating relationship: %w", err)
	}
	return nil
}

func insertRelationshipEvent(
	ctx context.Context,
	tx pgx.Tx,
	relationship Relationship,
	event Event,
) error {
	metadata, err := json.Marshal(relationshipEventMetadata{
		Revision:            event.Revision,
		PreviousPermissions: event.PreviousPermissions,
		Permissions:         event.Permissions,
	})
	if err != nil {
		return fmt.Errorf("family repository: encoding relationship event: %w", err)
	}
	action, err := eventAction(event.Kind)
	if err != nil {
		return err
	}
	recordJSON, err := json.Marshal(relationship.Record())
	if err != nil {
		return fmt.Errorf("family repository: encoding relationship digest: %w", err)
	}
	afterDigest := sha256.Sum256(recordJSON)

	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, actor_id, action, object_type,
			 object_id, scope_id, after_digest, reason_text, metadata)
		VALUES ($1::uuid, $2, 'user', $3::uuid, $4, $5, $6, $7::uuid,
		        $8, NULLIF($9, ''), $10)`,
		event.ID,
		event.OccurredAt,
		event.ActorID,
		action,
		relationshipObjectType,
		relationship.ID(),
		relationship.FamilyID(),
		afterDigest[:],
		event.Reason,
		metadata,
	)
	if isPostgresConflict(err) {
		return ErrConflict
	}
	if err != nil {
		return fmt.Errorf("family repository: inserting relationship event: %w", err)
	}
	return nil
}

func eventAction(kind EventKind) (string, error) {
	switch kind {
	case EventKindGranted:
		return actionGranted, nil
	case EventKindPermissionsReplaced:
		return actionPermissions, nil
	case EventKindRevoked:
		return actionRevoked, nil
	default:
		return "", fmt.Errorf("family repository: unsupported event kind %q", kind)
	}
}

func sameRelationshipIdentity(left, right Relationship) bool {
	leftRecord := left.Record()
	rightRecord := right.Record()
	return leftRecord.ID == rightRecord.ID &&
		leftRecord.FamilyID == rightRecord.FamilyID &&
		leftRecord.GuardianID == rightRecord.GuardianID &&
		leftRecord.StudentID == rightRecord.StudentID &&
		leftRecord.GrantedBy == rightRecord.GrantedBy &&
		leftRecord.GrantedAt.Equal(rightRecord.GrantedAt)
}

func permissionStrings(permissions []Permission) []string {
	values := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		values = append(values, string(permission))
	}
	return values
}

func isPostgresConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "23505" || pgErr.Code == "40001"
}
