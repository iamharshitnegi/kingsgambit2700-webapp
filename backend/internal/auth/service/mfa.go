package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	"github.com/kingsgambit2700/platform/backend/internal/auth/totp"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

const recoveryCodeCount = 10

var (
	ErrMFAUnavailable        = errors.New("auth: mfa unavailable")
	ErrMFAChallengeExpired   = errors.New("auth: mfa challenge expired")
	ErrMFAEnrollmentRequired = errors.New("auth: mfa enrollment required")
	ErrMFAAlreadyEnrolled    = errors.New("auth: mfa already enrolled")
	ErrInvalidMFA            = errors.New("auth: invalid mfa credential")
	ErrMFAResetForbidden     = errors.New("auth: mfa reset forbidden")
	ErrRecentMFARequired     = errors.New("auth: recent mfa required")
)

type MFAResetAuthorizer interface {
	AuthorizeMFAReset(context.Context, model.Principal, string) error
}

type MFAStatus struct {
	Required            bool       `json:"required"`
	Enrolled            bool       `json:"enrolled"`
	EnrollmentPending   bool       `json:"enrollmentPending"`
	VerifiedThisSession bool       `json:"verifiedThisSession"`
	ChallengeExpiresAt  *time.Time `json:"challengeExpiresAt,omitempty"`
	CSRFToken           string     `json:"csrfToken"`
}

