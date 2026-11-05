package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"net/url"
	"path"
	"strings"
	"time"

	"google.golang.org/api/idtoken"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var (
	ErrUnauthenticated = errors.New("auth: unauthenticated")
	ErrInvalidCSRF     = errors.New("auth: invalid csrf token")
	ErrInvalidInput    = errors.New("auth: invalid input")
	ErrMFARequired     = errors.New("auth: mfa required")
)

type EmailSender interface {
	SendMagicLink(context.Context, string, string, time.Time) error
	SendPasswordVerification(context.Context, string, string, time.Time) error
}

type Config struct {
	GoogleClientID     string
	PublicAPIURL       string
	PublicWebURL       string
	MagicLinkTTL       time.Duration
	SessionIdleTTL     time.Duration
	SessionAbsoluteTTL time.Duration
	CSRFTTL            time.Duration
	TOTPEncryptionKey  string
	MFAChallengeTTL    time.Duration
	MFARecentTTL       time.Duration
	TOTPIssuer         string
}

type Service struct {
	validateGoogleToken func(context.Context, string, string) (*idtoken.Payload, error)
	repo                repository.Repository
	email               EmailSender
	cfg                 Config
	now                 func() time.Time
	mfaCipher           *secretCipher
	resetAuthorizer     MFAResetAuthorizer
}

type SessionResult struct {
	Token     string
	CSRFToken string
	Session   model.Session
	User      model.User
}

type Option func(*Service) error

func WithMFAResetAuthorizer(authorizer MFAResetAuthorizer) Option {
	return func(service *Service) error {
		if authorizer == nil {
			return errors.New("auth: mfa reset authorizer is required")
		}
		service.resetAuthorizer = authorizer
		return nil
	}
}

func New(repo repository.Repository, email EmailSender, cfg Config, options ...Option) (*Service, error) {
	if repo == nil || email == nil {
		return nil, errors.New("auth: repository and email sender are required")
	}
	if cfg.MagicLinkTTL <= 0 || cfg.SessionIdleTTL <= 0 || cfg.SessionAbsoluteTTL <= 0 || cfg.CSRFTTL <= 0 {
		return nil, errors.New("auth: all lifetimes must be positive")
	}
	if cfg.MFAChallengeTTL == 0 {
		cfg.MFAChallengeTTL = 5 * time.Minute
	}
	if cfg.MFARecentTTL == 0 {
		cfg.MFARecentTTL = 5 * time.Minute
	}
	if cfg.MFAChallengeTTL < time.Minute || cfg.MFAChallengeTTL > 15*time.Minute ||
		cfg.MFARecentTTL < time.Minute || cfg.MFARecentTTL > 15*time.Minute {
		return nil, errors.New("auth: mfa lifetimes must be between one and fifteen minutes")
	}
	if strings.TrimSpace(cfg.TOTPIssuer) == "" {
		cfg.TOTPIssuer = "KingsGambit2700"
	}
	cipher, err := newSecretCipher(cfg.TOTPEncryptionKey)
	if err != nil {
		return nil, err
	}
	service := &Service{repo: repo, email: email, cfg: cfg, now: time.Now, mfaCipher: cipher, validateGoogleToken: idtoken.Validate}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("auth: nil service option")
		}
		if err := option(service); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (s *Service) RequestMagicLink(ctx context.Context, email, redirectPath string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if address, err := mail.ParseAddress(email); err != nil || !strings.EqualFold(address.Address, email) {
		return ErrInvalidInput
	}
	redirectPath = safeRedirect(redirectPath)
	raw, hash, err := token(32)
	if err != nil {
		return fmt.Errorf("generating magic link: %w", err)
	}
	now := s.now().UTC()
	link := model.MagicLink{
		ID:           platformid.New(),
		Email:        email,
		TokenHash:    hash,
		RedirectPath: redirectPath,
		ExpiresAt:    now.Add(s.cfg.MagicLinkTTL),
		CreatedAt:    now,
	}
	if err := s.repo.CreateMagicLink(ctx, link); err != nil {
		return fmt.Errorf("creating magic link: %w", err)
	}
	consumeURL := strings.TrimRight(s.cfg.PublicAPIURL, "/") + "/api/v1/auth/magic-links/consume?token=" + url.QueryEscape(raw)
	if err := s.email.SendMagicLink(ctx, email, consumeURL, link.ExpiresAt); err != nil {
		return fmt.Errorf("sending magic link: %w", err)
	}
	return nil
}

