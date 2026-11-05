//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgres_CredentialExchangeRollsBackWhenSessionInsertFails(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("KG_DATABASE_URL")
	}
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL and KG_DATABASE_URL are not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)

	repo := NewPostgres(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	identity := strings.ReplaceAll(platformid.New(), "-", "")
	guardianEmail := "auth-integration-" + identity + "@example.com"
	firstMagicHash := integrationDigest(identity + ":magic:first")
	secondMagicHash := integrationDigest(identity + ":magic:second")
	firstLink := postgresMagicLink(firstMagicHash, guardianEmail, now)
	secondLink := postgresMagicLink(secondMagicHash, guardianEmail, now)
	if err := repo.CreateMagicLink(ctx, firstLink); err != nil {
		t.Fatalf("CreateMagicLink(first) error = %v", err)
	}

	occupied := postgresSession(identity+":occupied", now)
	_, guardian, _, err := repo.ExchangeMagicLink(ctx, firstMagicHash, occupied, now)
	if err != nil {
		t.Fatalf("ExchangeMagicLink(first) error = %v", err)
	}
	studentID := platformid.New()
	creatorID := platformid.New()
	studentRoleID := platformid.New()
	studentCodeID := platformid.New()
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		statements := []struct {
			query string
			args  []any
		}{
			{"DELETE FROM auth_sessions WHERE user_id IN ($1::uuid, $2::uuid)", []any{guardian.ID, studentID}},
			{"DELETE FROM student_login_codes WHERE id = $1::uuid", []any{studentCodeID}},
			{"DELETE FROM auth_magic_links WHERE id IN ($1::uuid, $2::uuid)", []any{firstLink.ID, secondLink.ID}},
			{"DELETE FROM role_grants WHERE user_id IN ($1::uuid, $2::uuid)", []any{guardian.ID, studentID}},
			{"DELETE FROM users WHERE id IN ($1::uuid, $2::uuid, $3::uuid)", []any{guardian.ID, studentID, creatorID}},
		}
		for _, statement := range statements {
			if _, err := pool.Exec(cleanupCtx, statement.query, statement.args...); err != nil {
				t.Errorf("cleanup query %q: %v", statement.query, err)
			}
		}
	})

	if err := repo.CreateMagicLink(ctx, secondLink); err != nil {
		t.Fatalf("CreateMagicLink(second) error = %v", err)
	}
	if _, _, _, err := repo.ExchangeMagicLink(ctx, secondMagicHash, occupied, now); err == nil {
		t.Fatal("ExchangeMagicLink(duplicate session) error = nil, want conflict")
	}
	if _, _, _, err := repo.ExchangeMagicLink(ctx, secondMagicHash, postgresSession(identity+":magic:retry", now), now); err != nil {
		t.Fatalf("ExchangeMagicLink(retry) error = %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, display_name, status, created_at, updated_at)
		VALUES ($1::uuid, 'Integration Student', 'active', $3, $3),
		       ($2::uuid, 'Integration Guardian', 'active', $3, $3)`,
		studentID, creatorID, now,
	); err != nil {
		t.Fatalf("seeding student users: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO role_grants (id, user_id, role, granted_at)
		VALUES ($1::uuid, $2::uuid, 'student', $3)`, studentRoleID, studentID, now); err != nil {
		t.Fatalf("seeding student role: %v", err)
	}
	studentCodeHash := integrationDigest(identity + ":student-code")
	if err := repo.CreateStudentCode(ctx, model.StudentCode{
		ID:            studentCodeID,
		StudentUserID: studentID,
		CodeHash:      studentCodeHash,
		ExpiresAt:     now.Add(10 * time.Minute),
		CreatedBy:     creatorID,
	}); err != nil {
		t.Fatalf("CreateStudentCode() error = %v", err)
	}
	if _, _, err := repo.ExchangeStudentCode(ctx, studentCodeHash, occupied, now); err == nil {
		t.Fatal("ExchangeStudentCode(duplicate session) error = nil, want conflict")
	}
	if _, _, err := repo.ExchangeStudentCode(ctx, studentCodeHash, postgresSession(identity+":student:retry", now), now); err != nil {
		t.Fatalf("ExchangeStudentCode(retry) error = %v", err)
	}
}

func postgresMagicLink(hash []byte, email string, now time.Time) model.MagicLink {
	return model.MagicLink{
		ID:           platformid.New(),
		Email:        email,
		TokenHash:    hash,
		RedirectPath: "/portal",
		ExpiresAt:    now.Add(15 * time.Minute),
		CreatedAt:    now,
	}
}

func postgresSession(seed string, now time.Time) model.Session {
	return model.Session{
		ID:                platformid.New(),
		TokenHash:         integrationDigest(seed + ":token"),
		CSRFHash:          integrationDigest(seed + ":csrf"),
		CSRFExpiresAt:     now.Add(time.Hour),
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     now.Add(6 * time.Hour),
		AbsoluteExpiresAt: now.Add(24 * time.Hour),
		UserAgentHash:     integrationDigest(seed + ":user-agent"),
		IPAddressHash:     integrationDigest(seed + ":ip-address"),
	}
}

func integrationDigest(value string) []byte {
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}
