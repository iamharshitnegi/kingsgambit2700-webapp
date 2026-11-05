package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) CreateMagicLink(ctx context.Context, link model.MagicLink) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO auth_magic_links
			(id, email, token_hash, redirect_path, expires_at, created_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)`,
		link.ID, link.Email, link.TokenHash, link.RedirectPath, link.ExpiresAt, link.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("auth repository: inserting magic link: %w", err)
	}
	return nil
}

func (p *Postgres) ExchangeMagicLink(
	ctx context.Context,
	hash []byte,
	session model.Session,
	now time.Time,
) (model.MagicLink, model.User, model.Session, error) {
	if !validNewSession(session) {
		return model.MagicLink{}, model.User{}, model.Session{}, ErrInvalidSession
	}

	var link model.MagicLink
	var user model.User
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE auth_magic_links
			SET consumed_at = $2
			WHERE token_hash = $1
			  AND consumed_at IS NULL
			  AND expires_at > $2
			RETURNING id::text, email, token_hash, redirect_path, expires_at, consumed_at, created_at`, hash, now,
		).Scan(
			&link.ID,
			&link.Email,
			&link.TokenHash,
			&link.RedirectPath,
			&link.ExpiresAt,
			&link.ConsumedAt,
			&link.CreatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("auth repository: consuming magic link: %w", err)
		}

		user, err = findOrCreateGuardianByEmail(ctx, tx, link.Email, now)
		if err != nil {
			return err
		}
		if user.Status != "active" {
			return ErrNotFound
		}

		session.UserID = user.ID
		if err := insertSession(ctx, tx, session, ""); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return model.MagicLink{}, model.User{}, model.Session{}, fmt.Errorf("auth repository: exchanging magic link: %w", err)
	}
	return link, user, session, nil
}

func (p *Postgres) SessionByTokenHash(ctx context.Context, hash []byte) (model.Session, model.User, error) {
	var session model.Session
	var user model.User
	err := p.pool.QueryRow(ctx, `
		SELECT s.id::text, s.user_id::text, s.token_hash, s.csrf_hash, s.csrf_expires_at,
		       s.created_at, s.last_seen_at, s.idle_expires_at, s.absolute_expires_at, s.revoked_at,
		       s.user_agent_hash, s.ip_address_hash, s.mfa_verified_at, COALESCE(s.mfa_method, ''), COALESCE(s.device_id::text, ''),
		       u.id::text, COALESCE(u.email, ''), u.display_name, u.status, u.created_at
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1`, hash,
	).Scan(
		&session.ID, &session.UserID, &session.TokenHash, &session.CSRFHash, &session.CSRFExpiresAt, &session.CreatedAt,
		&session.LastSeenAt, &session.IdleExpiresAt, &session.AbsoluteExpiresAt, &session.RevokedAt,
		&session.UserAgentHash, &session.IPAddressHash,
		&session.MFAVerifiedAt, &session.MFAMethod, &session.DeviceID,
		&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Session{}, model.User{}, ErrNotFound
	}
	if err != nil {
		return model.Session{}, model.User{}, fmt.Errorf("auth repository: selecting session: %w", err)
	}
	roles, err := rolesForUser(ctx, p.pool, user.ID)
	if err != nil {
		return model.Session{}, model.User{}, err
	}
	user.Roles = roles
	return session, user, nil
}

