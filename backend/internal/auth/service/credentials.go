package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/password"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	"google.golang.org/api/idtoken"
)

var ErrGoogleUnavailable = errors.New("auth: Google sign-in is not configured")
var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{2,31}$`)

type PasswordSignup struct {
	Email        string `json:"email"`
	Username     string `json:"username"`
	DisplayName  string `json:"displayName"`
	Password     string `json:"password"`
	RedirectPath string `json:"redirectPath"`
}

func (s *Service) credentials() (repository.CredentialsRepository, error) {
	repo, ok := s.repo.(repository.CredentialsRepository)
	if !ok {
		return nil, errors.New("auth: credential repository unavailable")
	}
	return repo, nil
}

func validEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	return err == nil && address.Address == email && len(email) <= 254
}

func (s *Service) RegisterPassword(ctx context.Context, input PasswordSignup) error {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.DisplayName = strings.TrimSpace(input.DisplayName)
	if !validEmail(input.Email) || !usernamePattern.MatchString(input.Username) ||
		input.DisplayName == "" || utf8.RuneCountInString(input.DisplayName) > 100 || !password.Valid(input.Password) {
		return ErrInvalidInput
	}
	repo, err := s.credentials()
	if err != nil {
		return err
	}
	hash, err := password.Hash(input.Password)
	if err != nil {
		return err
	}
	raw, tokenHash, err := token(32)
	if err != nil {
		return err
	}
	expires := s.now().UTC().Add(s.cfg.MagicLinkTTL)
	err = repo.CreatePasswordRegistration(ctx, model.PasswordRegistration{
		TokenHash: tokenHash, Email: input.Email, Username: input.Username,
		DisplayName: input.DisplayName, PasswordHash: hash,
		RedirectPath: safeRedirect(input.RedirectPath), ExpiresAt: expires,
	})
	if err != nil {
		return err
	}
	link := strings.TrimRight(s.cfg.PublicAPIURL, "/") + "/api/v1/auth/password/verify?token=" + url.QueryEscape(raw)
	return s.email.SendPasswordVerification(ctx, input.Email, link, expires)
}

func (s *Service) VerifyPasswordRegistration(ctx context.Context, raw, userAgent, ip string) (SessionResult, string, error) {
	if raw == "" {
		return SessionResult{}, "", ErrUnauthenticated
	}
	repo, err := s.credentials()
	if err != nil {
		return SessionResult{}, "", err
	}
	now := s.now().UTC()
	result, err := s.newSession("", userAgent, ip, now)
	if err != nil {
		return SessionResult{}, "", err
	}
	user, session, redirect, err := repo.CompletePasswordRegistration(ctx, digest(raw), result.Session, now)
	if err != nil {
		return SessionResult{}, "", err
	}
	result.User, result.Session = user, session
	return result, strings.TrimRight(s.cfg.PublicWebURL, "/") + safeRedirect(redirect), nil
}

func (s *Service) LoginPassword(ctx context.Context, login, value, userAgent, ip string) (SessionResult, error) {
	login = strings.ToLower(strings.TrimSpace(login))
	if login == "" || len(login) > 254 || len(value) > 512 {
		return SessionResult{}, ErrUnauthenticated
	}
	repo, err := s.credentials()
	if err != nil {
		return SessionResult{}, err
	}
	identity, lookupErr := repo.PasswordIdentity(ctx, login)
	if lookupErr != nil && !errors.Is(lookupErr, repository.ErrNotFound) {
		return SessionResult{}, lookupErr
	}
	hash := identity.PasswordHash
	if lookupErr != nil {
		hash = password.DummyHash
	}
	valid := password.Verify(hash, value)
	if !valid || lookupErr != nil {
		return SessionResult{}, ErrUnauthenticated
	}
	now := s.now().UTC()
	result, err := s.newSession(identity.UserID, userAgent, ip, now)
	if err != nil {
		return SessionResult{}, err
	}
	result.User, result.Session, err = repo.CreatePasswordSession(ctx, identity, result.Session, now)
	if errors.Is(err, repository.ErrNotFound) {
		return SessionResult{}, ErrUnauthenticated
	}
	return result, err
}

type GoogleConfiguration struct {
	Enabled  bool   `json:"enabled"`
	ClientID string `json:"clientId,omitempty"`
	Nonce    string `json:"nonce,omitempty"`
}

func (s *Service) GoogleConfiguration(ctx context.Context) (GoogleConfiguration, error) {
	if s.cfg.GoogleClientID == "" {
		return GoogleConfiguration{}, nil
	}
	repo, err := s.credentials()
	if err != nil {
		return GoogleConfiguration{}, err
	}
	raw, hash, err := token(32)
	if err != nil {
		return GoogleConfiguration{}, err
	}
	if err := repo.CreateGoogleChallenge(ctx, hash, s.now().UTC().Add(10*time.Minute)); err != nil {
		return GoogleConfiguration{}, err
	}
	return GoogleConfiguration{Enabled: true, ClientID: s.cfg.GoogleClientID, Nonce: raw}, nil
}

func (s *Service) LoginGoogle(ctx context.Context, credential, nonce, userAgent, ip string) (SessionResult, error) {
	if s.cfg.GoogleClientID == "" {
		return SessionResult{}, ErrGoogleUnavailable
	}
	if len(nonce) != 43 || credential == "" || len(credential) > 16384 {
		return SessionResult{}, ErrUnauthenticated
	}
	verifyCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	payload, err := s.validateGoogleToken(verifyCtx, credential, s.cfg.GoogleClientID)
	if err != nil {
		return SessionResult{}, ErrUnauthenticated
	}
	identity, err := googleIdentity(payload, nonce, s.cfg.GoogleClientID, s.now())
	if err != nil {
		return SessionResult{}, err
	}
	repo, err := s.credentials()
	if err != nil {
		return SessionResult{}, err
	}
	now := s.now().UTC()
	result, err := s.newSession("", userAgent, ip, now)
	if err != nil {
		return SessionResult{}, err
	}
	result.User, result.Session, err = repo.ExchangeGoogle(ctx, identity, digest(nonce), result.Session, now)
	if errors.Is(err, repository.ErrNotFound) {
		return SessionResult{}, ErrUnauthenticated
	}
	return result, err
}

func googleIdentity(payload *idtoken.Payload, nonce, clientID string, now time.Time) (model.GoogleIdentity, error) {
	if payload == nil || (payload.Issuer != "accounts.google.com" && payload.Issuer != "https://accounts.google.com") ||
		payload.Subject == "" || payload.Audience != clientID || payload.Expires <= now.Unix() || payload.IssuedAt > now.Add(time.Minute).Unix() {
		return model.GoogleIdentity{}, ErrUnauthenticated
	}
	tokenNonce, _ := payload.Claims["nonce"].(string)
	verified, _ := payload.Claims["email_verified"].(bool)
	email, _ := payload.Claims["email"].(string)
	email = strings.ToLower(strings.TrimSpace(email))
	if !verified || !validEmail(email) || nonce == "" || subtle.ConstantTimeCompare([]byte(tokenNonce), []byte(nonce)) != 1 {
		return model.GoogleIdentity{}, ErrUnauthenticated
	}
	name, _ := payload.Claims["name"].(string)
	if strings.TrimSpace(name) == "" || utf8.RuneCountInString(name) > 100 {
		name = email
	}
	domain, _ := payload.Claims["hd"].(string)
	return model.GoogleIdentity{Subject: payload.Subject, Email: email, DisplayName: name,
		EmailAuthoritative: strings.HasSuffix(email, "@gmail.com") || domain != ""}, nil
}
