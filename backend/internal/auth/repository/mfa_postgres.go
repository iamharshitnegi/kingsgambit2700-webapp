package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
)

func (p *Postgres) StartTOTPEnrollment(ctx context.Context, method model.MFAMethod, now time.Time) error {
	if method.ID == "" || method.UserID == "" || len(method.EncryptedSecret) < 30 || method.VerifiedAt != nil || method.DisabledAt != nil {
		return ErrConflict
	}
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var existingID string
		var verifiedAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT id::text, verified_at
			FROM staff_mfa_methods
			WHERE user_id = $1::uuid AND method = 'totp' AND disabled_at IS NULL
			FOR UPDATE`, method.UserID).Scan(&existingID, &verifiedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("auth repository: locking active mfa method: %w", err)
		}
		if err == nil && verifiedAt != nil {
			return ErrConflict
		}
		if existingID != "" {
			if _, err := tx.Exec(ctx, `
				UPDATE staff_mfa_methods SET disabled_at = $2
				WHERE id = $1::uuid AND verified_at IS NULL AND disabled_at IS NULL`, existingID, now); err != nil {
				return fmt.Errorf("auth repository: replacing pending mfa method: %w", err)
			}
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO staff_mfa_methods
				(id, user_id, method, encrypted_secret, created_at)
			VALUES ($1::uuid, $2::uuid, 'totp', $3, $4)`,
			method.ID, method.UserID, method.EncryptedSecret, method.CreatedAt,
		)
		if err != nil {
			return translateMFAWriteError("inserting pending mfa method", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("auth repository: starting totp enrollment: %w", err)
	}
	return nil
}

func (p *Postgres) ActiveTOTPMethod(ctx context.Context, userID string) (model.MFAMethod, error) {
	var method model.MFAMethod
	err := p.pool.QueryRow(ctx, `
		SELECT id::text, user_id::text, encrypted_secret, verified_at, last_totp_step, created_at, disabled_at
		FROM staff_mfa_methods
		WHERE user_id = $1::uuid AND method = 'totp' AND disabled_at IS NULL`, userID,
	).Scan(
		&method.ID,
		&method.UserID,
		&method.EncryptedSecret,
		&method.VerifiedAt,
		&method.LastTOTPStep,
		&method.CreatedAt,
		&method.DisabledAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.MFAMethod{}, ErrNotFound
	}
	if err != nil {
		return model.MFAMethod{}, fmt.Errorf("auth repository: selecting active mfa method: %w", err)
	}
	return method, nil
}

