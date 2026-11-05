package service

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
)

type captureEmail struct {
	link         string
	verification bool
}

func (c *captureEmail) SendPasswordVerification(_ context.Context, _, link string, _ time.Time) error {
	c.link, c.verification = link, true
	return nil
}

func TestMagicLinkRejectsDisplayNamesAndCommentsInIdentity(t *testing.T) {
	service := &Service{}
	for _, email := range []string{"Parent <parent@example.com>", "parent@example.com (Parent)", "parent@example.com, other@example.com"} {
		if err := service.RequestMagicLink(t.Context(), email, "/portal"); !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("RequestMagicLink(%q) = %v, want invalid input", email, err)
		}
	}
}

func (c *captureEmail) SendMagicLink(_ context.Context, _, link string, _ time.Time) error {
	c.link = link
	return nil
}

func TestMagicLinkIsOneTimeAndCreatesSession(t *testing.T) {
	repo := repository.NewMemory()
	email := &captureEmail{}
	svc, err := New(repo, email, Config{
		PublicAPIURL:       "https://api.example.com",
		PublicWebURL:       "https://www.example.com",
		MagicLinkTTL:       time.Minute,
		SessionIdleTTL:     time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := svc.RequestMagicLink(context.Background(), "Parent@example.com", "//evil.example"); err != nil {
		t.Fatalf("RequestMagicLink() error = %v", err)
	}
	const marker = "token="
	idx := len(email.link) - 1
	for idx >= 0 && email.link[idx] != '=' {
		idx--
	}
	if idx < 0 || email.link[idx-len(marker)+1:idx+1] != marker {
		t.Fatalf("magic link missing token: %s", email.link)
	}
	raw := email.link[idx+1:]
	result, redirect, err := svc.ConsumeMagicLink(context.Background(), raw, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("ConsumeMagicLink() error = %v", err)
	}
	if result.Token == "" || result.CSRFToken == "" {
		t.Fatal("ConsumeMagicLink() returned empty session credentials")
	}
	if redirect != "https://www.example.com/portal" {
		t.Fatalf("redirect = %q", redirect)
	}
	if _, _, err := svc.ConsumeMagicLink(context.Background(), raw, "ua", "127.0.0.1"); err == nil {
		t.Fatal("second ConsumeMagicLink() expected error")
	}
}

func TestSessionCSRFValidation(t *testing.T) {
	start := time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC)
	svc, result := authenticatedSession(t, start, Config{
		MagicLinkTTL:       time.Minute,
		SessionIdleTTL:     4 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
	})
	if err := svc.ValidateCSRF(context.Background(), result.Token, result.CSRFToken); err != nil {
		t.Fatalf("ValidateCSRF() error = %v", err)
	}
	svc.now = func() time.Time { return start.Add(time.Hour) }
	if err := svc.ValidateCSRF(context.Background(), result.Token, result.CSRFToken); err != nil {
		t.Fatalf("CSRF should last for its session: %v", err)
	}
	svc.now = func() time.Time { return start.Add(24 * time.Hour) }
	if err := svc.ValidateCSRF(context.Background(), result.Token, result.CSRFToken); err == nil {
		t.Fatal("ValidateCSRF() accepted an expired CSRF token")
	}
}

func TestRotateSessionPreservesAbsoluteExpiryAndRevokesOldToken(t *testing.T) {
	start := time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC)
	svc, original := authenticatedSession(t, start, Config{
		MagicLinkTTL:       time.Minute,
		SessionIdleTTL:     6 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
	})
	svc.now = func() time.Time { return start.Add(2 * time.Hour) }
	rotated, err := svc.RotateSession(context.Background(), original.Token, "new-agent", "192.0.2.1")
	if err != nil {
		t.Fatalf("RotateSession() error = %v", err)
	}
	if !rotated.Session.AbsoluteExpiresAt.Equal(original.Session.AbsoluteExpiresAt) {
		t.Fatalf("absolute expiry changed from %s to %s", original.Session.AbsoluteExpiresAt, rotated.Session.AbsoluteExpiresAt)
	}
	if _, err := svc.Authenticate(context.Background(), original.Token); err == nil {
		t.Fatal("Authenticate() accepted the rotated token")
	}
	if _, err := svc.Authenticate(context.Background(), rotated.Token); err != nil {
		t.Fatalf("Authenticate() rejected the replacement token: %v", err)
	}
	svc.now = func() time.Time { return original.Session.AbsoluteExpiresAt }
	if _, err := svc.Authenticate(context.Background(), rotated.Token); err == nil {
		t.Fatal("Authenticate() accepted a token at its absolute expiry")
	}
}

