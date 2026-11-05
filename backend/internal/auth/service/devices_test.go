package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	"github.com/kingsgambit2700/platform/backend/internal/auth/totp"
)

func TestRememberedBrowserRestoresAfterInactivityAndRevokesDescendants(t *testing.T) {
	start := time.Date(2026, 9, 22, 10, 0, 0, 0, time.UTC)
	svc, session := authenticatedSession(t, start, Config{MagicLinkTTL: time.Minute, SessionIdleTTL: 30 * time.Minute, SessionAbsoluteTTL: 7 * 24 * time.Hour, CSRFTTL: 30 * time.Minute})
	raw, csrf, err := svc.RememberSession(t.Context(), session.Token)
	if err != nil || raw == "" {
		t.Fatalf("remember: %v", err)
	}
	for range 5 {
		next, err := svc.IssueCSRF(t.Context(), session.Token)
		if err != nil || next != csrf {
			t.Fatalf("session read invalidated CSRF: %v", err)
		}
	}
	svc.now = func() time.Time { return start.Add(30 * 24 * time.Hour) }
	if _, err := svc.Authenticate(t.Context(), session.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("old access session must expire: %v", err)
	}
	restored, err := svc.RestoreSession(t.Context(), raw, csrf, "browser", "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.CSRFToken != csrf {
		t.Fatal("restore invalidated another tab's CSRF")
	}
	principal, err := svc.Authenticate(t.Context(), restored.Token)
	if err != nil {
		t.Fatal(err)
	}
	devices, err := svc.ListSessions(t.Context(), principal)
	if err != nil || len(devices) != 1 || !devices[0].Remembered || !devices[0].Current {
		t.Fatalf("remembered device list: %+v, %v", devices, err)
	}
	if err := svc.ValidateCSRF(t.Context(), restored.Token, csrf); err != nil {
		t.Fatal(err)
	}
	if err := svc.Logout(t.Context(), session.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(t.Context(), restored.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("logout failed to revoke descendant: %v", err)
	}
	if _, err := svc.RestoreSession(t.Context(), raw, csrf, "browser", "127.0.0.1"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("logout resurrected: %v", err)
	}
}

func TestConcurrentBrowserBootstrapAndRestorationKeepCSRFValid(t *testing.T) {
	start := time.Now().UTC()
	svc, session := authenticatedSession(t, start, Config{MagicLinkTTL: time.Minute, SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour, CSRFTTL: time.Minute})
	var wg sync.WaitGroup
	type remembered struct {
		raw, csrf string
		err       error
	}
	outcomes := make(chan remembered, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			raw, csrf, err := svc.RememberSession(context.Background(), session.Token)
			outcomes <- remembered{raw, csrf, err}
		}()
	}
	wg.Wait()
	close(outcomes)
	raw, csrf := "", ""
	for outcome := range outcomes {
		if outcome.err != nil {
			t.Fatal(outcome.err)
		}
		if csrf != "" && csrf != outcome.csrf {
			t.Fatal("concurrent reads changed CSRF")
		}
		csrf = outcome.csrf
		if outcome.raw != "" {
			if raw != "" && raw != outcome.raw {
				t.Fatal("multiple device credentials")
			}
			raw = outcome.raw
		}
	}
	if raw == "" {
		t.Fatal("no remembered credential")
	}
	svc.now = func() time.Time { return start.Add(48 * time.Hour) }
	results := make(chan SessionResult, 12)
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.RestoreSession(context.Background(), raw, csrf, "browser", "ip")
			if err != nil {
				t.Error(err)
				return
			}
			results <- result
		}()
	}
	wg.Wait()
	close(results)
	for result := range results {
		if result.CSRFToken != csrf {
			t.Fatal("restoration changed CSRF")
		}
		if err := svc.ValidateCSRF(t.Context(), result.Token, csrf); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRememberedStaffPreservesMFAAndResetRevokesDevice(t *testing.T) {
	start := time.Now().UTC()
	svc, repo, email, staff := newMFAService(t, start)
	primary, _ := signInMFAUser(t, svc, email, staff.Email)
	if _, _, err := svc.RememberSession(t.Context(), primary.Token); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("remembered before MFA: %v", err)
	}
	enrollment, err := svc.StartTOTPEnrollment(t.Context(), primary.Token)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.Generate(enrollment.Secret, start)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := svc.ConfirmTOTPEnrollment(t.Context(), primary.Token, code, "browser", "ip", "test")
	if err != nil {
		t.Fatal(err)
	}
	raw, csrf, err := svc.RememberSession(t.Context(), verified.Token)
	if err != nil {
		t.Fatal(err)
	}
	svc.now = func() time.Time { return start.Add(30 * 24 * time.Hour) }
	restored, err := svc.RestoreSession(t.Context(), raw, csrf, "browser", "ip")
	if err != nil {
		t.Fatal(err)
	}
	principal, err := svc.Authenticate(t.Context(), restored.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.MFAVerifiedAt == nil || !principal.MFAVerifiedAt.Equal(start) {
		t.Fatal("restoration changed MFA assurance")
	}
	if !errors.Is(svc.RequireRecentMFA(principal), ErrRecentMFARequired) {
		t.Fatal("restoration bypassed sensitive-action MFA")
	}
	if err := repo.ResetMFA(t.Context(), repository.MFAReset{TargetUserID: staff.ID, At: svc.now()}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RestoreSession(t.Context(), raw, csrf, "browser", "ip"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("MFA reset resurrected device: %v", err)
	}
}

func TestRememberedRevocationAndCurrentAccountStatus(t *testing.T) {
	start := time.Now().UTC()
	for _, operation := range []string{"selected", "others", "all", "suspended"} {
		t.Run(operation, func(t *testing.T) {
			svc, repo, email := newTestService(t, start, Config{MagicLinkTTL: time.Minute, SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour, CSRFTTL: time.Minute})
			first := signInWithMagicLink(t, svc, email, "parent@example.com", "first")
			raw, csrf, err := svc.RememberSession(t.Context(), first.Token)
			if err != nil {
				t.Fatal(err)
			}
			current := signInWithMagicLink(t, svc, email, "parent@example.com", "current")
			currentRaw, currentCSRF, err := svc.RememberSession(t.Context(), current.Token)
			if err != nil {
				t.Fatal(err)
			}
			principal, err := svc.Authenticate(t.Context(), current.Token)
			if err != nil {
				t.Fatal(err)
			}
			switch operation {
			case "selected":
				_, err = svc.RevokeSession(t.Context(), principal, first.Session.ID)
			case "others":
				err = svc.RevokeOtherSessions(t.Context(), principal)
			case "all":
				err = svc.RevokeAllSessions(t.Context(), principal)
			case "suspended":
				user := first.User
				user.Status = "suspended"
				repo.PutUser(user)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.RestoreSession(t.Context(), raw, csrf, "first", "ip"); !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("restoration survived %s: %v", operation, err)
			}
			if operation == "others" || operation == "selected" {
				if _, err := svc.RestoreSession(t.Context(), currentRaw, currentCSRF, "current", "ip"); err != nil {
					t.Fatalf("current browser revoked: %v", err)
				}
			}
		})
	}
}

type unavailableSessions struct{ repository.Repository }

func (unavailableSessions) SessionByTokenHash(context.Context, []byte) (model.Session, model.User, error) {
	return model.Session{}, model.User{}, errors.New("database offline")
}
func TestAuthenticationOutageIsNotUnauthenticated(t *testing.T) {
	svc, session := authenticatedSession(t, time.Now(), Config{MagicLinkTTL: time.Minute, SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour, CSRFTTL: time.Minute})
	svc.repo = unavailableSessions{svc.repo}
	if _, err := svc.Authenticate(t.Context(), session.Token); err == nil || errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("outage became logout: %v", err)
	}
}

func TestPromotedRememberedUserCompletesMFAWithoutLosingRemembering(t *testing.T) {
	start := time.Now().UTC()
	svc, repo, email, staff := newMFAService(t, start)
	guardian := staff
	guardian.Roles = []model.Role{model.RoleGuardian}
	repo.PutUser(guardian)
	session, _ := signInMFAUser(t, svc, email, staff.Email)
	raw, csrf, err := svc.RememberSession(t.Context(), session.Token)
	if err != nil {
		t.Fatal(err)
	}
	repo.PutUser(staff)
	if _, err := svc.Authenticate(t.Context(), session.Token); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("promotion bypassed MFA: %v", err)
	}
	enrollment, err := svc.StartTOTPEnrollment(t.Context(), session.Token)
	if err != nil {
		t.Fatal(err)
	}
	code, err := totp.Generate(enrollment.Secret, start)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := svc.ConfirmTOTPEnrollment(t.Context(), session.Token, code, "browser", "ip", "promoted")
	if err != nil {
		t.Fatal(err)
	}
	if completed.CSRFToken != csrf {
		t.Fatal("MFA invalidated device CSRF")
	}
	svc.now = func() time.Time { return start.Add(30 * 24 * time.Hour) }
	restored, err := svc.RestoreSession(t.Context(), raw, csrf, "browser", "ip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(t.Context(), restored.Token); err != nil {
		t.Fatalf("completed staff MFA was not remembered: %v", err)
	}
}
