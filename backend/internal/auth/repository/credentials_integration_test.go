//go:build integration

package repository

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresPasswordAndGoogleCredentials(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	repo := NewPostgres(pool)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Microsecond)
	registration := model.PasswordRegistration{TokenHash: integrationDigest("register"), Email: "parent@gmail.com", Username: "parent", DisplayName: "Parent", PasswordHash: "test-hash", RedirectPath: "/portal", ExpiresAt: now.Add(time.Hour)}
	if err := repo.CreatePasswordRegistration(ctx, registration); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PasswordIdentity(ctx, "parent"); !errors.Is(err, ErrNotFound) {
		t.Fatal("unverified signup is usable")
	}
	user, session, _, err := repo.CompletePasswordRegistration(ctx, registration.TokenHash, postgresSession("signup-session", now), now)
	if err != nil || len(user.Roles) != 1 || user.Roles[0] != model.RoleGuardian {
		t.Fatalf("registration: %+v %v", user, err)
	}
	if _, _, _, err := repo.CompletePasswordRegistration(ctx, registration.TokenHash, postgresSession("replay", now), now); !errors.Is(err, ErrNotFound) {
		t.Fatal("replayed signup")
	}
	for _, login := range []string{"PARENT", "PARENT@gmail.com"} {
		identity, err := repo.PasswordIdentity(ctx, login)
		if err != nil || identity.UserID != user.ID {
			t.Fatalf("lookup %s: %+v %v", login, identity, err)
		}
		if _, _, err := repo.CreatePasswordSession(ctx, identity, postgresSession(login, now), now); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := repo.CreatePasswordSession(ctx, model.PasswordIdentity{UserID: user.ID, PasswordHash: "old-hash"}, postgresSession("old", now), now); !errors.Is(err, ErrNotFound) {
		t.Fatal("accepted stale password hash")
	}
	// An existing mailbox must never receive an unauthenticated signup's password.
	registration.TokenHash = integrationDigest("takeover")
	registration.PasswordHash = "attacker-hash"
	if err := repo.CreatePasswordRegistration(ctx, registration); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := repo.CompletePasswordRegistration(ctx, registration.TokenHash, postgresSession("takeover", now), now); !errors.Is(err, ErrConflict) {
		t.Fatalf("existing account registration = %v", err)
	}
	identity, err := repo.PasswordIdentity(ctx, "parent")
	if err != nil || identity.PasswordHash != "test-hash" {
		t.Fatal("existing credentials changed")
	}
	// A failed session insert rolls back signup and leaves the token usable.
	registration.TokenHash = integrationDigest("rollback")
	registration.Email, registration.Username = "second@example.test", "second"
	if err := repo.CreatePasswordRegistration(ctx, registration); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := repo.CompletePasswordRegistration(ctx, registration.TokenHash, session, now); err == nil {
		t.Fatal("duplicate session accepted")
	}
	if _, _, _, err := repo.CompletePasswordRegistration(ctx, registration.TokenHash, postgresSession("retry-signup", now), now); err != nil {
		t.Fatalf("registration did not roll back: %v", err)
	}

	google := model.GoogleIdentity{Subject: "google-subject", Email: user.Email, DisplayName: "Google Parent", EmailAuthoritative: true}
	nonce := integrationDigest("google-nonce")
	if err := repo.CreateGoogleChallenge(ctx, nonce, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	linked, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("google", now), now)
	if err != nil || linked.ID != user.ID {
		t.Fatalf("Google linking = %+v %v", linked, err)
	}
	if _, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("google-replay", now), now); !errors.Is(err, ErrNotFound) {
		t.Fatal("replayed Google nonce")
	}
	// Returning Google identity is resolved by subject even when email changes.
	google.Email = "changed@gmail.com"
	nonce = integrationDigest("google-return")
	if err := repo.CreateGoogleChallenge(ctx, nonce, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	returning, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("google-return", now), now)
	if err != nil || returning.ID != user.ID {
		t.Fatalf("Google subject not stable: %v", err)
	}
	// Untrusted email providers cannot implicitly link an existing account.
	google.Subject, google.Email, google.EmailAuthoritative = "untrusted", "second@example.test", false
	nonce = integrationDigest("untrusted")
	if err := repo.CreateGoogleChallenge(ctx, nonce, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("untrusted", now), now); !errors.Is(err, ErrConflict) {
		t.Fatalf("untrusted email link: %v", err)
	}
	google.Email = "new-untrusted@example.test"
	if _, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("untrusted-new", now), now); !errors.Is(err, ErrConflict) {
		t.Fatalf("untrusted email signup: %v", err)
	}
	// Google creation is also atomic with session insertion and nonce consumption.
	google.Subject, google.Email, google.EmailAuthoritative = "new-google", "new@gmail.com", true
	nonce = integrationDigest("new-google")
	if err := repo.CreateGoogleChallenge(ctx, nonce, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.ExchangeGoogle(ctx, google, nonce, session, now); err == nil {
		t.Fatal("Google accepted duplicate session")
	}
	if _, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("new-google", now), now); err != nil {
		t.Fatalf("Google did not roll back: %v", err)
	}
	// Suspension takes effect for both primary sign-in methods.
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'suspended' WHERE id = $1::uuid`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.PasswordIdentity(ctx, "parent"); !errors.Is(err, ErrNotFound) {
		t.Fatal("suspended password user accepted")
	}
	nonce = integrationDigest("suspended")
	if err := repo.CreateGoogleChallenge(ctx, nonce, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	google.Subject = "google-subject"
	if _, _, err := repo.ExchangeGoogle(ctx, google, nonce, postgresSession("suspended", now), now); !errors.Is(err, ErrNotFound) {
		t.Fatal("suspended Google user accepted")
	}
}