func (p *Postgres) ListActiveSessions(ctx context.Context, userID string, now time.Time) ([]model.Session, error) {
	rows, err := p.pool.Query(ctx, `
 SELECT id::text,user_id::text,created_at,last_seen_at,idle_expires_at,absolute_expires_at,user_agent_hash,ip_address_hash,COALESCE(device_id::text,'')
 FROM (
  SELECT DISTINCT ON (COALESCE(s.device_id,s.id)) s.* FROM auth_sessions s LEFT JOIN auth_devices d ON d.id=s.device_id
  WHERE s.user_id=$1::uuid AND (
   (s.device_id IS NULL AND s.revoked_at IS NULL AND s.idle_expires_at>$2 AND s.absolute_expires_at>$2)
   OR (d.id IS NOT NULL AND d.revoked_at IS NULL))
  ORDER BY COALESCE(s.device_id,s.id),s.last_seen_at DESC,s.id DESC
 ) devices ORDER BY last_seen_at DESC,id DESC`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("auth repository: listing active sessions: %w", err)
	}
	defer rows.Close()

	sessions := make([]model.Session, 0)
	for rows.Next() {
		var session model.Session
		if err := rows.Scan(
			&session.ID,
			&session.UserID,
			&session.CreatedAt,
			&session.LastSeenAt,
			&session.IdleExpiresAt,
			&session.AbsoluteExpiresAt,
			&session.UserAgentHash,
			&session.IPAddressHash,
			&session.DeviceID,
		); err != nil {
			return nil, fmt.Errorf("auth repository: scanning active session: %w", err)
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth repository: iterating active sessions: %w", err)
	}
	return sessions, nil
}

func (p *Postgres) TouchSession(ctx context.Context, id string, seenAt, idleExpiresAt time.Time) error {
	result, err := p.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET last_seen_at = $2, idle_expires_at = LEAST($3, absolute_expires_at)
		WHERE id = $1::uuid AND revoked_at IS NULL`, id, seenAt, idleExpiresAt)
	if err != nil {
		return fmt.Errorf("auth repository: touching session: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) RotateSessionCSRF(ctx context.Context, id string, hash []byte, expiresAt time.Time) error {
	result, err := p.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET csrf_hash = $2, csrf_expires_at = LEAST($3, absolute_expires_at)
		WHERE id = $1::uuid AND revoked_at IS NULL`, id, hash, expiresAt)
	if err != nil {
		return fmt.Errorf("auth repository: rotating csrf: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) ReplaceSession(ctx context.Context, oldID string, replacement model.Session, now time.Time) error {
	if !validNewSession(replacement) {
		return ErrInvalidSession
	}

	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var oldUserID string
		var oldAbsoluteExpiry time.Time
		err := tx.QueryRow(ctx, `
			SELECT user_id::text, absolute_expires_at
			FROM auth_sessions
			WHERE id = $1::uuid
			  AND revoked_at IS NULL
			  AND idle_expires_at > $2
			  AND absolute_expires_at > $2
			FOR UPDATE`, oldID, now).Scan(&oldUserID, &oldAbsoluteExpiry)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("auth repository: locking rotated session: %w", err)
		}
		if replacement.UserID != oldUserID {
			return ErrInvalidSession
		}

		replacement.AbsoluteExpiresAt = minTime(replacement.AbsoluteExpiresAt, oldAbsoluteExpiry)
		replacement.IdleExpiresAt = minTime(replacement.IdleExpiresAt, replacement.AbsoluteExpiresAt)
		replacement.CSRFExpiresAt = minTime(replacement.CSRFExpiresAt, replacement.AbsoluteExpiresAt)
		if !validNewSession(replacement) {
			return ErrInvalidSession
		}

		if _, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at = $2 WHERE id = $1::uuid`, oldID, now); err != nil {
			return fmt.Errorf("auth repository: revoking rotated session: %w", err)
		}
		return insertSession(ctx, tx, replacement, oldID)
	})
	if err != nil {
		return fmt.Errorf("auth repository: replacing session: %w", err)
	}
	return nil
}

func (p *Postgres) RevokeUserSession(ctx context.Context, userID, sessionID string, now time.Time) error {
	return pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=COALESCE(revoked_at,$3) WHERE user_id=$1::uuid AND id=(SELECT device_id FROM auth_sessions WHERE id=$2::uuid AND user_id=$1::uuid)`, userID, sessionID, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,$3) WHERE user_id=$1::uuid AND (id=$2::uuid OR device_id=(SELECT device_id FROM auth_sessions WHERE id=$2::uuid AND user_id=$1::uuid))`, userID, sessionID, now)
		return err
	})
}

func (p *Postgres) RevokeOtherSessions(ctx context.Context, userID, currentSessionID string, now time.Time) error {
	return pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=COALESCE(revoked_at,$3) WHERE user_id=$1::uuid AND id IS DISTINCT FROM (SELECT device_id FROM auth_sessions WHERE id=$2::uuid AND user_id=$1::uuid)`, userID, currentSessionID, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,$3) WHERE user_id=$1::uuid AND id<>$2::uuid AND (device_id IS NULL OR device_id IS DISTINCT FROM (SELECT device_id FROM auth_sessions WHERE id=$2::uuid AND user_id=$1::uuid))`, userID, currentSessionID, now)
		return err
	})
}

func (p *Postgres) RevokeAllSessions(ctx context.Context, userID string, now time.Time) error {
	return pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1::uuid`, userID, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1::uuid`, userID, now)
		return err
	})
}

func (p *Postgres) CreateStudentCode(ctx context.Context, code model.StudentCode) error {
	_, err := p.pool.Exec(ctx, `
		INSERT INTO student_login_codes (id, student_user_id, code_hash, expires_at, created_by, created_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid, $6)`,
		code.ID, code.StudentUserID, code.CodeHash, code.ExpiresAt, code.CreatedBy, time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("auth repository: inserting student code: %w", err)
	}
	return nil
}

func (p *Postgres) ExchangeStudentCode(
	ctx context.Context,
	hash []byte,
	session model.Session,
	now time.Time,
) (model.User, model.Session, error) {
	if !validNewSession(session) {
		return model.User{}, model.Session{}, ErrInvalidSession
	}

	var user model.User
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			UPDATE student_login_codes c
			SET consumed_at = $2
			FROM users u
			WHERE c.code_hash = $1 AND c.consumed_at IS NULL AND c.expires_at > $2
			  AND u.id = c.student_user_id AND u.status = 'active'
			RETURNING u.id::text, COALESCE(u.email, ''), u.display_name, u.status, u.created_at`, hash, now,
		).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("auth repository: consuming student code: %w", err)
		}

		user.Roles, err = rolesForUser(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		session.UserID = user.ID
		if err := insertSession(ctx, tx, session, ""); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return model.User{}, model.Session{}, fmt.Errorf("auth repository: exchanging student code: %w", err)
	}
	return user, session, nil
}