func TestListSessionsReturnsSafeDeviceSummariesAndMarksCurrent(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC)
	svc, _, email := newTestService(t, start, Config{
		MagicLinkTTL:       time.Minute,
		SessionIdleTTL:     6 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
	})
	first := signInWithMagicLink(t, svc, email, "parent@example.com", "first device")
	current := signInWithMagicLink(t, svc, email, "parent@example.com", "current device")
	principal, err := svc.Authenticate(context.Background(), current.Token)
	if err != nil {
		t.Fatalf("Authenticate(current) error = %v", err)
	}

	sessions, err := svc.ListSessions(context.Background(), principal)
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("ListSessions() count = %d, want 2", len(sessions))
	}
	byID := make(map[string]bool, len(sessions))
	for _, session := range sessions {
		if session.DeviceFingerprint == "" {
			t.Fatalf("session %q has an empty device fingerprint", session.ID)
		}
		byID[session.ID] = session.Current
	}
	if byID[first.Session.ID] {
		t.Fatal("first session is marked current")
	}
	if !byID[current.Session.ID] {
		t.Fatal("current session is not marked current")
	}
}

func TestSessionRevocationIsScopedAndSupportsOtherAndAllModes(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 27, 8, 0, 0, 0, time.UTC)
	svc, _, email := newTestService(t, start, Config{
		MagicLinkTTL:       time.Minute,
		SessionIdleTTL:     6 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
	})
	first := signInWithMagicLink(t, svc, email, "parent@example.com", "first device")
	current := signInWithMagicLink(t, svc, email, "parent@example.com", "current device")
	otherUser := signInWithMagicLink(t, svc, email, "other@example.com", "other user device")
	principal, err := svc.Authenticate(context.Background(), current.Token)
	if err != nil {
		t.Fatalf("Authenticate(current) error = %v", err)
	}

	revokedCurrent, err := svc.RevokeSession(context.Background(), principal, otherUser.Session.ID)
	if err != nil {
		t.Fatalf("RevokeSession(other user) error = %v", err)
	}
	if revokedCurrent {
		t.Fatal("RevokeSession(other user) reported the current session revoked")
	}
	if _, err := svc.Authenticate(context.Background(), otherUser.Token); err != nil {
		t.Fatalf("other user's session was revoked through an IDOR: %v", err)
	}

	if err := svc.RevokeOtherSessions(context.Background(), principal); err != nil {
		t.Fatalf("RevokeOtherSessions() error = %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), first.Token); err == nil {
		t.Fatal("Authenticate(first) accepted a session revoked by RevokeOtherSessions")
	}
	if _, err := svc.Authenticate(context.Background(), current.Token); err != nil {
		t.Fatalf("Authenticate(current) rejected the preserved session: %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), otherUser.Token); err != nil {
		t.Fatalf("RevokeOtherSessions crossed the user boundary: %v", err)
	}

	newCurrent := signInWithMagicLink(t, svc, email, "parent@example.com", "replacement device")
	newPrincipal, err := svc.Authenticate(context.Background(), newCurrent.Token)
	if err != nil {
		t.Fatalf("Authenticate(replacement) error = %v", err)
	}
	if err := svc.RevokeAllSessions(context.Background(), newPrincipal); err != nil {
		t.Fatalf("RevokeAllSessions() error = %v", err)
	}
	if _, err := svc.Authenticate(context.Background(), current.Token); err == nil {
		t.Fatal("Authenticate(current) accepted a session revoked by RevokeAllSessions")
	}
	if _, err := svc.Authenticate(context.Background(), newCurrent.Token); err == nil {
		t.Fatal("Authenticate(replacement) accepted a session revoked by RevokeAllSessions")
	}
	if _, err := svc.Authenticate(context.Background(), otherUser.Token); err != nil {
		t.Fatalf("RevokeAllSessions crossed the user boundary: %v", err)
	}
}

func authenticatedSession(t *testing.T, start time.Time, cfg Config) (*Service, SessionResult) {
	t.Helper()
	svc, _, email := newTestService(t, start, cfg)
	return svc, signInWithMagicLink(t, svc, email, "parent@example.com", "agent")
}

func newTestService(
	t *testing.T,
	start time.Time,
	cfg Config,
) (*Service, *repository.Memory, *captureEmail) {
	t.Helper()
	cfg.PublicAPIURL = "https://api.example.com"
	cfg.PublicWebURL = "https://www.example.com"
	repo := repository.NewMemory()
	email := &captureEmail{}
	svc, err := New(repo, email, cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	svc.now = func() time.Time { return start }
	return svc, repo, email
}

func signInWithMagicLink(
	t *testing.T,
	svc *Service,
	email *captureEmail,
	address string,
	userAgent string,
) SessionResult {
	t.Helper()
	if err := svc.RequestMagicLink(context.Background(), address, "/portal"); err != nil {
		t.Fatalf("RequestMagicLink() error = %v", err)
	}
	parsed, err := url.Parse(email.link)
	if err != nil {
		t.Fatalf("parsing magic link: %v", err)
	}
	raw := parsed.Query().Get("token")
	if raw == "" {
		t.Fatal("magic link has no token")
	}
	result, _, err := svc.ConsumeMagicLink(context.Background(), raw, userAgent, "192.0.2.1")
	if err != nil {
		t.Fatalf("ConsumeMagicLink() error = %v", err)
	}
	return result
}
