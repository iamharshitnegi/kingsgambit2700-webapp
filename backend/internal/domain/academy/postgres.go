package academy

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
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type PostgresRepository struct{ pool *pgxpool.Pool }

func NewPostgresRepository(pool *pgxpool.Pool) (*PostgresRepository, error) {
	if pool == nil {
		return nil, invalid("database is required")
	}
	return &PostgresRepository{pool: pool}, nil
}

var _ Repository = (*PostgresRepository)(nil)

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r *PostgresRepository) within(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("starting academy transaction: %w", err)
	}
	defer tx.Rollback(ctx)
	// Serialize academy changes before reading conflicts/capacity. Credit grants
	// additionally serialize on entitlement_accounts in the ledger repository.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(270012)"); err != nil {
		return err
	}
	if err = fn(tx); err != nil {
		return dbError(err)
	}
	return dbError(tx.Commit(ctx))
}
func dbError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if errors.Is(err, entitlement.ErrInsufficientBalance) {
		return ErrCredits
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505", "40001", "40P01":
			return ErrConflict
		case "23503", "23514", "22P02":
			return ErrInvalid
		}
	}
	return err
}

func permission(ctx context.Context, q querier, actor, code, student string) (bool, bool, error) {
	var allow, deny bool
	err := q.QueryRow(ctx, `SELECT
 EXISTS(SELECT 1 FROM users u WHERE u.id=$1::uuid AND u.status='active') AND (
 EXISTS(SELECT 1 FROM role_grants g JOIN role_permission_defaults d ON d.role=g.role WHERE g.user_id=$1::uuid AND g.revoked_at IS NULL AND d.permission_code=$2)
 OR EXISTS(SELECT 1 FROM permission_grants g WHERE g.user_id=$1::uuid AND g.permission_code=$2 AND g.effect='allow' AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>now()) AND (g.scope_type='global' OR (g.scope_type='student' AND g.scope_id=NULLIF($3,'')::uuid) OR (g.scope_type='family' AND g.scope_id IN(SELECT family_id FROM guardian_student_relationships WHERE student_profile_id=NULLIF($3,'')::uuid AND status='active'))))),
 EXISTS(SELECT 1 FROM permission_grants g WHERE g.user_id=$1::uuid AND g.permission_code=$2 AND g.effect='deny' AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>now()) AND (g.scope_type='global' OR (g.scope_type='student' AND g.scope_id=NULLIF($3,'')::uuid) OR (g.scope_type='family' AND g.scope_id IN(SELECT family_id FROM guardian_student_relationships WHERE student_profile_id=NULLIF($3,'')::uuid AND status='active'))))`, actor, code, student).Scan(&allow, &deny)
	return allow, deny, err
}
func role(ctx context.Context, q querier, actor, name string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u JOIN role_grants g ON g.user_id=u.id WHERE u.id=$1::uuid AND u.status='active' AND g.role=$2 AND g.revoked_at IS NULL)`, actor, name).Scan(&ok)
	return ok, err
}
func requirePermission(ctx context.Context, q querier, actor, code string) error {
	allowed, denied, err := permission(ctx, q, actor, code, "")
	if err != nil {
		return err
	}
	if !allowed || denied {
		return ErrDenied
	}
	return nil
}
func studentPermission(ctx context.Context, q querier, actor, student, code, guardianPermission string, coachFallback bool) error {
	allow, deny, err := permission(ctx, q, actor, code, student)
	if err != nil {
		return err
	}
	if deny {
		return ErrDenied
	}
	if allow {
		return nil
	}
	var related bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users u WHERE u.id=$1::uuid AND u.status='active') AND (
 ($4<>'' AND EXISTS(SELECT 1 FROM guardian_student_relationships r WHERE r.guardian_user_id=$1::uuid AND r.student_profile_id=$2::uuid AND r.status='active' AND $4=ANY(r.permissions)))
 OR ($3 AND EXISTS(SELECT 1 FROM role_grants g WHERE g.user_id=$1::uuid AND g.role='coach' AND g.revoked_at IS NULL) AND EXISTS(SELECT 1 FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=$2::uuid AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.coach_user_id=$1::uuid AND b.status='active'))
 )`, actor, student, coachFallback, guardianPermission).Scan(&related)
	if err != nil {
		return err
	}
	if !related {
		return ErrDenied
	}
	return nil
}
func batchPermission(ctx context.Context, q querier, actor, batch, code string) error {
	allow, deny, err := permission(ctx, q, actor, code, "")
	if err != nil {
		return err
	}
	if deny {
		return ErrDenied
	}
	if allow {
		return nil
	}
	var ok bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM batches b JOIN users u ON u.id=b.coach_user_id JOIN role_grants g ON g.user_id=u.id WHERE b.id=$1::uuid AND b.coach_user_id=$2::uuid AND b.status='active' AND u.status='active' AND g.role='coach' AND g.revoked_at IS NULL)`, batch, actor).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrDenied
	}
	return nil
}
func audit(ctx context.Context, tx pgx.Tx, actor, action, object string, now time.Time, metadata any) error {
	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(id,occurred_at,actor_type,actor_id,action,object_type,object_id,metadata) VALUES($1::uuid,$2,'user',$3::uuid,$4,'academy',$5,$6)`, platformid.New(), now, actor, action, object, body)
	return err
}
func replay(ctx context.Context, tx pgx.Tx, actor, operation, key string, input, output any) (bool, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(raw)
	var oldDigest, body []byte
	err = tx.QueryRow(ctx, `SELECT request_digest,response FROM academy_request_results WHERE actor_id=$1::uuid AND operation=$2 AND idempotency_key=$3`, actor, operation, key).Scan(&oldDigest, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if string(oldDigest) != string(digest[:]) {
		return false, ErrConflict
	}
	return true, json.Unmarshal(body, output)
}
func remember(ctx context.Context, tx pgx.Tx, actor, operation, key string, input, output any, now time.Time) error {
	raw, err := json.Marshal(input)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(raw)
	body, err := json.Marshal(output)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO academy_request_results(actor_id,operation,idempotency_key,request_digest,response,created_at) VALUES($1::uuid,$2,$3,$4,$5,$6)`, actor, operation, key, digest[:], body, now)
	return err
}

// consentCurrent validates the two published policy versions as well as the
// organization identity; a stale acceptance cannot silently authorize teaching.
func consentCurrent(ctx context.Context, q querier, student string) (bool, error) {
	var ok bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_settings WHERE legal_name IS NOT NULL AND legal_name<>'' AND email IS NOT NULL) AND
 (SELECT count(*)=2 FROM (SELECT DISTINCT ON(p.slug) v.id FROM policy_versions v JOIN policies p ON p.id=v.policy_id WHERE p.slug IN('privacy','child-safety') AND v.status='published' AND v.effective_at<=now() ORDER BY p.slug,v.effective_at DESC,v.created_at DESC) current_policy WHERE EXISTS(SELECT 1 FROM consent_records c JOIN student_profiles s ON s.user_id=c.subject_user_id WHERE s.id=$1::uuid AND c.policy_version_id=current_policy.id AND c.withdrawn_at IS NULL))`, student).Scan(&ok)
	return ok, err
}