func (p *Postgres) ConfirmTOTPEnrollment(
	ctx context.Context,
	confirmation MFAEnrollmentConfirmation,
) (model.Session, error) {
	if err := validateRecoveryCodes(confirmation.MethodID, confirmation.RecoveryCodes); err != nil {
		return model.Session{}, err
	}
	var replacement model.Session
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var verifiedAt *time.Time
		err := tx.QueryRow(ctx, `
			SELECT verified_at
			FROM staff_mfa_methods
			WHERE id = $1::uuid AND user_id = $2::uuid AND method = 'totp' AND disabled_at IS NULL
			FOR UPDATE`, confirmation.MethodID, confirmation.UserID).Scan(&verifiedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("auth repository: locking pending mfa method: %w", err)
		}
		if verifiedAt != nil {
			return ErrConflict
		}

		replacement, err = lockAndRotateMFASession(
			ctx,
			tx,
			confirmation.UserID,
			confirmation.CurrentSession,
			confirmation.Replacement,
			confirmation.At,
			true,
		)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE staff_mfa_methods
			SET verified_at = $2, last_totp_step = $3
			WHERE id = $1::uuid`, confirmation.MethodID, confirmation.At, confirmation.TOTPStep); err != nil {
			return fmt.Errorf("auth repository: verifying mfa method: %w", err)
		}
		for _, recoveryCode := range confirmation.RecoveryCodes {
			if _, err := tx.Exec(ctx, `
				INSERT INTO staff_mfa_recovery_codes (id, method_id, code_hash, created_at)
				VALUES ($1::uuid, $2::uuid, $3, $4)`,
				recoveryCode.ID, recoveryCode.MethodID, recoveryCode.CodeHash, recoveryCode.CreatedAt,
			); err != nil {
				return translateMFAWriteError("inserting mfa recovery code", err)
			}
		}
		return insertMFAAudit(ctx, tx, confirmation.Audit)
	})
	if err != nil {
		return model.Session{}, fmt.Errorf("auth repository: confirming totp enrollment: %w", err)
	}
	return replacement, nil
}

func (p *Postgres) CompleteTOTPChallenge(
	ctx context.Context,
	completion MFAChallengeCompletion,
) (model.Session, error) {
	var replacement model.Session
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var lastStep *int64
		err := tx.QueryRow(ctx, `
			SELECT last_totp_step
			FROM staff_mfa_methods
			WHERE id = $1::uuid AND user_id = $2::uuid AND method = 'totp'
			  AND verified_at IS NOT NULL AND disabled_at IS NULL
			FOR UPDATE`, completion.MethodID, completion.UserID).Scan(&lastStep)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("auth repository: locking verified mfa method: %w", err)
		}
		if lastStep != nil && completion.TOTPStep <= *lastStep {
			return ErrConsumed
		}

		replacement, err = lockAndRotateMFASession(
			ctx,
			tx,
			completion.UserID,
			completion.CurrentSession,
			completion.Replacement,
			completion.At,
			false,
		)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE staff_mfa_methods SET last_totp_step = $2 WHERE id = $1::uuid`,
			completion.MethodID, completion.TOTPStep,
		); err != nil {
			return fmt.Errorf("auth repository: advancing totp replay counter: %w", err)
		}
		return insertMFAAudit(ctx, tx, completion.Audit)
	})
	if err != nil {
		return model.Session{}, fmt.Errorf("auth repository: completing totp challenge: %w", err)
	}
	return replacement, nil
}

func (p *Postgres) CompleteRecoveryChallenge(
	ctx context.Context,
	completion MFARecoveryCompletion,
) (model.Session, error) {
	var replacement model.Session
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		var recoveryCodeID string
		err := tx.QueryRow(ctx, `
			SELECT c.id::text
			FROM staff_mfa_recovery_codes c
			JOIN staff_mfa_methods m ON m.id = c.method_id
			WHERE c.method_id = $1::uuid AND c.code_hash = $2 AND c.used_at IS NULL
			  AND m.user_id = $3::uuid AND m.verified_at IS NOT NULL AND m.disabled_at IS NULL
			FOR UPDATE OF c, m`, completion.MethodID, completion.RecoveryCodeHash, completion.UserID,
		).Scan(&recoveryCodeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrConsumed
		}
		if err != nil {
			return fmt.Errorf("auth repository: locking mfa recovery code: %w", err)
		}

		replacement, err = lockAndRotateMFASession(
			ctx,
			tx,
			completion.UserID,
			completion.CurrentSession,
			completion.Replacement,
			completion.At,
			false,
		)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE staff_mfa_recovery_codes SET used_at = $2 WHERE id = $1::uuid`,
			recoveryCodeID, completion.At,
		); err != nil {
			return fmt.Errorf("auth repository: consuming mfa recovery code: %w", err)
		}
		return insertMFAAudit(ctx, tx, completion.Audit)
	})
	if err != nil {
		return model.Session{}, fmt.Errorf("auth repository: completing recovery challenge: %w", err)
	}
	return replacement, nil
}

func (p *Postgres) ResetMFA(ctx context.Context, reset MFAReset) error {
	err := pgx.BeginTxFunc(ctx, p.pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		if err := lockDeviceUser(ctx, tx, reset.TargetUserID); err != nil {
			return err
		}
		var methodID string
		err := tx.QueryRow(ctx, `
			SELECT id::text
			FROM staff_mfa_methods
			WHERE user_id = $1::uuid AND method = 'totp' AND disabled_at IS NULL
			FOR UPDATE`, reset.TargetUserID).Scan(&methodID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("auth repository: locking mfa method for reset: %w", err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE staff_mfa_methods SET disabled_at = $2 WHERE id = $1::uuid`,
			methodID, reset.At,
		); err != nil {
			return fmt.Errorf("auth repository: disabling mfa method: %w", err)
		}
		if _, err := tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=COALESCE(revoked_at,$2) WHERE user_id=$1::uuid`, reset.TargetUserID, reset.At); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE auth_sessions
			SET revoked_at = COALESCE(revoked_at, $2)
			WHERE user_id = $1::uuid AND revoked_at IS NULL`, reset.TargetUserID, reset.At,
		); err != nil {
			return fmt.Errorf("auth repository: revoking sessions after mfa reset: %w", err)
		}
		return insertMFAAudit(ctx, tx, reset.Audit)
	})
	if err != nil {
		return fmt.Errorf("auth repository: resetting mfa: %w", err)
	}
	return nil
}

