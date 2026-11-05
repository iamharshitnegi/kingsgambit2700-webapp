package repository

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
)

func (p *Postgres) RememberSession(ctx context.Context, sessionID string, device model.Device, csrfHash []byte, now time.Time) (model.Device, error) {
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, device.UserID); err != nil {
			return err
		}
		var currentDevice string
		var status string
		err := tx.QueryRow(ctx, `SELECT COALESCE(s.device_id::text, ''), s.mfa_verified_at, COALESCE(s.mfa_method, ''), u.status
   FROM auth_sessions s JOIN users u ON u.id=s.user_id
   WHERE s.id=$1::uuid AND s.user_id=$2::uuid AND s.revoked_at IS NULL AND s.idle_expires_at>$3 AND s.absolute_expires_at>$3
   FOR UPDATE OF s`, sessionID, device.UserID, now).Scan(&currentDevice, &device.MFAVerifiedAt, &device.MFAMethod, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "active" {
			return ErrInvalidSession
		}
		roles, err := rolesForUser(ctx, tx, device.UserID)
		if err != nil {
			return err
		}
		if (model.User{Roles: roles}).RequiresMFA() && device.MFAVerifiedAt == nil {
			return ErrInvalidSession
		}
		if currentDevice != "" {
			var existingHash []byte
			var revokedAt *time.Time
			if err := tx.QueryRow(ctx, `SELECT token_hash, revoked_at FROM auth_devices WHERE id=$1::uuid`, currentDevice).Scan(&existingHash, &revokedAt); err != nil {
				return err
			}
			if revokedAt != nil || subtle.ConstantTimeCompare(existingHash, device.TokenHash) != 1 {
				return ErrConflict
			}
			device.ID = currentDevice
			return nil
		}
		_, err = tx.Exec(ctx, `INSERT INTO auth_devices(id,user_id,token_hash,created_at,last_seen_at,mfa_verified_at,mfa_method)
   VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,NULLIF($7,''))`, device.ID, device.UserID, device.TokenHash, device.CreatedAt, device.LastSeenAt, device.MFAVerifiedAt, device.MFAMethod)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE auth_sessions SET device_id=$2::uuid, csrf_hash=$3, csrf_expires_at=absolute_expires_at WHERE id=$1::uuid`, sessionID, device.ID, csrfHash)
		return err
	})
	if err != nil {
		return model.Device{}, fmt.Errorf("remembering session: %w", err)
	}
	return device, nil
}

func (p *Postgres) DeviceByID(ctx context.Context, id string) (model.Device, model.User, error) {
	return p.findDevice(ctx, `d.id=$1::uuid`, id)
}
func (p *Postgres) DeviceByTokenHash(ctx context.Context, hash []byte) (model.Device, model.User, error) {
	return p.findDevice(ctx, `d.token_hash=$1`, hash)
}
func (p *Postgres) findDevice(ctx context.Context, predicate string, value any) (model.Device, model.User, error) {
	var device model.Device
	var user model.User
	// predicate is chosen only by the two fixed queries above, never from user input.
	err := p.pool.QueryRow(ctx, `SELECT d.id::text,d.user_id::text,d.token_hash,d.created_at,d.last_seen_at,d.revoked_at,d.mfa_verified_at,COALESCE(d.mfa_method,''),
  u.id::text,COALESCE(u.email,''),u.display_name,u.status,u.created_at FROM auth_devices d JOIN users u ON u.id=d.user_id WHERE `+predicate, value).
		Scan(&device.ID, &device.UserID, &device.TokenHash, &device.CreatedAt, &device.LastSeenAt, &device.RevokedAt, &device.MFAVerifiedAt, &device.MFAMethod,
			&user.ID, &user.Email, &user.DisplayName, &user.Status, &user.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Device{}, model.User{}, ErrNotFound
	}
	if err != nil {
		return model.Device{}, model.User{}, fmt.Errorf("reading remembered device: %w", err)
	}
	user.Roles, err = rolesForUser(ctx, p.pool, user.ID)
	return device, user, err
}

func (p *Postgres) RestoreDeviceSession(ctx context.Context, device model.Device, session model.Session, now time.Time) error {
	if !validNewSession(session) || session.DeviceID != device.ID || session.UserID != device.UserID {
		return ErrInvalidSession
	}
	return pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, device.UserID); err != nil {
			return err
		}
		var id string
		err := tx.QueryRow(ctx, `SELECT d.id::text FROM auth_devices d JOIN users u ON u.id=d.user_id WHERE d.id=$1::uuid AND d.user_id=$2::uuid AND d.revoked_at IS NULL AND u.status='active' FOR UPDATE OF d`, device.ID, session.UserID).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := insertSession(ctx, tx, session, ""); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE auth_devices SET last_seen_at=$2 WHERE id=$1::uuid`, device.ID, now)
		return err
	})
}

func (p *Postgres) RevokeDevice(ctx context.Context, userID, deviceID string, now time.Time) error {
	return pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=COALESCE(revoked_at,$3) WHERE id=$1::uuid AND user_id=$2::uuid`, deviceID, userID, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=COALESCE(revoked_at,$3) WHERE device_id=$1::uuid AND user_id=$2::uuid`, deviceID, userID, now)
		return err
	})
}

// Serialize credential creation, restoration and revocation for a user. Without
// this lock, revoke-all can miss a device being attached by another transaction.
func lockDeviceUser(ctx context.Context, tx pgx.Tx, userID string) error {
	var id string
	err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE id=$1::uuid FOR UPDATE`, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