func findOrCreateGuardianByEmail(ctx context.Context, tx pgx.Tx, email string, now time.Time) (model.User, error) {
	var user model.User
	err := tx.QueryRow(ctx, `
		SELECT id::text, email, display_name, status, created_at
		FROM users
		WHERE email = $1
		FOR UPDATE`, email).Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt)
	created := errors.Is(err, pgx.ErrNoRows)
	if err != nil && !created {
		return model.User{}, fmt.Errorf("auth repository: selecting user by email: %w", err)
	}
	if created {
		user = model.User{
			ID:          platformid.New(),
			Email:       email,
			DisplayName: email,
			Status:      "active",
			CreatedAt:   now,
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO users (id, email, display_name, status, created_at, updated_at)
			VALUES ($1::uuid, $2, $3, 'active', $4, $4)`,
			user.ID, user.Email, user.DisplayName, now,
		); err != nil {
			return model.User{}, fmt.Errorf("auth repository: inserting guardian user: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_grants (id, user_id, role, granted_at)
			VALUES ($1::uuid, $2::uuid, 'guardian', $3)`, platformid.New(), user.ID, now); err != nil {
			return model.User{}, fmt.Errorf("auth repository: granting guardian role: %w", err)
		}
	}

	roles, err := rolesForUser(ctx, tx, user.ID)
	if err != nil {
		return model.User{}, err
	}
	user.Roles = roles
	return user, nil
}

func insertSession(ctx context.Context, tx pgx.Tx, session model.Session, rotatedFromID string) error {
	var rotatedFrom any
	if rotatedFromID != "" {
		rotatedFrom = rotatedFromID
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO auth_sessions
			(id, user_id, token_hash, csrf_hash, csrf_expires_at, created_at, last_seen_at,
			 idle_expires_at, absolute_expires_at, user_agent_hash, ip_address_hash, rotated_from_id,
			 mfa_verified_at, mfa_method, device_id)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12::uuid, $13, NULLIF($14, ''), NULLIF($15, '')::uuid)`,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.CSRFHash,
		session.CSRFExpiresAt,
		session.CreatedAt,
		session.LastSeenAt,
		session.IdleExpiresAt,
		session.AbsoluteExpiresAt,
		session.UserAgentHash,
		session.IPAddressHash,
		rotatedFrom,
		session.MFAVerifiedAt,
		session.MFAMethod,
		session.DeviceID,
	)
	if err != nil {
		return fmt.Errorf("auth repository: inserting session: %w", err)
	}
	return nil
}

type rowQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func rolesForUser(ctx context.Context, db rowQuerier, userID string) ([]model.Role, error) {
	rows, err := db.Query(ctx, `SELECT role FROM role_grants WHERE user_id = $1::uuid AND revoked_at IS NULL ORDER BY role`, userID)
	if err != nil {
		return nil, fmt.Errorf("auth repository: selecting roles: %w", err)
	}
	defer rows.Close()
	roles := make([]model.Role, 0)
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, fmt.Errorf("auth repository: scanning role: %w", err)
		}
		roles = append(roles, model.Role(strings.ToLower(role)))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("auth repository: iterating roles: %w", err)
	}
	return roles, nil
}
