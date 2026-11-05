package repository

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var (
	ErrNotFound       = errors.New("auth repository: not found")
	ErrConsumed       = errors.New("auth repository: token consumed")
	ErrConflict       = errors.New("auth repository: conflict")
	ErrInvalidSession = errors.New("auth repository: invalid session")
)

type Repository interface {
	RememberSession(context.Context, string, model.Device, []byte, time.Time) (model.Device, error)
	DeviceByID(context.Context, string) (model.Device, model.User, error)
	DeviceByTokenHash(context.Context, []byte) (model.Device, model.User, error)
	RestoreDeviceSession(context.Context, model.Device, model.Session, time.Time) error
	RevokeDevice(context.Context, string, string, time.Time) error
	CreateMagicLink(context.Context, model.MagicLink) error
	ExchangeMagicLink(context.Context, []byte, model.Session, time.Time) (model.MagicLink, model.User, model.Session, error)
	ExchangeStudentCode(context.Context, []byte, model.Session, time.Time) (model.User, model.Session, error)
	SessionByTokenHash(context.Context, []byte) (model.Session, model.User, error)
	ListActiveSessions(context.Context, string, time.Time) ([]model.Session, error)
	TouchSession(context.Context, string, time.Time, time.Time) error
	RotateSessionCSRF(context.Context, string, []byte, time.Time) error
	ReplaceSession(context.Context, string, model.Session, time.Time) error
	RevokeUserSession(context.Context, string, string, time.Time) error
	RevokeOtherSessions(context.Context, string, string, time.Time) error
	RevokeAllSessions(context.Context, string, time.Time) error
	CreateStudentCode(context.Context, model.StudentCode) error
	StartTOTPEnrollment(context.Context, model.MFAMethod, time.Time) error
	ActiveTOTPMethod(context.Context, string) (model.MFAMethod, error)
	ConfirmTOTPEnrollment(context.Context, MFAEnrollmentConfirmation) (model.Session, error)
	CompleteTOTPChallenge(context.Context, MFAChallengeCompletion) (model.Session, error)
	CompleteRecoveryChallenge(context.Context, MFARecoveryCompletion) (model.Session, error)
	ResetMFA(context.Context, MFAReset) error
}

type MFAEnrollmentConfirmation struct {
	MethodID       string
	UserID         string
	CurrentSession string
	TOTPStep       int64
	RecoveryCodes  []model.MFARecoveryCode
	Replacement    model.Session
	Audit          model.AuditEvent
	At             time.Time
}

type MFAChallengeCompletion struct {
	MethodID       string
	UserID         string
	CurrentSession string
	TOTPStep       int64
	Replacement    model.Session
	Audit          model.AuditEvent
	At             time.Time
}

type MFARecoveryCompletion struct {
	MethodID         string
	UserID           string
	CurrentSession   string
	RecoveryCodeHash []byte
	Replacement      model.Session
	Audit            model.AuditEvent
	At               time.Time
}

type MFAReset struct {
	ActorID      string
	TargetUserID string
	Audit        model.AuditEvent
	At           time.Time
}

type Memory struct {
	mu            sync.Mutex
	magicLinks    map[string]model.MagicLink
	users         map[string]model.User
	usersByEmail  map[string]string
	devices       map[string]model.Device
	sessions      map[string]model.Session
	studentCodes  map[string]model.StudentCode
	mfaMethods    map[string]model.MFAMethod
	mfaByUser     map[string]string
	recoveryCodes map[string]model.MFARecoveryCode
	auditEvents   []model.AuditEvent
}

func NewMemory() *Memory {
	return &Memory{
		magicLinks:    make(map[string]model.MagicLink),
		users:         make(map[string]model.User),
		usersByEmail:  make(map[string]string),
		devices:       make(map[string]model.Device),
		sessions:      make(map[string]model.Session),
		studentCodes:  make(map[string]model.StudentCode),
		mfaMethods:    make(map[string]model.MFAMethod),
		mfaByUser:     make(map[string]string),
		recoveryCodes: make(map[string]model.MFARecoveryCode),
	}
}

func (m *Memory) CreateMagicLink(_ context.Context, link model.MagicLink) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.magicLinks[string(link.TokenHash)] = link
	return nil
}

