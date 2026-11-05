package repository

import (
	"context"
	"crypto/sha256"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
)

func (m *Memory) StartTOTPEnrollment(_ context.Context, method model.MFAMethod, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if method.ID == "" || method.UserID == "" || len(method.EncryptedSecret) < 30 || method.VerifiedAt != nil || method.DisabledAt != nil {
		return ErrConflict
	}
	if existingID := m.mfaByUser[method.UserID]; existingID != "" {
		existing := m.mfaMethods[existingID]
		if existing.IsVerified() {
			return ErrConflict
		}
		disabledAt := now
		existing.DisabledAt = &disabledAt
		m.mfaMethods[existingID] = existing
	}
	method.EncryptedSecret = append([]byte(nil), method.EncryptedSecret...)
	m.mfaMethods[method.ID] = method
	m.mfaByUser[method.UserID] = method.ID
	return nil
}

func (m *Memory) ActiveTOTPMethod(_ context.Context, userID string) (model.MFAMethod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	methodID := m.mfaByUser[userID]
	method, ok := m.mfaMethods[methodID]
	if !ok || method.DisabledAt != nil {
		return model.MFAMethod{}, ErrNotFound
	}
	method.EncryptedSecret = append([]byte(nil), method.EncryptedSecret...)
	return method, nil
}

func (m *Memory) ConfirmTOTPEnrollment(
	_ context.Context,
	confirmation MFAEnrollmentConfirmation,
) (model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	method, ok := m.mfaMethods[confirmation.MethodID]
	if !ok || method.UserID != confirmation.UserID || method.DisabledAt != nil {
		return model.Session{}, ErrNotFound
	}
	if method.VerifiedAt != nil {
		return model.Session{}, ErrConflict
	}
	replacement, oldKey, oldSession, err := m.validateMFARotation(
		confirmation.UserID,
		confirmation.CurrentSession,
		confirmation.Replacement,
		confirmation.At,
		true,
	)
	if err != nil {
		return model.Session{}, err
	}
	if err := validateRecoveryCodes(confirmation.MethodID, confirmation.RecoveryCodes); err != nil {
		return model.Session{}, err
	}

	verifiedAt := confirmation.At
	method.VerifiedAt = &verifiedAt
	method.LastTOTPStep = ptr(confirmation.TOTPStep)
	m.mfaMethods[method.ID] = method
	for _, recoveryCode := range confirmation.RecoveryCodes {
		copyCode := recoveryCode
		copyCode.CodeHash = append([]byte(nil), recoveryCode.CodeHash...)
		m.recoveryCodes[recoveryKey(recoveryCode.MethodID, recoveryCode.CodeHash)] = copyCode
	}
	m.applyMFARotation(oldKey, oldSession, replacement, confirmation.At)
	m.appendAudit(confirmation.Audit)
	return replacement, nil
}

func (m *Memory) CompleteTOTPChallenge(
	_ context.Context,
	completion MFAChallengeCompletion,
) (model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	method, ok := m.mfaMethods[completion.MethodID]
	if !ok || method.UserID != completion.UserID || !method.IsVerified() {
		return model.Session{}, ErrNotFound
	}
	if method.LastTOTPStep != nil && completion.TOTPStep <= *method.LastTOTPStep {
		return model.Session{}, ErrConsumed
	}
	replacement, oldKey, oldSession, err := m.validateMFARotation(
		completion.UserID,
		completion.CurrentSession,
		completion.Replacement,
		completion.At,
		false,
	)
	if err != nil {
		return model.Session{}, err
	}
	method.LastTOTPStep = ptr(completion.TOTPStep)
	m.mfaMethods[method.ID] = method
	m.applyMFARotation(oldKey, oldSession, replacement, completion.At)
	m.appendAudit(completion.Audit)
	return replacement, nil
}

func (m *Memory) CompleteRecoveryChallenge(
	_ context.Context,
	completion MFARecoveryCompletion,
) (model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	method, ok := m.mfaMethods[completion.MethodID]
	if !ok || method.UserID != completion.UserID || !method.IsVerified() {
		return model.Session{}, ErrNotFound
	}
	key := recoveryKey(completion.MethodID, completion.RecoveryCodeHash)
	recoveryCode, ok := m.recoveryCodes[key]
	if !ok || recoveryCode.UsedAt != nil {
		return model.Session{}, ErrConsumed
	}
	replacement, oldKey, oldSession, err := m.validateMFARotation(
		completion.UserID,
		completion.CurrentSession,
		completion.Replacement,
		completion.At,
		false,
	)
	if err != nil {
		return model.Session{}, err
	}
	usedAt := completion.At
	recoveryCode.UsedAt = &usedAt
	m.recoveryCodes[key] = recoveryCode
	m.applyMFARotation(oldKey, oldSession, replacement, completion.At)
	m.appendAudit(completion.Audit)
	return replacement, nil
}

