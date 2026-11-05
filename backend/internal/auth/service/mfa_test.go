package service

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	"github.com/kingsgambit2700/platform/backend/internal/auth/totp"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestService_StaffSessionRequiresMFAAndEnrollmentRotatesIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	start := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	svc, repo, email, staff := newMFAService(t, start)
	primary, redirect := signInMFAUser(t, svc, email, staff.Email)

	if redirect != "https://www.example.com/auth/complete?mfa=required&next=%2Fportal" {
		t.Fatalf("redirect = %q", redirect)
	}
	if _, err := svc.Authenticate(ctx, primary.Token); !errors.Is(err, ErrMFARequired) {
		t.Fatalf("Authenticate(primary) error = %v, want %v", err, ErrMFARequired)
	}
	if _, err := svc.AuthenticatePrimary(ctx, primary.Token); err != nil {
		t.Fatalf("AuthenticatePrimary() error = %v", err)
	}

	firstEnrollment, err := svc.StartTOTPEnrollment(ctx, primary.Token)
	if err != nil {
		t.Fatalf("StartTOTPEnrollment(first) error = %v", err)
	}
	secondEnrollment, err := svc.StartTOTPEnrollment(ctx, primary.Token)
	if err != nil {
		t.Fatalf("StartTOTPEnrollment(second) error = %v", err)
	}
	if firstEnrollment.Secret == secondEnrollment.Secret {
		t.Fatal("restarted enrollment reused its one-time plaintext secret")
	}
	method, err := repo.ActiveTOTPMethod(ctx, staff.ID)
	if err != nil {
		t.Fatalf("ActiveTOTPMethod() error = %v", err)
	}
	if strings.Contains(string(method.EncryptedSecret), secondEnrollment.Secret) {
		t.Fatal("repository stored the TOTP secret in plaintext")
	}
	oldCode, err := totp.Generate(firstEnrollment.Secret, start)
	if err != nil {
		t.Fatalf("totp.Generate(old) error = %v", err)
	}
	if _, err := svc.ConfirmTOTPEnrollment(ctx, primary.Token, oldCode, "agent", "192.0.2.10", "request-old"); !errors.Is(err, ErrInvalidMFA) {
		t.Fatalf("ConfirmTOTPEnrollment(old secret) error = %v, want %v", err, ErrInvalidMFA)
	}
	code, err := totp.Generate(secondEnrollment.Secret, start)
	if err != nil {
		t.Fatalf("totp.Generate() error = %v", err)
	}
	completed, err := svc.ConfirmTOTPEnrollment(ctx, primary.Token, code, "agent", "192.0.2.10", "request-confirm")
	if err != nil {
		t.Fatalf("ConfirmTOTPEnrollment() error = %v", err)
	}
	if len(completed.RecoveryCodes) != recoveryCodeCount {
		t.Fatalf("recovery code count = %d, want %d", len(completed.RecoveryCodes), recoveryCodeCount)
	}
	if _, err := svc.Authenticate(ctx, primary.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(old primary) error = %v, want revoked", err)
	}
	principal, err := svc.Authenticate(ctx, completed.Token)
	if err != nil {
		t.Fatalf("Authenticate(mfa session) error = %v", err)
	}
	if principal.MFAVerifiedAt == nil {
		t.Fatal("MFA-completed principal has no assurance timestamp")
	}
	if events := repo.AuditEvents(); len(events) != 1 || events[0].Action != "mfa.totp.enrolled" {
		t.Fatalf("audit events = %+v", events)
	}
}

func TestService_TOTPAndRecoveryChallengesRejectReplay(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	start := time.Date(2026, time.August, 27, 11, 0, 0, 0, time.UTC)
	svc, _, email, staff := newMFAService(t, start)
	primary, _ := signInMFAUser(t, svc, email, staff.Email)
	enrollment, err := svc.StartTOTPEnrollment(ctx, primary.Token)
	if err != nil {
		t.Fatalf("StartTOTPEnrollment() error = %v", err)
	}
	enrollmentCode, err := totp.Generate(enrollment.Secret, start)
	if err != nil {
		t.Fatalf("totp.Generate(enrollment) error = %v", err)
	}
	completed, err := svc.ConfirmTOTPEnrollment(ctx, primary.Token, enrollmentCode, "agent", "192.0.2.1", "enroll")
	if err != nil {
		t.Fatalf("ConfirmTOTPEnrollment() error = %v", err)
	}

	svc.now = func() time.Time { return start.Add(30 * time.Second) }
	totpPrimary, _ := signInMFAUser(t, svc, email, staff.Email)
	challengeCode, err := totp.Generate(enrollment.Secret, svc.now())
	if err != nil {
		t.Fatalf("totp.Generate(challenge) error = %v", err)
	}
	if _, err := svc.CompleteMFAChallenge(ctx, totpPrimary.Token, challengeCode, "", "agent", "192.0.2.2", "totp"); err != nil {
		t.Fatalf("CompleteMFAChallenge(totp) error = %v", err)
	}
	replayPrimary, _ := signInMFAUser(t, svc, email, staff.Email)
	if _, err := svc.CompleteMFAChallenge(ctx, replayPrimary.Token, challengeCode, "", "agent", "192.0.2.3", "totp-replay"); !errors.Is(err, ErrInvalidMFA) {
		t.Fatalf("CompleteMFAChallenge(totp replay) error = %v, want %v", err, ErrInvalidMFA)
	}

	recoveryPrimary, _ := signInMFAUser(t, svc, email, staff.Email)
	firstRecovery := completed.RecoveryCodes[0]
	if _, err := svc.CompleteMFAChallenge(ctx, recoveryPrimary.Token, "", firstRecovery, "agent", "192.0.2.4", "recovery"); err != nil {
		t.Fatalf("CompleteMFAChallenge(recovery) error = %v", err)
	}
	recoveryReplayPrimary, _ := signInMFAUser(t, svc, email, staff.Email)
	if _, err := svc.CompleteMFAChallenge(ctx, recoveryReplayPrimary.Token, "", firstRecovery, "agent", "192.0.2.5", "recovery-replay"); !errors.Is(err, ErrInvalidMFA) {
		t.Fatalf("CompleteMFAChallenge(recovery replay) error = %v, want %v", err, ErrInvalidMFA)
	}
}

