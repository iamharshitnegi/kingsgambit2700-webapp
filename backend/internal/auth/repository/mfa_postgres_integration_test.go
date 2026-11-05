//go:build integration

package repository

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgres_MFAEnrollmentChallengeRecoveryAndResetAreAtomic(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("KG_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL and KG_DATABASE_URL are not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	repo := NewPostgres(pool)
	ctx := context.Background()
	now := time.Date(2026, time.August, 27, 13, 0, 0, 0, time.UTC)

	userID := platformid.New()
	roleID := platformid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES ($1::uuid, 'mfa-integration@example.com', 'MFA Integration Coach', 'active', $2, $2)`, userID, now); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO role_grants (id, user_id, role, granted_at)
		VALUES ($1::uuid, $2::uuid, 'coach', $3)`, roleID, userID, now); err != nil {
		t.Fatalf("seeding role: %v", err)
	}

	primary := insertPrimarySessionForMFA(t, ctx, repo, userID, "mfa-integration@example.com", now, "primary")
	method := model.MFAMethod{
		ID:              platformid.New(),
		UserID:          userID,
		EncryptedSecret: bytes.Repeat([]byte{0x9a}, 64),
		CreatedAt:       now,
	}
	if err := repo.StartTOTPEnrollment(ctx, method, now); err != nil {
		t.Fatalf("StartTOTPEnrollment() error = %v", err)
	}
	recoveryHash := integrationDigest("recovery-one")
	recoveryCode := model.MFARecoveryCode{
		ID:        platformid.New(),
		MethodID:  method.ID,
		CodeHash:  recoveryHash,
		CreatedAt: now,
	}
	replacement := postgresSession("mfa-confirmed", now)
	replacement.UserID = userID
	persisted, err := repo.ConfirmTOTPEnrollment(ctx, MFAEnrollmentConfirmation{
		MethodID:       method.ID,
		UserID:         userID,
		CurrentSession: primary.ID,
		TOTPStep:       100,
		RecoveryCodes:  []model.MFARecoveryCode{recoveryCode},
		Replacement:    replacement,
		Audit:          integrationMFAAudit(userID, "mfa.totp.enrolled", method.ID, now),
		At:             now,
	})
	if err != nil {
		t.Fatalf("ConfirmTOTPEnrollment() error = %v", err)
	}
	if persisted.MFAVerifiedAt == nil || persisted.MFAMethod != model.MFAMethodTOTP {
		t.Fatalf("confirmed session assurance = %+v", persisted)
	}
	old, _, err := repo.SessionByTokenHash(ctx, primary.TokenHash)
	if err != nil {
		t.Fatalf("SessionByTokenHash(primary) error = %v", err)
	}
	if old.RevokedAt == nil {
		t.Fatal("enrollment did not revoke primary session")
	}
	stepUpAt := now.Add(time.Minute)
	stepUpReplacement := postgresSession("mfa-step-up", stepUpAt)
	stepUpReplacement.UserID = userID
	stepUp, err := repo.CompleteTOTPChallenge(ctx, MFAChallengeCompletion{
		MethodID:       method.ID,
		UserID:         userID,
		CurrentSession: persisted.ID,
		TOTPStep:       101,
		Replacement:    stepUpReplacement,
		Audit:          integrationMFAAudit(userID, "mfa.challenge.completed", method.ID, stepUpAt),
		At:             stepUpAt,
	})
	if err != nil {
		t.Fatalf("CompleteTOTPChallenge(step-up) error = %v", err)
	}
	if stepUp.MFAVerifiedAt == nil || !stepUp.MFAVerifiedAt.Equal(stepUpAt) {
		t.Fatalf("step-up session assurance = %+v", stepUp)
	}
	previousAssured, _, err := repo.SessionByTokenHash(ctx, persisted.TokenHash)
	if err != nil {
		t.Fatalf("SessionByTokenHash(previous assured) error = %v", err)
	}
	if previousAssured.RevokedAt == nil {
		t.Fatal("step-up did not revoke the previous assured session")
	}

	replayedStepPrimary := insertPrimarySessionForMFA(t, ctx, repo, userID, "mfa-integration@example.com", now.Add(2*time.Minute), "step-replay")
	replayedStepReplacement := postgresSession("step-replay-replacement", now.Add(2*time.Minute))
	replayedStepReplacement.UserID = userID
	if _, err := repo.CompleteTOTPChallenge(ctx, MFAChallengeCompletion{
		MethodID:       method.ID,
		UserID:         userID,
		CurrentSession: replayedStepPrimary.ID,
		TOTPStep:       100,
		Replacement:    replayedStepReplacement,
		Audit:          integrationMFAAudit(userID, "mfa.challenge.completed", method.ID, now.Add(2*time.Minute)),
		At:             now.Add(2 * time.Minute),
	}); !errors.Is(err, ErrConsumed) {
		t.Fatalf("CompleteTOTPChallenge(replay) error = %v, want %v", err, ErrConsumed)
	}
	replayStillActive, _, err := repo.SessionByTokenHash(ctx, replayedStepPrimary.TokenHash)
	if err != nil || replayStillActive.RevokedAt != nil {
		t.Fatalf("failed replay mutated primary session: session=%+v error=%v", replayStillActive, err)
	}

	recoveryPrimary := insertPrimarySessionForMFA(t, ctx, repo, userID, "mfa-integration@example.com", now.Add(3*time.Minute), "recovery")
	recoveryReplacement := postgresSession("recovery-replacement", now.Add(3*time.Minute))
	recoveryReplacement.UserID = userID
	if _, err := repo.CompleteRecoveryChallenge(ctx, MFARecoveryCompletion{
		MethodID:         method.ID,
		UserID:           userID,
		CurrentSession:   recoveryPrimary.ID,
		RecoveryCodeHash: recoveryHash,
		Replacement:      recoveryReplacement,
		Audit:            integrationMFAAudit(userID, "mfa.challenge.completed", method.ID, now.Add(3*time.Minute)),
		At:               now.Add(3 * time.Minute),
	}); err != nil {
		t.Fatalf("CompleteRecoveryChallenge() error = %v", err)
	}

	recoveryReplayPrimary := insertPrimarySessionForMFA(t, ctx, repo, userID, "mfa-integration@example.com", now.Add(4*time.Minute), "recovery-replay")
	recoveryReplayReplacement := postgresSession("recovery-replay-replacement", now.Add(4*time.Minute))
	recoveryReplayReplacement.UserID = userID
	if _, err := repo.CompleteRecoveryChallenge(ctx, MFARecoveryCompletion{
		MethodID:         method.ID,
		UserID:           userID,
		CurrentSession:   recoveryReplayPrimary.ID,
		RecoveryCodeHash: recoveryHash,
		Replacement:      recoveryReplayReplacement,
		Audit:            integrationMFAAudit(userID, "mfa.challenge.completed", method.ID, now.Add(4*time.Minute)),
		At:               now.Add(4 * time.Minute),
	}); !errors.Is(err, ErrConsumed) {
		t.Fatalf("CompleteRecoveryChallenge(replay) error = %v, want %v", err, ErrConsumed)
	}

	if err := repo.ResetMFA(ctx, MFAReset{
		ActorID:      userID,
		TargetUserID: userID,
		Audit: model.AuditEvent{
			ID:        platformid.New(),
			ActorID:   userID,
			Action:    "mfa.reset",
			ObjectID:  userID,
			Reason:    "integration recovery validation",
			RequestID: "integration-reset",
			Metadata:  map[string]any{},
			CreatedAt: now.Add(5 * time.Minute),
		},
		At: now.Add(5 * time.Minute),
	}); err != nil {
		t.Fatalf("ResetMFA() error = %v", err)
	}
	if _, err := repo.ActiveTOTPMethod(ctx, userID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ActiveTOTPMethod(after reset) error = %v, want %v", err, ErrNotFound)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM audit_events
		WHERE actor_id = $1::uuid AND action IN ('mfa.totp.enrolled', 'mfa.challenge.completed', 'mfa.reset')`, userID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("counting audit rows: %v", err)
	}
	if auditCount != 4 {
		t.Fatalf("audit event count = %d, want 4", auditCount)
	}
}

func insertPrimarySessionForMFA(
	t *testing.T,
	ctx context.Context,
	repo *Postgres,
	userID string,
	email string,
	now time.Time,
	seed string,
) model.Session {
	t.Helper()
	link := postgresMagicLink(integrationDigest(seed+":magic"), email, now)
	if err := repo.CreateMagicLink(ctx, link); err != nil {
		t.Fatalf("CreateMagicLink(%s) error = %v", seed, err)
	}
	session := postgresSession(seed+":session", now)
	_, user, session, err := repo.ExchangeMagicLink(ctx, link.TokenHash, session, now)
	if err != nil {
		t.Fatalf("ExchangeMagicLink(%s) error = %v", seed, err)
	}
	if user.ID != userID {
		t.Fatalf("ExchangeMagicLink(%s) user = %q, want %q", seed, user.ID, userID)
	}
	return session
}

func integrationMFAAudit(userID, action, objectID string, at time.Time) model.AuditEvent {
	return model.AuditEvent{
		ID:        platformid.New(),
		ActorID:   userID,
		Action:    action,
		ObjectID:  objectID,
		RequestID: "integration-request",
		Metadata:  map[string]any{},
		CreatedAt: at,
	}
}