func (m *Memory) ExchangeMagicLink(
	_ context.Context,
	hash []byte,
	session model.Session,
	now time.Time,
) (model.MagicLink, model.User, model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := string(hash)
	link, ok := m.magicLinks[key]
	if !ok || !now.Before(link.ExpiresAt) {
		return model.MagicLink{}, model.User{}, model.Session{}, ErrNotFound
	}
	if link.ConsumedAt != nil {
		return model.MagicLink{}, model.User{}, model.Session{}, ErrConsumed
	}
	if !validNewSession(session) {
		return model.MagicLink{}, model.User{}, model.Session{}, ErrInvalidSession
	}
	if _, exists := m.sessions[string(session.TokenHash)]; exists {
		return model.MagicLink{}, model.User{}, model.Session{}, ErrConflict
	}

	user, exists := m.userByEmail(link.Email)
	if exists && user.Status != "active" {
		return model.MagicLink{}, model.User{}, model.Session{}, ErrNotFound
	}
	if !exists {
		user = model.User{
			ID:          platformid.New(),
			Email:       link.Email,
			DisplayName: link.Email,
			Roles:       []model.Role{model.RoleGuardian},
			Status:      "active",
			CreatedAt:   now,
		}
	}

	session.UserID = user.ID
	consumedAt := now
	link.ConsumedAt = &consumedAt
	m.magicLinks[key] = link
	if !exists {
		m.users[user.ID] = user
		m.usersByEmail[user.Email] = user.ID
	}
	m.sessions[string(session.TokenHash)] = session
	return link, user, session, nil
}

func (m *Memory) SessionByTokenHash(_ context.Context, hash []byte) (model.Session, model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[string(hash)]
	if !ok {
		return model.Session{}, model.User{}, ErrNotFound
	}
	user, ok := m.users[session.UserID]
	if !ok {
		return model.Session{}, model.User{}, ErrNotFound
	}
	return session, user, nil
}

func (m *Memory) ListActiveSessions(_ context.Context, userID string, now time.Time) ([]model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sessions := make([]model.Session, 0)
	for _, session := range m.sessions {
		device, remembered := m.devices[session.DeviceID]
		if session.UserID == userID && (session.IsActive(now) || (remembered && device.RevokedAt == nil)) {
			if remembered && device.RevokedAt != nil {
				continue
			}
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].LastSeenAt.Equal(sessions[j].LastSeenAt) {
			return sessions[i].ID > sessions[j].ID
		}
		return sessions[i].LastSeenAt.After(sessions[j].LastSeenAt)
	})
	seen := make(map[string]bool)
	unique := make([]model.Session, 0, len(sessions))
	for _, session := range sessions {
		if session.DeviceID != "" {
			if seen[session.DeviceID] {
				continue
			}
			seen[session.DeviceID] = true
		}
		unique = append(unique, session)
	}
	return unique, nil
}

func (m *Memory) TouchSession(_ context.Context, id string, seenAt, idleExpiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.sessions {
		if session.ID != id || session.RevokedAt != nil {
			continue
		}
		session.LastSeenAt = seenAt
		session.IdleExpiresAt = minTime(idleExpiresAt, session.AbsoluteExpiresAt)
		m.sessions[key] = session
		return nil
	}
	return ErrNotFound
}

func (m *Memory) RotateSessionCSRF(_ context.Context, id string, hash []byte, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.sessions {
		if session.ID != id || session.RevokedAt != nil {
			continue
		}
		session.CSRFHash = append([]byte(nil), hash...)
		session.CSRFExpiresAt = minTime(expiresAt, session.AbsoluteExpiresAt)
		m.sessions[key] = session
		return nil
	}
	return ErrNotFound
}

func (m *Memory) ReplaceSession(_ context.Context, oldID string, replacement model.Session, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !validNewSession(replacement) {
		return ErrInvalidSession
	}
	if _, exists := m.sessions[string(replacement.TokenHash)]; exists {
		return ErrConflict
	}

	var oldKey string
	var oldSession model.Session
	for key, session := range m.sessions {
		if session.ID != oldID || !session.IsActive(now) {
			continue
		}
		oldKey = key
		oldSession = session
		break
	}
	if oldKey == "" {
		return ErrNotFound
	}
	if replacement.UserID != oldSession.UserID {
		return ErrInvalidSession
	}
	replacement.AbsoluteExpiresAt = minTime(replacement.AbsoluteExpiresAt, oldSession.AbsoluteExpiresAt)
	replacement.IdleExpiresAt = minTime(replacement.IdleExpiresAt, replacement.AbsoluteExpiresAt)
	replacement.CSRFExpiresAt = minTime(replacement.CSRFExpiresAt, replacement.AbsoluteExpiresAt)
	if !validNewSession(replacement) {
		return ErrInvalidSession
	}

	revokedAt := now
	oldSession.RevokedAt = &revokedAt
	m.sessions[oldKey] = oldSession
	m.sessions[string(replacement.TokenHash)] = replacement
	return nil
}