func (s *Service) ConsumeMagicLink(ctx context.Context, rawToken, userAgent, ipAddress string) (SessionResult, string, error) {
	if rawToken == "" {
		return SessionResult{}, "", ErrUnauthenticated
	}
	now := s.now().UTC()
	result, err := s.newSession("", userAgent, ipAddress, now)
	if err != nil {
		return SessionResult{}, "", err
	}
	link, user, session, err := s.repo.ExchangeMagicLink(ctx, digest(rawToken), result.Session, now)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrConsumed) {
			return SessionResult{}, "", ErrUnauthenticated
		}
		return SessionResult{}, "", fmt.Errorf("exchanging magic link: %w", err)
	}
	result.Session = session
	result.User = user
	redirect := safeRedirect(link.RedirectPath)
	if user.RequiresMFA() {
		redirect = "/auth/complete?mfa=required&next=" + url.QueryEscape(redirect)
	}
	return result, strings.TrimRight(s.cfg.PublicWebURL, "/") + redirect, nil
}

func (s *Service) ExchangeStudentCode(ctx context.Context, rawCode, userAgent, ipAddress string) (SessionResult, error) {
	if len(rawCode) < 12 {
		return SessionResult{}, ErrUnauthenticated
	}
	now := s.now().UTC()
	result, err := s.newSession("", userAgent, ipAddress, now)
	if err != nil {
		return SessionResult{}, err
	}
	user, session, err := s.repo.ExchangeStudentCode(ctx, digest(rawCode), result.Session, now)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) || errors.Is(err, repository.ErrConsumed) {
			return SessionResult{}, ErrUnauthenticated
		}
		return SessionResult{}, fmt.Errorf("exchanging student code: %w", err)
	}
	result.Session = session
	result.User = user
	return result, nil
}

func (s *Service) Authenticate(ctx context.Context, rawToken string) (model.Principal, error) {
	session, user, err := s.loadPrimarySession(ctx, rawToken)
	if err != nil {
		return model.Principal{}, err
	}
	if user.RequiresMFA() && session.MFAVerifiedAt == nil {
		// Session discovery must not send an expired primary sign-in back to MFA.
		if !s.now().UTC().Before(session.CreatedAt.Add(s.cfg.MFAChallengeTTL)) {
			return model.Principal{}, ErrUnauthenticated
		}
		return model.Principal{}, ErrMFARequired
	}
	return principalFor(session, user), nil
}

// AuthenticatePrimary validates the first factor while preserving the
// restricted state of a staff session that has not completed MFA. It is only
// suitable for MFA enrollment, MFA challenge, and explicit sign-out routes.
func (s *Service) AuthenticatePrimary(ctx context.Context, rawToken string) (model.Principal, error) {
	session, user, err := s.loadPrimarySession(ctx, rawToken)
	if err != nil {
		return model.Principal{}, err
	}
	return principalFor(session, user), nil
}

// IssueCSRF is stable for a session or remembered device. Reading session state
// must not invalidate forms in another tab.
func (s *Service) IssueCSRF(ctx context.Context, rawToken string) (string, error) {
	session, _, err := s.loadPrimarySession(ctx, rawToken)
	if err != nil {
		return "", err
	}
	csrf := credentialCSRF([]byte(rawToken), "session-csrf-v1")
	if session.DeviceID != "" {
		device, _, err := s.repo.DeviceByID(ctx, session.DeviceID)
		if err != nil {
			return "", fmt.Errorf("reading csrf device: %w", err)
		}
		csrf = deviceCSRF(device)
	}
	// Upgrade legacy random tokens once; all concurrent reads derive the same value.
	if subtle.ConstantTimeCompare(session.CSRFHash, digest(csrf)) != 1 || !session.CSRFExpiresAt.Equal(session.AbsoluteExpiresAt) {
		if err := s.repo.RotateSessionCSRF(ctx, session.ID, digest(csrf), session.AbsoluteExpiresAt); err != nil {
			return "", fmt.Errorf("upgrading csrf token: %w", err)
		}
	}
	return csrf, nil
}

func (s *Service) ValidateCSRF(ctx context.Context, rawSession, csrf string) error {
	if rawSession == "" || csrf == "" {
		return ErrInvalidCSRF
	}
	session, _, err := s.loadPrimarySession(ctx, rawSession)
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) {
			return ErrInvalidCSRF
		}
		return err
	}
	if !s.now().UTC().Before(session.CSRFExpiresAt) || subtle.ConstantTimeCompare(session.CSRFHash, digest(csrf)) != 1 {
		return ErrInvalidCSRF
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, rawSession string) error {
	session, user, err := s.repo.SessionByTokenHash(ctx, digest(rawSession))
	if errors.Is(err, repository.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading session for logout: %w", err)
	}
	if err := s.repo.RevokeUserSession(ctx, user.ID, session.ID, s.now().UTC()); err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}