func lockAndRotateMFASession(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	currentSessionID string,
	replacement model.Session,
	now time.Time,
	requireUnverified bool,
) (model.Session, error) {
	if !validNewSession(replacement) || replacement.UserID != userID || replacement.MFAVerifiedAt != nil {
		return model.Session{}, ErrInvalidSession
	}
	var absoluteExpiresAt time.Time
	var mfaVerifiedAt *time.Time
	err := tx.QueryRow(ctx, `
		SELECT absolute_expires_at, mfa_verified_at
		FROM auth_sessions
		WHERE id = $1::uuid AND user_id = $2::uuid
		  AND revoked_at IS NULL AND idle_expires_at > $3 AND absolute_expires_at > $3
		FOR UPDATE`, currentSessionID, userID, now).Scan(&absoluteExpiresAt, &mfaVerifiedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Session{}, ErrInvalidSession
	}
	if err != nil {
		return model.Session{}, fmt.Errorf("auth repository: locking primary session: %w", err)
	}
	if requireUnverified && mfaVerifiedAt != nil {
		return model.Session{}, ErrInvalidSession
	}
	replacement.AbsoluteExpiresAt = minTime(replacement.AbsoluteExpiresAt, absoluteExpiresAt)
	replacement.IdleExpiresAt = minTime(replacement.IdleExpiresAt, replacement.AbsoluteExpiresAt)
	replacement.CSRFExpiresAt = minTime(replacement.CSRFExpiresAt, replacement.AbsoluteExpiresAt)
	verifiedAt := now
	replacement.MFAVerifiedAt = &verifiedAt
	replacement.MFAMethod = model.MFAMethodTOTP
	if !validNewSession(replacement) {
		return model.Session{}, ErrInvalidSession
	}
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions SET revoked_at = $2 WHERE id = $1::uuid`, currentSessionID, now,
	); err != nil {
		return model.Session{}, fmt.Errorf("auth repository: revoking primary session: %w", err)
	}
	if err := insertSession(ctx, tx, replacement, currentSessionID); err != nil {
		return model.Session{}, err
	}
	if replacement.DeviceID != "" {
		result, err := tx.Exec(ctx, `UPDATE auth_devices SET mfa_verified_at=$3,mfa_method=$4 WHERE id=$1::uuid AND user_id=$2::uuid AND revoked_at IS NULL`, replacement.DeviceID, userID, now, model.MFAMethodTOTP)
		if err != nil {
			return model.Session{}, err
		}
		if result.RowsAffected() != 1 {
			return model.Session{}, ErrInvalidSession
		}
	}
	return replacement, nil
}

func insertMFAAudit(ctx context.Context, tx pgx.Tx, event model.AuditEvent) error {
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("auth repository: encoding mfa audit metadata: %w", err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, actor_id, action, object_type, object_id,
			 reason_text, request_id, metadata)
		VALUES ($1::uuid, $2, 'user', $3::uuid, $4, 'staff_mfa', $5, NULLIF($6, ''), NULLIF($7, ''), $8::jsonb)`,
		event.ID,
		event.CreatedAt,
		event.ActorID,
		event.Action,
		event.ObjectID,
		event.Reason,
		event.RequestID,
		metadata,
	)
	if err != nil {
		return fmt.Errorf("auth repository: inserting mfa audit event: %w", err)
	}
	return nil
}

func translateMFAWriteError(operation string, err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return ErrConflict
	}
	return fmt.Errorf("auth repository: %s: %w", operation, err)
}