func (m *Memory) RevokeUserSession(_ context.Context, userID, sessionID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.sessions {
		if session.ID != sessionID || session.UserID != userID {
			continue
		}
		if session.DeviceID != "" {
			m.revokeDeviceLocked(userID, session.DeviceID, now)
		}
		session.RevokedAt = &now
		m.sessions[key] = session
	}
	return nil
}

func (m *Memory) RevokeOtherSessions(_ context.Context, userID, currentSessionID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	currentDevice := ""
	for _, session := range m.sessions {
		if session.ID == currentSessionID && session.UserID == userID {
			currentDevice = session.DeviceID
		}
	}
	for id, device := range m.devices {
		if device.UserID == userID && id != currentDevice {
			m.revokeDeviceLocked(userID, id, now)
		}
	}
	for key, session := range m.sessions {
		if session.UserID == userID && session.ID != currentSessionID && (currentDevice == "" || session.DeviceID != currentDevice) {
			session.RevokedAt = &now
			m.sessions[key] = session
		}
	}
	return nil
}

func (m *Memory) RevokeAllSessions(_ context.Context, userID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, device := range m.devices {
		if device.UserID == userID {
			m.revokeDeviceLocked(userID, id, now)
		}
	}
	for key, session := range m.sessions {
		if session.UserID == userID {
			session.RevokedAt = &now
			m.sessions[key] = session
		}
	}
	return nil
}

func (m *Memory) CreateStudentCode(_ context.Context, code model.StudentCode) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.studentCodes[string(code.CodeHash)]; exists {
		return ErrConflict
	}
	m.studentCodes[string(code.CodeHash)] = code
	return nil
}

func (m *Memory) ExchangeStudentCode(
	_ context.Context,
	hash []byte,
	session model.Session,
	now time.Time,
) (model.User, model.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := string(hash)
	code, ok := m.studentCodes[key]
	if !ok || !now.Before(code.ExpiresAt) {
		return model.User{}, model.Session{}, ErrNotFound
	}
	if code.ConsumedAt != nil {
		return model.User{}, model.Session{}, ErrConsumed
	}
	user, ok := m.users[code.StudentUserID]
	if !ok || user.Status != "active" {
		return model.User{}, model.Session{}, ErrNotFound
	}
	if !validNewSession(session) {
		return model.User{}, model.Session{}, ErrInvalidSession
	}
	if _, exists := m.sessions[string(session.TokenHash)]; exists {
		return model.User{}, model.Session{}, ErrConflict
	}

	session.UserID = user.ID
	consumedAt := now
	code.ConsumedAt = &consumedAt
	m.studentCodes[key] = code
	m.sessions[string(session.TokenHash)] = session
	return user, session, nil
}

func (m *Memory) PutUser(user model.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[user.ID] = user
	if user.Email != "" {
		m.usersByEmail[user.Email] = user.ID
	}
}

func (m *Memory) userByEmail(email string) (model.User, bool) {
	id, ok := m.usersByEmail[email]
	if !ok {
		return model.User{}, false
	}
	user, ok := m.users[id]
	return user, ok
}

func validNewSession(session model.Session) bool {
	return session.ID != "" &&
		len(session.TokenHash) == sha256DigestSize &&
		len(session.CSRFHash) == sha256DigestSize &&
		len(session.UserAgentHash) == sha256DigestSize &&
		len(session.IPAddressHash) == sha256DigestSize &&
		!session.CreatedAt.IsZero() &&
		!session.LastSeenAt.IsZero() &&
		session.RevokedAt == nil &&
		!session.LastSeenAt.Before(session.CreatedAt) &&
		session.CreatedAt.Before(session.IdleExpiresAt) &&
		session.CreatedAt.Before(session.CSRFExpiresAt) &&
		session.CreatedAt.Before(session.AbsoluteExpiresAt) &&
		!session.IdleExpiresAt.After(session.AbsoluteExpiresAt) &&
		!session.CSRFExpiresAt.After(session.AbsoluteExpiresAt) &&
		((session.MFAVerifiedAt == nil && session.MFAMethod == "") ||
			(session.MFAVerifiedAt != nil && session.MFAMethod == model.MFAMethodTOTP))
}

func minTime(left, right time.Time) time.Time {
	if left.After(right) {
		return right
	}
	return left
}

const sha256DigestSize = 32