func TestService_MFAChallengeRefreshesExpiredAssurance(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	start := time.Date(2026, time.August, 27, 11, 30, 0, 0, time.UTC)
	svc, _, email, staff := newMFAService(t, start)
	primary, _ := signInMFAUser(t, svc, email, staff.Email)
	enrollment, err := svc.StartTOTPEnrollment(ctx, primary.Token)
	if err != nil {
		t.Fatalf("StartTOTPEnrollment() error = %v", err)
	}
	enrollmentCode, err := totp.Generate(enrollment.Secret, start)
	if err != nil {
		t.Fatalf("totp.Generate(enrollment) error = %v", err)
	}
	completed, err := svc.ConfirmTOTPEnrollment(
		ctx,
		primary.Token,
		enrollmentCode,
		"agent",
		"192.0.2.1",
		"enroll",
	)
	if err != nil {
		t.Fatalf("ConfirmTOTPEnrollment() error = %v", err)
	}

	stepUpAt := start.Add(7 * time.Minute)
	svc.now = func() time.Time { return stepUpAt }
	stalePrincipal, err := svc.Authenticate(ctx, completed.Token)
	if err != nil {
		t.Fatalf("Authenticate(stale assurance) error = %v", err)
	}
	if err := svc.RequireRecentMFA(stalePrincipal); !errors.Is(err, ErrRecentMFARequired) {
		t.Fatalf("RequireRecentMFA(stale assurance) error = %v, want %v", err, ErrRecentMFARequired)
	}
	stepUpCode, err := totp.Generate(enrollment.Secret, stepUpAt)
	if err != nil {
		t.Fatalf("totp.Generate(step-up) error = %v", err)
	}
	refreshed, err := svc.CompleteMFAChallenge(
		ctx,
		completed.Token,
		stepUpCode,
		"",
		"agent",
		"192.0.2.1",
		"step-up",
	)
	if err != nil {
		t.Fatalf("CompleteMFAChallenge(step-up) error = %v", err)
	}
	if _, err := svc.Authenticate(ctx, completed.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(pre-step-up session) error = %v, want revoked", err)
	}
	principal, err := svc.Authenticate(ctx, refreshed.Token)
	if err != nil {
		t.Fatalf("Authenticate(refreshed session) error = %v", err)
	}
	if principal.MFAVerifiedAt == nil || !principal.MFAVerifiedAt.Equal(stepUpAt) {
		t.Fatalf("refreshed MFA assurance = %v, want %v", principal.MFAVerifiedAt, stepUpAt)
	}
	if err := svc.RequireRecentMFA(principal); err != nil {
		t.Fatalf("RequireRecentMFA(refreshed assurance) error = %v", err)
	}
}