type TOTPEnrollment struct {
	Secret     string    `json:"secret"`
	OTPAuthURI string    `json:"otpauthUri"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type MFACompletionResult struct {
	SessionResult
	RecoveryCodes []string
}

func (s *Service) MFAStatus(ctx context.Context, rawSession string) (MFAStatus, error) {
	session, user, err := s.loadPrimarySession(ctx, rawSession)
	if err != nil {
		return MFAStatus{}, err
	}
	status := MFAStatus{
		Required:            user.RequiresMFA(),
		VerifiedThisSession: session.MFAVerifiedAt != nil,
	}
	if user.RequiresMFA() && session.MFAVerifiedAt == nil {
		expiresAt := session.CreatedAt.Add(s.cfg.MFAChallengeTTL)
		if !s.now().UTC().Before(expiresAt) {
			return MFAStatus{}, ErrMFAChallengeExpired
		}
		status.ChallengeExpiresAt = &expiresAt
	}
	if user.RequiresMFA() {
		method, methodErr := s.repo.ActiveTOTPMethod(ctx, user.ID)
		switch {
		case methodErr == nil:
			status.Enrolled = method.IsVerified()
			status.EnrollmentPending = !method.IsVerified()
		case errors.Is(methodErr, repository.ErrNotFound):
		default:
			return MFAStatus{}, fmt.Errorf("reading mfa status: %w", methodErr)
		}
	}
	csrf, err := s.IssueCSRF(ctx, rawSession)
	if err != nil {
		return MFAStatus{}, err
	}
	status.CSRFToken = csrf
	return status, nil
}

// StartTOTPEnrollment creates a fresh pending method. Its plaintext secret is
// returned by this call only; subsequent reads expose enrollment state but not
// the secret. Repeating the call replaces an unverified pending method.
func (s *Service) StartTOTPEnrollment(ctx context.Context, rawSession string) (TOTPEnrollment, error) {
	session, user, err := s.mfaPrimarySession(ctx, rawSession)
	if err != nil {
		return TOTPEnrollment{}, err
	}
	if s.mfaCipher == nil {
		return TOTPEnrollment{}, ErrMFAUnavailable
	}
	method, err := s.repo.ActiveTOTPMethod(ctx, user.ID)
	if err == nil && method.IsVerified() {
		return TOTPEnrollment{}, ErrMFAAlreadyEnrolled
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return TOTPEnrollment{}, fmt.Errorf("checking current mfa method: %w", err)
	}

	secret, err := totp.NewSecret()
	if err != nil {
		return TOTPEnrollment{}, err
	}
	methodID := platformid.New()
	encrypted, err := s.mfaCipher.encrypt([]byte(secret), secretAAD(user.ID, methodID))
	if err != nil {
		return TOTPEnrollment{}, err
	}
	now := s.now().UTC()
	if err := s.repo.StartTOTPEnrollment(ctx, model.MFAMethod{
		ID:              methodID,
		UserID:          user.ID,
		EncryptedSecret: encrypted,
		CreatedAt:       now,
	}, now); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			return TOTPEnrollment{}, ErrMFAAlreadyEnrolled
		}
		return TOTPEnrollment{}, fmt.Errorf("starting totp enrollment: %w", err)
	}
	return TOTPEnrollment{
		Secret:     secret,
		OTPAuthURI: s.otpAuthURI(user, secret),
		ExpiresAt:  session.CreatedAt.Add(s.cfg.MFAChallengeTTL),
	}, nil
}

func (s *Service) ConfirmTOTPEnrollment(
	ctx context.Context,
	rawSession string,
	code string,
	userAgent string,
	ipAddress string,
	requestID string,
) (MFACompletionResult, error) {
	session, user, err := s.mfaPrimarySession(ctx, rawSession)
	if err != nil {
		return MFACompletionResult{}, err
	}
	if s.mfaCipher == nil {
		return MFACompletionResult{}, ErrMFAUnavailable
	}
	method, err := s.repo.ActiveTOTPMethod(ctx, user.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return MFACompletionResult{}, ErrMFAEnrollmentRequired
	}
	if err != nil {
		return MFACompletionResult{}, fmt.Errorf("loading pending mfa method: %w", err)
	}
	if method.IsVerified() {
		return MFACompletionResult{}, ErrMFAAlreadyEnrolled
	}
	secret, err := s.mfaCipher.decrypt(method.EncryptedSecret, secretAAD(user.ID, method.ID))
	if err != nil {
		return MFACompletionResult{}, err
	}
	defer clearBytes(secret)
	now := s.now().UTC()
	step, valid := totp.MatchCounter(string(secret), strings.TrimSpace(code), now)
	if !valid {
		return MFACompletionResult{}, ErrInvalidMFA
	}

	displayCodes := make([]string, 0, recoveryCodeCount)
	recoveryCodes := make([]model.MFARecoveryCode, 0, recoveryCodeCount)
	for range recoveryCodeCount {
		display, normalized, err := newRecoveryCode()
		if err != nil {
			return MFACompletionResult{}, err
		}
		displayCodes = append(displayCodes, display)
		recoveryCodes = append(recoveryCodes, model.MFARecoveryCode{
			ID:        platformid.New(),
			MethodID:  method.ID,
			CodeHash:  digest(normalized),
			CreatedAt: now,
		})
	}
	replacement, err := s.newSession(user.ID, userAgent, ipAddress, now)
	if err != nil {
		return MFACompletionResult{}, err
	}
	if err := s.inheritDevice(ctx, &replacement, session); err != nil {
		return MFACompletionResult{}, err
	}
	persisted, err := s.repo.ConfirmTOTPEnrollment(ctx, repository.MFAEnrollmentConfirmation{
		MethodID:       method.ID,
		UserID:         user.ID,
		CurrentSession: session.ID,
		TOTPStep:       step,
		RecoveryCodes:  recoveryCodes,
		Replacement:    replacement.Session,
		Audit:          mfaAudit(user.ID, "mfa.totp.enrolled", method.ID, "", requestID, now, nil),
		At:             now,
	})
	if err != nil {
		if errors.Is(err, repository.ErrConflict) || errors.Is(err, repository.ErrConsumed) {
			return MFACompletionResult{}, ErrInvalidMFA
		}
		return MFACompletionResult{}, fmt.Errorf("confirming totp enrollment: %w", err)
	}
	replacement.Session = persisted
	replacement.User = user
	return MFACompletionResult{SessionResult: replacement, RecoveryCodes: displayCodes}, nil
}

func (s *Service) CompleteMFAChallenge(
	ctx context.Context,
	rawSession string,
	code string,
	recoveryCode string,
	userAgent string,
	ipAddress string,
	requestID string,
) (SessionResult, error) {
	session, user, err := s.mfaChallengeSession(ctx, rawSession)
	if err != nil {
		return SessionResult{}, err
	}
	if s.mfaCipher == nil {
		return SessionResult{}, ErrMFAUnavailable
	}
	code = strings.TrimSpace(code)
	recoveryCode = normalizeRecoveryCode(recoveryCode)
	if (code == "") == (recoveryCode == "") {
		return SessionResult{}, ErrInvalidInput
	}
	method, err := s.repo.ActiveTOTPMethod(ctx, user.ID)
	if errors.Is(err, repository.ErrNotFound) || (err == nil && !method.IsVerified()) {
		return SessionResult{}, ErrMFAEnrollmentRequired
	}
	if err != nil {
		return SessionResult{}, fmt.Errorf("loading verified mfa method: %w", err)
	}

	now := s.now().UTC()
	replacement, err := s.newSession(user.ID, userAgent, ipAddress, now)
	if err != nil {
		return SessionResult{}, err
	}
	if err := s.inheritDevice(ctx, &replacement, session); err != nil {
		return SessionResult{}, err
	}
	if code != "" {
		secret, err := s.mfaCipher.decrypt(method.EncryptedSecret, secretAAD(user.ID, method.ID))
		if err != nil {
			return SessionResult{}, err
		}
		defer clearBytes(secret)
		step, valid := totp.MatchCounter(string(secret), code, now)
		if !valid {
			return SessionResult{}, ErrInvalidMFA
		}
		persisted, err := s.repo.CompleteTOTPChallenge(ctx, repository.MFAChallengeCompletion{
			MethodID:       method.ID,
			UserID:         user.ID,
			CurrentSession: session.ID,
			TOTPStep:       step,
			Replacement:    replacement.Session,
			Audit: mfaAudit(
				user.ID,
				"mfa.challenge.completed",
				method.ID,
				"",
				requestID,
				now,
				map[string]any{"factor": "totp"},
			),
			At: now,
		})
		if err != nil {
			return SessionResult{}, translateChallengeError(err)
		}
		replacement.Session = persisted
	} else {
		if len(recoveryCode) != 24 {
			return SessionResult{}, ErrInvalidMFA
		}
		persisted, err := s.repo.CompleteRecoveryChallenge(ctx, repository.MFARecoveryCompletion{
			MethodID:         method.ID,
			UserID:           user.ID,
			CurrentSession:   session.ID,
			RecoveryCodeHash: digest(recoveryCode),
			Replacement:      replacement.Session,
			Audit: mfaAudit(
				user.ID,
				"mfa.challenge.completed",
				method.ID,
				"",
				requestID,
				now,
				map[string]any{"factor": "recovery_code"},
			),
			At: now,
		})
		if err != nil {
			return SessionResult{}, translateChallengeError(err)
		}
		replacement.Session = persisted
	}
	replacement.User = user
	return replacement, nil
}

func (s *Service) RequireRecentMFA(principal model.Principal) error {
	if principal.User.ID == "" || principal.SessionID == "" {
		return ErrUnauthenticated
	}
	if principal.MFAVerifiedAt == nil {
		return ErrRecentMFARequired
	}
	now := s.now().UTC()
	if principal.MFAVerifiedAt.After(now) || !now.Before(principal.MFAVerifiedAt.Add(s.cfg.MFARecentTTL)) {
		return ErrRecentMFARequired
	}
	return nil
}

func (s *Service) ResetMFA(
	ctx context.Context,
	actor model.Principal,
	targetUserID string,
	reason string,
	requestID string,
) error {
	if err := s.RequireRecentMFA(actor); err != nil {
		return err
	}
	if s.resetAuthorizer == nil || !platformid.ValidReference(targetUserID) {
		return ErrMFAResetForbidden
	}
	reason = strings.TrimSpace(reason)
	length := utf8.RuneCountInString(reason)
	if length < 12 || length > 500 {
		return ErrInvalidInput
	}
	if err := s.resetAuthorizer.AuthorizeMFAReset(ctx, actor, targetUserID); err != nil {
		return ErrMFAResetForbidden
	}
	now := s.now().UTC()
	if err := s.repo.ResetMFA(ctx, repository.MFAReset{
		ActorID:      actor.User.ID,
		TargetUserID: targetUserID,
		Audit: mfaAudit(
			actor.User.ID,
			"mfa.reset",
			targetUserID,
			reason,
			requestID,
			now,
			map[string]any{"targetUserId": targetUserID},
		),
		At: now,
	}); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrMFAEnrollmentRequired
		}
		return fmt.Errorf("resetting mfa: %w", err)
	}
	return nil
}

func (s *Service) mfaPrimarySession(
	ctx context.Context,
	rawSession string,
) (model.Session, model.User, error) {
	session, user, err := s.loadPrimarySession(ctx, rawSession)
	if err != nil {
		return model.Session{}, model.User{}, err
	}
	if !user.RequiresMFA() {
		return model.Session{}, model.User{}, ErrMFAResetForbidden
	}
	if session.MFAVerifiedAt != nil {
		return model.Session{}, model.User{}, ErrMFAAlreadyEnrolled
	}
	if !s.now().UTC().Before(session.CreatedAt.Add(s.cfg.MFAChallengeTTL)) {
		return model.Session{}, model.User{}, ErrMFAChallengeExpired
	}
	return session, user, nil
}

// mfaChallengeSession accepts both a restricted first-factor session and an
// already MFA-verified session. The latter enables an authenticated staff
// member to refresh the recent-MFA assurance window before a sensitive action.
// Only the initial restricted challenge is subject to the short challenge TTL.
func (s *Service) mfaChallengeSession(
	ctx context.Context,
	rawSession string,
) (model.Session, model.User, error) {
	session, user, err := s.loadPrimarySession(ctx, rawSession)
	if err != nil {
		return model.Session{}, model.User{}, err
	}
	if !user.RequiresMFA() {
		return model.Session{}, model.User{}, ErrMFAResetForbidden
	}
	if session.MFAVerifiedAt == nil &&
		!s.now().UTC().Before(session.CreatedAt.Add(s.cfg.MFAChallengeTTL)) {
		return model.Session{}, model.User{}, ErrMFAChallengeExpired
	}
	return session, user, nil
}

func (s *Service) otpAuthURI(user model.User, secret string) string {
	account := strings.TrimSpace(user.Email)
	if account == "" {
		account = strings.TrimSpace(user.DisplayName)
	}
	query := make(url.Values)
	query.Set("secret", secret)
	query.Set("issuer", s.cfg.TOTPIssuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", "6")
	query.Set("period", "30")
	return (&url.URL{
		Scheme:   "otpauth",
		Host:     "totp",
		Path:     "/" + s.cfg.TOTPIssuer + ":" + account,
		RawQuery: query.Encode(),
	}).String()
}

func mfaAudit(
	actorID string,
	action string,
	objectID string,
	reason string,
	requestID string,
	now time.Time,
	metadata map[string]any,
) model.AuditEvent {
	return model.AuditEvent{
		ID:        platformid.New(),
		ActorID:   actorID,
		Action:    action,
		ObjectID:  objectID,
		Reason:    reason,
		RequestID: requestID,
		Metadata:  metadata,
		CreatedAt: now,
	}
}

func translateChallengeError(err error) error {
	switch {
	case errors.Is(err, repository.ErrConsumed), errors.Is(err, repository.ErrConflict):
		return ErrInvalidMFA
	case errors.Is(err, repository.ErrInvalidSession), errors.Is(err, repository.ErrNotFound):
		return ErrUnauthenticated
	default:
		return fmt.Errorf("completing mfa challenge: %w", err)
	}
}

func clearBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
