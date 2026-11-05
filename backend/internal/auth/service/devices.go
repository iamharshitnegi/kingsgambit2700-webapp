package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func credentialCSRF(key []byte, purpose string) string {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("kingsgambit2700:" + purpose))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// The verifier is never returned to clients. Deriving a separate, irreversible
// CSRF value lets every access session of one device share a stable token.
func deviceCSRF(device model.Device) string {
	return credentialCSRF(device.TokenHash, "device-csrf-v1")
}

func (s *Service) RememberSession(ctx context.Context, rawSession string) (string, string, error) {
	session, user, err := s.loadPrimarySession(ctx, rawSession)
	if err != nil {
		return "", "", err
	}
	if user.RequiresMFA() && session.MFAVerifiedAt == nil {
		return "", "", ErrMFARequired
	}
	if session.DeviceID != "" {
		device, _, err := s.repo.DeviceByID(ctx, session.DeviceID)
		if err != nil {
			return "", "", err
		}
		return "", deviceCSRF(device), nil
	}
	// Concurrent bootstraps of one legacy session obtain the same credential.
	raw := credentialCSRF([]byte(rawSession), "remembered-device-v1")
	now := s.now().UTC()
	device := model.Device{ID: platformid.New(), UserID: user.ID, TokenHash: digest(raw), CreatedAt: now, LastSeenAt: now}
	device, err = s.repo.RememberSession(ctx, session.ID, device, digest(deviceCSRF(device)), now)
	if err != nil {
		return "", "", fmt.Errorf("remembering browser: %w", err)
	}
	return raw, deviceCSRF(device), nil
}

func (s *Service) rememberedDevice(ctx context.Context, raw string) (model.Device, model.User, error) {
	if len(raw) != 43 {
		return model.Device{}, model.User{}, ErrUnauthenticated
	}
	device, user, err := s.repo.DeviceByTokenHash(ctx, digest(raw))
	if errors.Is(err, repository.ErrNotFound) || (err == nil && (device.RevokedAt != nil || user.Status != "active")) {
		return model.Device{}, model.User{}, ErrUnauthenticated
	}
	if err != nil {
		return model.Device{}, model.User{}, fmt.Errorf("reading remembered browser: %w", err)
	}
	return device, user, nil
}

func (s *Service) DeviceCSRF(ctx context.Context, raw string) (string, error) {
	device, _, err := s.rememberedDevice(ctx, raw)
	if err != nil {
		return "", err
	}
	return deviceCSRF(device), nil
}

func (s *Service) ValidateDeviceCSRF(ctx context.Context, raw, csrf string) error {
	expected, err := s.DeviceCSRF(ctx, raw)
	if err != nil {
		return err
	}
	if csrf == "" || subtle.ConstantTimeCompare([]byte(expected), []byte(csrf)) != 1 {
		return ErrInvalidCSRF
	}
	return nil
}

func (s *Service) RestoreSession(ctx context.Context, raw, csrf, userAgent, ip string) (SessionResult, error) {
	device, user, err := s.rememberedDevice(ctx, raw)
	if err != nil {
		return SessionResult{}, err
	}
	if subtle.ConstantTimeCompare([]byte(deviceCSRF(device)), []byte(csrf)) != 1 {
		return SessionResult{}, ErrInvalidCSRF
	}
	now := s.now().UTC()
	result, err := s.newSession(user.ID, userAgent, ip, now)
	if err != nil {
		return SessionResult{}, err
	}
	result.User = user
	result.Session.DeviceID = device.ID
	// Restoration retains the original MFA assurance, never freshens it.
	result.Session.MFAVerifiedAt, result.Session.MFAMethod = device.MFAVerifiedAt, device.MFAMethod
	result.CSRFToken = deviceCSRF(device)
	result.Session.CSRFHash = digest(result.CSRFToken)
	if err := s.repo.RestoreDeviceSession(ctx, device, result.Session, now); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return SessionResult{}, ErrUnauthenticated
		}
		return SessionResult{}, fmt.Errorf("restoring browser session: %w", err)
	}
	return result, nil
}

func (s *Service) LogoutDevice(ctx context.Context, raw string) error {
	device, _, err := s.rememberedDevice(ctx, raw)
	if errors.Is(err, ErrUnauthenticated) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.repo.RevokeDevice(ctx, device.UserID, device.ID, s.now().UTC())
}

func (s *Service) inheritDevice(ctx context.Context, result *SessionResult, previous model.Session) error {
	if previous.DeviceID == "" {
		return nil
	}
	device, _, err := s.repo.DeviceByID(ctx, previous.DeviceID)
	if err != nil {
		return err
	}
	if device.RevokedAt != nil {
		return ErrUnauthenticated
	}
	result.Session.DeviceID = previous.DeviceID
	result.CSRFToken = deviceCSRF(device)
	result.Session.CSRFHash = digest(result.CSRFToken)
	return nil
}