func (s *Service) ListSessions(ctx context.Context, principal model.Principal) ([]model.SessionSummary, error) {
	if principal.User.ID == "" || principal.SessionID == "" {
		return nil, ErrUnauthenticated
	}
	sessions, err := s.repo.ListActiveSessions(ctx, principal.User.ID, s.now().UTC())
	if err != nil {
		return nil, fmt.Errorf("listing sessions: %w", err)
	}

	summaries := make([]model.SessionSummary, 0, len(sessions))
	for _, session := range sessions {
		summaries = append(summaries, model.SessionSummary{
			ID:                session.ID,
			CreatedAt:         session.CreatedAt,
			LastSeenAt:        session.LastSeenAt,
			IdleExpiresAt:     session.IdleExpiresAt,
			AbsoluteExpiresAt: session.AbsoluteExpiresAt,
			DeviceFingerprint: fingerprint(session.UserAgentHash),
			Current:           session.ID == principal.SessionID || (session.DeviceID != "" && session.DeviceID == principal.DeviceID),
			Remembered:        session.DeviceID != "",
		})
	}
	return summaries, nil
}

func (s *Service) RevokeSession(ctx context.Context, principal model.Principal, sessionID string) (bool, error) {
	if principal.User.ID == "" || principal.SessionID == "" {
		return false, ErrUnauthenticated
	}
	if !platformid.Valid(sessionID) {
		return false, ErrInvalidInput
	}
	revokedCurrent := sessionID == principal.SessionID
	if principal.DeviceID != "" {
		sessions, err := s.repo.ListActiveSessions(ctx, principal.User.ID, s.now().UTC())
		if err != nil {
			return false, err
		}
		for _, session := range sessions {
			if session.ID == sessionID && session.DeviceID == principal.DeviceID {
				revokedCurrent = true
			}
		}
	}
	if err := s.repo.RevokeUserSession(ctx, principal.User.ID, sessionID, s.now().UTC()); err != nil {
		return false, fmt.Errorf("revoking selected session: %w", err)
	}
	return revokedCurrent, nil
}

func (s *Service) RevokeOtherSessions(ctx context.Context, principal model.Principal) error {
	if principal.User.ID == "" || principal.SessionID == "" {
		return ErrUnauthenticated
	}
	if err := s.repo.RevokeOtherSessions(ctx, principal.User.ID, principal.SessionID, s.now().UTC()); err != nil {
		return fmt.Errorf("revoking other sessions: %w", err)
	}
	return nil
}

func (s *Service) RevokeAllSessions(ctx context.Context, principal model.Principal) error {
	if principal.User.ID == "" || principal.SessionID == "" {
		return ErrUnauthenticated
	}
	if err := s.repo.RevokeAllSessions(ctx, principal.User.ID, s.now().UTC()); err != nil {
		return fmt.Errorf("revoking all sessions: %w", err)
	}
	return nil
}

func (s *Service) RotateSession(ctx context.Context, rawSession, userAgent, ipAddress string) (SessionResult, error) {
	oldSession, user, err := s.loadPrimarySession(ctx, rawSession)
	now := s.now().UTC()
	if err != nil {
		return SessionResult{}, err
	}
	if user.RequiresMFA() && oldSession.MFAVerifiedAt == nil {
		return SessionResult{}, ErrMFARequired
	}
	raw, hash, err := token(32)
	if err != nil {
		return SessionResult{}, fmt.Errorf("rotating session token: %w", err)
	}
	csrfRaw := credentialCSRF([]byte(raw), "session-csrf-v1")
	if oldSession.DeviceID != "" {
		device, _, err := s.repo.DeviceByID(ctx, oldSession.DeviceID)
		if err != nil {
			return SessionResult{}, err
		}
		csrfRaw = deviceCSRF(device)
	}
	csrfHash := digest(csrfRaw)
	idleExpiresAt := boundedExpiry(now, s.cfg.SessionIdleTTL, oldSession.AbsoluteExpiresAt)
	replacement := model.Session{
		ID:                platformid.New(),
		UserID:            user.ID,
		TokenHash:         hash,
		CSRFHash:          csrfHash,
		CSRFExpiresAt:     oldSession.AbsoluteExpiresAt,
		DeviceID:          oldSession.DeviceID,
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     idleExpiresAt,
		AbsoluteExpiresAt: oldSession.AbsoluteExpiresAt,
		UserAgentHash:     digest(userAgent),
		IPAddressHash:     digest(ipAddress),
		MFAVerifiedAt:     oldSession.MFAVerifiedAt,
		MFAMethod:         oldSession.MFAMethod,
	}
	if err := s.repo.ReplaceSession(ctx, oldSession.ID, replacement, now); err != nil {
		return SessionResult{}, fmt.Errorf("replacing session: %w", err)
	}
	return SessionResult{Token: raw, CSRFToken: csrfRaw, Session: replacement, User: user}, nil
}