func TestService_MFAChallengeExpiryAndResetGuards(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	authorizer := &recordingResetAuthorizer{}
	svc, repo, email, staff := newMFAService(t, start, WithMFAResetAuthorizer(authorizer))
	primary, _ := signInMFAUser(t, svc, email, staff.Email)
	svc.now = func() time.Time { return start.Add(5 * time.Minute) }
	if _, err := svc.MFAStatus(ctx, primary.Token); !errors.Is(err, ErrMFAChallengeExpired) {
		t.Fatalf("MFAStatus(expired) error = %v, want %v", err, ErrMFAChallengeExpired)
	}
	if _, err := svc.Authenticate(ctx, primary.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(expired challenge) = %v, want unauthenticated instead of another MFA redirect", err)
	}
	if err := svc.Logout(ctx, primary.Token); err != nil {
		t.Fatalf("Logout(expired mfa challenge) error = %v", err)
	}

	svc.now = func() time.Time { return start.Add(time.Minute) }
	fresh, _ := signInMFAUser(t, svc, email, staff.Email)
	enrollment, err := svc.StartTOTPEnrollment(ctx, fresh.Token)
	if err != nil {
		t.Fatalf("StartTOTPEnrollment() error = %v", err)
	}
	code, err := totp.Generate(enrollment.Secret, svc.now())
	if err != nil {
		t.Fatalf("totp.Generate() error = %v", err)
	}
	completed, err := svc.ConfirmTOTPEnrollment(ctx, fresh.Token, code, "agent", "192.0.2.1", "confirm")
	if err != nil {
		t.Fatalf("ConfirmTOTPEnrollment() error = %v", err)
	}
	principal, err := svc.Authenticate(ctx, completed.Token)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}

	tests := []struct {
		name      string
		prepare   func()
		reason    string
		wantError error
	}{
		{
			name: "reason is required",
			prepare: func() {
				svc.now = func() time.Time { return start.Add(time.Minute) }
			},
			reason:    "short",
			wantError: ErrInvalidInput,
		},
		{
			name: "recent mfa is required",
			prepare: func() {
				svc.now = func() time.Time { return start.Add(7 * time.Minute) }
			},
			reason:    "Verified support recovery request",
			wantError: ErrRecentMFARequired,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.prepare()
			err := svc.ResetMFA(ctx, principal, staff.ID, tt.reason, "request-reset")
			if !errors.Is(err, tt.wantError) {
				t.Fatalf("ResetMFA() error = %v, want %v", err, tt.wantError)
			}
		})
	}

	svc.now = func() time.Time { return start.Add(2 * time.Minute) }
	savedAuthorizer := svc.resetAuthorizer
	svc.resetAuthorizer = nil
	if err := svc.ResetMFA(ctx, principal, staff.ID, "Verified support recovery request", "request-denied"); !errors.Is(err, ErrMFAResetForbidden) {
		t.Fatalf("ResetMFA(without authorizer) error = %v, want %v", err, ErrMFAResetForbidden)
	}
	svc.resetAuthorizer = savedAuthorizer
	if err := svc.ResetMFA(ctx, principal, staff.ID, "Verified support recovery request", "request-reset"); err != nil {
		t.Fatalf("ResetMFA() error = %v", err)
	}
	if authorizer.target != staff.ID {
		t.Fatalf("authorizer target = %q, want %q", authorizer.target, staff.ID)
	}
	if _, err := svc.Authenticate(ctx, completed.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate(after reset) error = %v, want revoked", err)
	}
	events := repo.AuditEvents()
	last := events[len(events)-1]
	if last.Action != "mfa.reset" || last.Reason != "Verified support recovery request" || last.RequestID != "request-reset" {
		t.Fatalf("reset audit = %+v", last)
	}
}

type recordingResetAuthorizer struct {
	target string
}

func (a *recordingResetAuthorizer) AuthorizeMFAReset(
	_ context.Context,
	_ model.Principal,
	target string,
) error {
	a.target = target
	return nil
}

func newMFAService(
	t *testing.T,
	start time.Time,
	options ...Option,
) (*Service, *repository.Memory, *captureEmail, model.User) {
	t.Helper()
	repo := repository.NewMemory()
	staff := model.User{
		ID:          platformid.New(),
		Email:       "coach@example.com",
		DisplayName: "Academy Coach",
		Roles:       []model.Role{model.RoleCoach},
		Status:      "active",
		CreatedAt:   start,
	}
	repo.PutUser(staff)
	email := &captureEmail{}
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("m", 32)))
	svc, err := New(repo, email, Config{
		PublicAPIURL:       "https://api.example.com",
		PublicWebURL:       "https://www.example.com",
		MagicLinkTTL:       15 * time.Minute,
		SessionIdleTTL:     6 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
		TOTPEncryptionKey:  key,
		MFAChallengeTTL:    5 * time.Minute,
		MFARecentTTL:       5 * time.Minute,
	}, options...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	svc.now = func() time.Time { return start }
	return svc, repo, email, staff
}

func signInMFAUser(
	t *testing.T,
	svc *Service,
	email *captureEmail,
	address string,
) (SessionResult, string) {
	t.Helper()
	if err := svc.RequestMagicLink(context.Background(), address, "/portal"); err != nil {
		t.Fatalf("RequestMagicLink() error = %v", err)
	}
	raw := magicTokenFromLink(t, email.link)
	result, redirect, err := svc.ConsumeMagicLink(context.Background(), raw, "agent", "192.0.2.1")
	if err != nil {
		t.Fatalf("ConsumeMagicLink() error = %v", err)
	}
	return result, redirect
}

func magicTokenFromLink(t *testing.T, link string) string {
	t.Helper()
	marker := "token="
	index := strings.Index(link, marker)
	if index < 0 {
		t.Fatalf("magic link has no token: %s", link)
	}
	return link[index+len(marker):]
}
