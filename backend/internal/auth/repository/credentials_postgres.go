package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func (p *Postgres) CreatePasswordRegistration(ctx context.Context, registration model.PasswordRegistration) error {
	_, err := p.pool.Exec(ctx, `INSERT INTO auth_password_registrations
		(token_hash, email, username, display_name, password_hash, redirect_path, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, registration.TokenHash, registration.Email,
		registration.Username, registration.DisplayName, registration.PasswordHash,
		registration.RedirectPath, registration.ExpiresAt)
	return credentialError(err)
}

func (p *Postgres) CompletePasswordRegistration(ctx context.Context, hash []byte, session model.Session, now time.Time) (model.User, model.Session, string, error) {
	if !validNewSession(session) {
		return model.User{}, model.Session{}, "", ErrInvalidSession
	}
	var user model.User
	var registration model.PasswordRegistration
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `UPDATE auth_password_registrations SET consumed_at = $2
			WHERE token_hash = $1 AND consumed_at IS NULL AND expires_at > $2
			RETURNING email, username, display_name, password_hash, redirect_path`, hash, now).
			Scan(&registration.Email, &registration.Username, &registration.DisplayName,
				&registration.PasswordHash, &registration.RedirectPath)
		if err != nil {
			return credentialError(err)
		}
		// Never attach an unauthenticated signup's password to an existing account.
		user, err = insertGuardian(ctx, tx, registration.Email, registration.DisplayName, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO auth_password_credentials (user_id, username, password_hash, updated_at)
			VALUES ($1::uuid, $2, $3, $4)`, user.ID, registration.Username, registration.PasswordHash, now)
		if err != nil {
			return credentialError(err)
		}
		session.UserID = user.ID
		return insertSession(ctx, tx, session, "")
	})
	return user, session, registration.RedirectPath, credentialError(err)
}

func (p *Postgres) PasswordIdentity(ctx context.Context, login string) (model.PasswordIdentity, error) {
	var identity model.PasswordIdentity
	err := p.pool.QueryRow(ctx, `SELECT c.user_id::text, c.password_hash
		FROM auth_password_credentials c JOIN users u ON u.id = c.user_id
		WHERE u.status = 'active' AND
		  (($2 AND u.email = $1) OR (NOT $2 AND c.username = $1))`, login, strings.Contains(login, "@")).
		Scan(&identity.UserID, &identity.PasswordHash)
	return identity, credentialError(err)
}

func (p *Postgres) CreatePasswordSession(ctx context.Context, identity model.PasswordIdentity, session model.Session, now time.Time) (model.User, model.Session, error) {
	if !validNewSession(session) {
		return model.User{}, model.Session{}, ErrInvalidSession
	}
	var user model.User
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// Recheck status and credential under lock after the expensive password check.
		err := tx.QueryRow(ctx, `SELECT u.id::text, COALESCE(u.email, ''), u.display_name, u.status, u.created_at
			FROM users u JOIN auth_password_credentials c ON c.user_id = u.id
			WHERE u.id = $1::uuid AND c.password_hash = $2 AND u.status = 'active'
			FOR UPDATE OF u, c`, identity.UserID, identity.PasswordHash).
			Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt)
		if err != nil {
			return credentialError(err)
		}
		user.Roles, err = rolesForUser(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		session.UserID = user.ID
		return insertSession(ctx, tx, session, "")
	})
	return user, session, credentialError(err)
}

func (p *Postgres) CreateGoogleChallenge(ctx context.Context, hash []byte, expires time.Time) error {
	return pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM auth_google_challenges WHERE expires_at < now()`); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO auth_google_challenges (nonce_hash, expires_at) VALUES ($1, $2)`, hash, expires)
		return credentialError(err)
	})
}

func (p *Postgres) ExchangeGoogle(ctx context.Context, identity model.GoogleIdentity, nonce []byte, session model.Session, now time.Time) (model.User, model.Session, error) {
	if !validNewSession(session) {
		return model.User{}, model.Session{}, ErrInvalidSession
	}
	var user model.User
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `DELETE FROM auth_google_challenges WHERE nonce_hash = $1 AND expires_at > $2`, nonce, now)
		if err != nil {
			return err
		}
		if result.RowsAffected() != 1 {
			return ErrNotFound
		}
		// Serialize first sign-ins even when no identity row exists yet.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "google:"+identity.Subject); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT u.id::text, COALESCE(u.email, ''), u.display_name, u.status, u.created_at
			FROM auth_google_identities g JOIN users u ON u.id = g.user_id
			WHERE g.subject = $1 FOR UPDATE OF u`, identity.Subject).
			Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			// External Google-account email ownership may have changed since Google
			// verified it. Do not create a mailbox account or link one on that proof.
			if !identity.EmailAuthoritative {
				return ErrConflict
			}
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, "google-email:"+identity.Email); err != nil {
				return err
			}
			err = tx.QueryRow(ctx, `SELECT id::text, email, display_name, status, created_at FROM users WHERE email = $1 FOR UPDATE`, identity.Email).
				Scan(&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt)
			if errors.Is(err, pgx.ErrNoRows) {
				user, err = insertGuardian(ctx, tx, identity.Email, identity.DisplayName, now)
			}
			if err != nil {
				return credentialError(err)
			}
			if user.Status != "active" {
				return ErrNotFound
			}
			_, err = tx.Exec(ctx, `INSERT INTO auth_google_identities (subject, user_id, created_at) VALUES ($1, $2::uuid, $3)`, identity.Subject, user.ID, now)
		}
		if err != nil {
			return credentialError(err)
		}
		if user.Status != "active" {
			return ErrNotFound
		}
		user.Roles, err = rolesForUser(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		session.UserID = user.ID
		return insertSession(ctx, tx, session, "")
	})
	return user, session, credentialError(err)
}

func insertGuardian(ctx context.Context, tx pgx.Tx, email, name string, now time.Time) (model.User, error) {
	user := model.User{ID: platformid.New(), Email: email, DisplayName: name, Status: "active", CreatedAt: now, Roles: []model.Role{model.RoleGuardian}}
	_, err := tx.Exec(ctx, `INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, 'active', $4, $4)`, user.ID, email, name, now)
	if err != nil {
		return model.User{}, credentialError(err)
	}
	_, err = tx.Exec(ctx, `INSERT INTO role_grants (id, user_id, role, granted_at) VALUES ($1::uuid, $2::uuid, 'guardian', $3)`, platformid.New(), user.ID, now)
	return user, credentialError(err)
}

func credentialError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return fmt.Errorf("auth credentials: %w", err)
}