func (m *Memory) ResetMFA(_ context.Context, reset MFAReset) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	methodID := m.mfaByUser[reset.TargetUserID]
	method, ok := m.mfaMethods[methodID]
	if !ok || method.DisabledAt != nil {
		return ErrNotFound
	}
	disabledAt := reset.At
	method.DisabledAt = &disabledAt
	m.mfaMethods[method.ID] = method
	delete(m.mfaByUser, reset.TargetUserID)
	for key, session := range m.sessions {
		if session.UserID != reset.TargetUserID || session.RevokedAt != nil {
			continue
		}
		revokedAt := reset.At
		session.RevokedAt = &revokedAt
		m.sessions[key] = session
	}
	for id, device := range m.devices {
		if device.UserID == reset.TargetUserID {
			m.revokeDeviceLocked(reset.TargetUserID, id, reset.At)
		}
	}
	m.appendAudit(reset.Audit)
	return nil
}

func (m *Memory) AuditEvents() []model.AuditEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]model.AuditEvent, len(m.auditEvents))
	copy(result, m.auditEvents)
	return result
}

func (m *Memory) validateMFARotation(
	userID string,
	currentSessionID string,
	replacement model.Session,
	now time.Time,
	requireUnverified bool,
) (model.Session, string, model.Session, error) {
	if !validNewSession(replacement) || replacement.UserID != userID {
		return model.Session{}, "", model.Session{}, ErrInvalidSession
	}
	if _, exists := m.sessions[string(replacement.TokenHash)]; exists {
		return model.Session{}, "", model.Session{}, ErrConflict
	}
	var oldKey string
	var oldSession model.Session
	for key, session := range m.sessions {
		if session.ID == currentSessionID && session.UserID == userID && session.IsActive(now) {
			oldKey = key
			oldSession = session
			break
		}
	}
	if oldKey == "" || (requireUnverified && oldSession.MFAVerifiedAt != nil) {
		return model.Session{}, "", model.Session{}, ErrInvalidSession
	}
	if replacement.DeviceID != "" {
		device, ok := m.devices[replacement.DeviceID]
		if !ok || device.RevokedAt != nil || device.UserID != userID {
			return model.Session{}, "", model.Session{}, ErrInvalidSession
		}
	}
	replacement.AbsoluteExpiresAt = minTime(replacement.AbsoluteExpiresAt, oldSession.AbsoluteExpiresAt)
	replacement.IdleExpiresAt = minTime(replacement.IdleExpiresAt, replacement.AbsoluteExpiresAt)
	replacement.CSRFExpiresAt = minTime(replacement.CSRFExpiresAt, replacement.AbsoluteExpiresAt)
	verifiedAt := now
	replacement.MFAVerifiedAt = &verifiedAt
	replacement.MFAMethod = model.MFAMethodTOTP
	if !validNewSession(replacement) {
		return model.Session{}, "", model.Session{}, ErrInvalidSession
	}
	return replacement, oldKey, oldSession, nil
}

func (m *Memory) applyMFARotation(
	oldKey string,
	oldSession model.Session,
	replacement model.Session,
	now time.Time,
) {
	revokedAt := now
	oldSession.RevokedAt = &revokedAt
	m.sessions[oldKey] = oldSession
	m.sessions[string(replacement.TokenHash)] = replacement
	if replacement.DeviceID != "" {
		device := m.devices[replacement.DeviceID]
		device.MFAVerifiedAt, device.MFAMethod = replacement.MFAVerifiedAt, replacement.MFAMethod
		m.devices[device.ID] = device
	}
}

func (m *Memory) appendAudit(event model.AuditEvent) {
	copyEvent := event
	copyEvent.Metadata = cloneMetadata(event.Metadata)
	m.auditEvents = append(m.auditEvents, copyEvent)
}

func validateRecoveryCodes(methodID string, codes []model.MFARecoveryCode) error {
	if len(codes) == 0 {
		return ErrConflict
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if code.ID == "" || code.MethodID != methodID || len(code.CodeHash) != sha256.Size || code.UsedAt != nil {
			return ErrConflict
		}
		key := string(code.CodeHash)
		if _, exists := seen[key]; exists {
			return ErrConflict
		}
		seen[key] = struct{}{}
	}
	return nil
}

func recoveryKey(methodID string, hash []byte) string {
	return methodID + ":" + string(hash)
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	clone := make(map[string]any, len(metadata))
	for key, value := range metadata {
		clone[key] = value
	}
	return clone
}

func ptr[T any](value T) *T { return &value }
