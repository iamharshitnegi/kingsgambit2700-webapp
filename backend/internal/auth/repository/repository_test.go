package repository

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestMemory_ExchangeMagicLinkRollsBackConsumptionWhenSessionCreationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, time.August, 27, 9, 0, 0, 0, time.UTC)
	repo := NewMemory()

	firstHash := digestForTest(1)
	if err := repo.CreateMagicLink(ctx, magicLinkForTest(firstHash, "parent@example.com", now)); err != nil {
		t.Fatalf("CreateMagicLink(first) error = %v", err)
	}
	occupiedSession := sessionForTest(10, now)
	if _, _, _, err := repo.ExchangeMagicLink(ctx, firstHash, occupiedSession, now); err != nil {
		t.Fatalf("ExchangeMagicLink(first) error = %v", err)
	}

	secondHash := digestForTest(2)
	if err := repo.CreateMagicLink(ctx, magicLinkForTest(secondHash, "parent@example.com", now)); err != nil {
		t.Fatalf("CreateMagicLink(second) error = %v", err)
	}
	duplicateSession := sessionForTest(10, now)
	if _, _, _, err := repo.ExchangeMagicLink(ctx, secondHash, duplicateSession, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("ExchangeMagicLink(duplicate session) error = %v, want %v", err, ErrConflict)
	}

	retrySession := sessionForTest(11, now)
	link, _, persisted, err := repo.ExchangeMagicLink(ctx, secondHash, retrySession, now)
	if err != nil {
		t.Fatalf("ExchangeMagicLink(retry) error = %v", err)
	}
	if link.ConsumedAt == nil {
		t.Fatal("ExchangeMagicLink(retry) did not consume the link")
	}
	if !bytes.Equal(persisted.TokenHash, retrySession.TokenHash) {
		t.Fatal("ExchangeMagicLink(retry) persisted the wrong session")
	}
}

func TestMemory_ExchangeStudentCodeRollsBackConsumptionWhenSessionCreationFails(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, time.August, 27, 9, 0, 0, 0, time.UTC)
	repo := NewMemory()
	student := model.User{
		ID:          platformid.New(),
		DisplayName: "Student",
		Roles:       []model.Role{model.RoleStudent},
		Status:      "active",
		CreatedAt:   now,
	}
	repo.PutUser(student)

	firstHash := digestForTest(21)
	if err := repo.CreateStudentCode(ctx, studentCodeForTest(firstHash, student.ID, now)); err != nil {
		t.Fatalf("CreateStudentCode(first) error = %v", err)
	}
	occupiedSession := sessionForTest(30, now)
	if _, _, err := repo.ExchangeStudentCode(ctx, firstHash, occupiedSession, now); err != nil {
		t.Fatalf("ExchangeStudentCode(first) error = %v", err)
	}

	secondHash := digestForTest(22)
	if err := repo.CreateStudentCode(ctx, studentCodeForTest(secondHash, student.ID, now)); err != nil {
		t.Fatalf("CreateStudentCode(second) error = %v", err)
	}
	duplicateSession := sessionForTest(30, now)
	if _, _, err := repo.ExchangeStudentCode(ctx, secondHash, duplicateSession, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("ExchangeStudentCode(duplicate session) error = %v, want %v", err, ErrConflict)
	}

	retrySession := sessionForTest(31, now)
	user, persisted, err := repo.ExchangeStudentCode(ctx, secondHash, retrySession, now)
	if err != nil {
		t.Fatalf("ExchangeStudentCode(retry) error = %v", err)
	}
	if user.ID != student.ID {
		t.Fatalf("ExchangeStudentCode(retry) user ID = %q, want %q", user.ID, student.ID)
	}
	if !bytes.Equal(persisted.TokenHash, retrySession.TokenHash) {
		t.Fatal("ExchangeStudentCode(retry) persisted the wrong session")
	}
}

func TestMemory_ReplaceSessionCannotExtendOriginalAbsoluteExpiry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, time.August, 27, 9, 0, 0, 0, time.UTC)
	repo := NewMemory()
	linkHash := digestForTest(71)
	if err := repo.CreateMagicLink(ctx, magicLinkForTest(linkHash, "parent@example.com", now)); err != nil {
		t.Fatalf("CreateMagicLink() error = %v", err)
	}
	original := sessionForTest(72, now)
	_, user, original, err := repo.ExchangeMagicLink(ctx, linkHash, original, now)
	if err != nil {
		t.Fatalf("ExchangeMagicLink() error = %v", err)
	}

	replacementTime := now.Add(30 * time.Minute)
	replacement := sessionForTest(73, replacementTime)
	replacement.UserID = user.ID
	replacement.AbsoluteExpiresAt = original.AbsoluteExpiresAt.Add(24 * time.Hour)
	if err := repo.ReplaceSession(ctx, original.ID, replacement, replacementTime); err != nil {
		t.Fatalf("ReplaceSession() error = %v", err)
	}
	persisted, _, err := repo.SessionByTokenHash(ctx, replacement.TokenHash)
	if err != nil {
		t.Fatalf("SessionByTokenHash(replacement) error = %v", err)
	}
	if !persisted.AbsoluteExpiresAt.Equal(original.AbsoluteExpiresAt) {
		t.Fatalf(
			"replacement absolute expiry = %s, want original %s",
			persisted.AbsoluteExpiresAt,
			original.AbsoluteExpiresAt,
		)
	}
}

func magicLinkForTest(hash []byte, email string, now time.Time) model.MagicLink {
	return model.MagicLink{
		ID:           platformid.New(),
		Email:        email,
		TokenHash:    hash,
		RedirectPath: "/portal",
		ExpiresAt:    now.Add(15 * time.Minute),
		CreatedAt:    now,
	}
}

func studentCodeForTest(hash []byte, studentUserID string, now time.Time) model.StudentCode {
	return model.StudentCode{
		ID:            platformid.New(),
		StudentUserID: studentUserID,
		CodeHash:      hash,
		ExpiresAt:     now.Add(10 * time.Minute),
		CreatedBy:     platformid.New(),
	}
}

func sessionForTest(hashByte byte, now time.Time) model.Session {
	return model.Session{
		ID:                platformid.New(),
		TokenHash:         digestForTest(hashByte),
		CSRFHash:          digestForTest(hashByte + 1),
		CSRFExpiresAt:     now.Add(time.Hour),
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     now.Add(time.Hour),
		AbsoluteExpiresAt: now.Add(24 * time.Hour),
		UserAgentHash:     digestForTest(hashByte + 2),
		IPAddressHash:     digestForTest(hashByte + 3),
	}
}

func digestForTest(value byte) []byte {
	return bytes.Repeat([]byte{value}, sha256DigestSize)
}