func (s *Service) loadPrimarySession(ctx context.Context, rawToken string) (model.Session, model.User, error) {
	if rawToken == "" {
		return model.Session{}, model.User{}, ErrUnauthenticated
	}
	session, user, err := s.repo.SessionByTokenHash(ctx, digest(rawToken))
	if errors.Is(err, repository.ErrNotFound) || (err == nil && user.Status != "active") {
		return model.Session{}, model.User{}, ErrUnauthenticated
	}
	if err != nil {
		return model.Session{}, model.User{}, fmt.Errorf("reading session: %w", err)
	}
	if session.DeviceID != "" {
		device, _, deviceErr := s.repo.DeviceByID(ctx, session.DeviceID)
		if errors.Is(deviceErr, repository.ErrNotFound) || (deviceErr == nil && (device.RevokedAt != nil || device.UserID != user.ID)) {
			return model.Session{}, model.User{}, ErrUnauthenticated
		}
		if deviceErr != nil {
			return model.Session{}, model.User{}, fmt.Errorf("checking remembered device: %w", deviceErr)
		}
	}
	now := s.now().UTC()
	if !session.IsActive(now) {
		return model.Session{}, model.User{}, ErrUnauthenticated
	}
	idleExpires := boundedExpiry(now, s.cfg.SessionIdleTTL, session.AbsoluteExpiresAt)
	if now.Sub(session.LastSeenAt) >= time.Minute {
		if err := s.repo.TouchSession(ctx, session.ID, now, idleExpires); err != nil {
			return model.Session{}, model.User{}, fmt.Errorf("touching session: %w", err)
		}
	}
	return session, user, nil
}

func principalFor(session model.Session, user model.User) model.Principal {
	return model.Principal{
		SessionID:     session.ID,
		DeviceID:      session.DeviceID,
		User:          user,
		MFAVerifiedAt: session.MFAVerifiedAt,
	}
}

func (s *Service) newSession(userID, userAgent, ipAddress string, now time.Time) (SessionResult, error) {
	raw, hash, err := token(32)
	if err != nil {
		return SessionResult{}, fmt.Errorf("generating session: %w", err)
	}
	csrfRaw := credentialCSRF([]byte(raw), "session-csrf-v1")
	csrfHash := digest(csrfRaw)
	absoluteExpiresAt := now.Add(s.cfg.SessionAbsoluteTTL)
	session := model.Session{
		ID:                platformid.New(),
		UserID:            userID,
		TokenHash:         hash,
		CSRFHash:          csrfHash,
		CSRFExpiresAt:     absoluteExpiresAt,
		CreatedAt:         now,
		LastSeenAt:        now,
		IdleExpiresAt:     boundedExpiry(now, s.cfg.SessionIdleTTL, absoluteExpiresAt),
		AbsoluteExpiresAt: absoluteExpiresAt,
		UserAgentHash:     digest(userAgent),
		IPAddressHash:     digest(ipAddress),
	}
	return SessionResult{Token: raw, CSRFToken: csrfRaw, Session: session}, nil
}

func token(size int) (string, []byte, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(value)
	return raw, digest(raw), nil
}

func digest(value string) []byte {
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}

func fingerprint(hash []byte) string {
	const fingerprintBytes = 9
	if len(hash) < fingerprintBytes {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(hash[:fingerprintBytes])
}

func boundedExpiry(now time.Time, ttl time.Duration, absolute time.Time) time.Time {
	expiresAt := now.Add(ttl)
	if expiresAt.After(absolute) {
		return absolute
	}
	return expiresAt
}

func safeRedirect(value string) string {
	if value == "" || strings.Contains(value, "\\") || hasControlCharacter(value) {
		return "/portal"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil ||
		parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/") ||
		strings.HasPrefix(parsed.Path, "//") || strings.Contains(parsed.Path, "\\") ||
		hasControlCharacter(parsed.Path) {
		return "/portal"
	}
	parsed.Path = path.Clean(parsed.Path)
	parsed.RawPath = ""
	return parsed.RequestURI()
}

func hasControlCharacter(value string) bool {
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return true
		}
	}
	return false
}
