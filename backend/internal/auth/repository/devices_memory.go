package repository

import (
	"context"
	"crypto/subtle"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
)

func (m *Memory) RememberSession(_ context.Context, sessionID string, device model.Device, csrfHash []byte, now time.Time) (model.Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, session := range m.sessions {
		if session.ID != sessionID || session.UserID != device.UserID || !session.IsActive(now) {
			continue
		}
		user := m.users[session.UserID]
		if user.Status != "active" || (user.RequiresMFA() && session.MFAVerifiedAt == nil) {
			return model.Device{}, ErrInvalidSession
		}
		if session.DeviceID != "" {
			existing := m.devices[session.DeviceID]
			if existing.RevokedAt != nil || subtle.ConstantTimeCompare(existing.TokenHash, device.TokenHash) != 1 {
				return model.Device{}, ErrConflict
			}
			return existing, nil
		}
		device.MFAVerifiedAt, device.MFAMethod = session.MFAVerifiedAt, session.MFAMethod
		m.devices[device.ID] = device
		session.DeviceID, session.CSRFHash, session.CSRFExpiresAt = device.ID, csrfHash, session.AbsoluteExpiresAt
		m.sessions[key] = session
		return device, nil
	}
	return model.Device{}, ErrNotFound
}

func (m *Memory) DeviceByID(_ context.Context, id string) (model.Device, model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, ok := m.devices[id]
	if !ok {
		return model.Device{}, model.User{}, ErrNotFound
	}
	user, ok := m.users[device.UserID]
	if !ok {
		return model.Device{}, model.User{}, ErrNotFound
	}
	return device, user, nil
}

func (m *Memory) DeviceByTokenHash(_ context.Context, hash []byte) (model.Device, model.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, device := range m.devices {
		if subtle.ConstantTimeCompare(device.TokenHash, hash) == 1 {
			user, ok := m.users[device.UserID]
			if !ok {
				return model.Device{}, model.User{}, ErrNotFound
			}
			return device, user, nil
		}
	}
	return model.Device{}, model.User{}, ErrNotFound
}

func (m *Memory) RestoreDeviceSession(_ context.Context, expected model.Device, session model.Session, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	device, ok := m.devices[expected.ID]
	if !ok || device.RevokedAt != nil || device.UserID != session.UserID || session.DeviceID != device.ID || m.users[device.UserID].Status != "active" {
		return ErrNotFound
	}
	if !validNewSession(session) {
		return ErrInvalidSession
	}
	if _, ok := m.sessions[string(session.TokenHash)]; ok {
		return ErrConflict
	}
	m.sessions[string(session.TokenHash)] = session
	device.LastSeenAt = now
	m.devices[device.ID] = device
	return nil
}

func (m *Memory) RevokeDevice(_ context.Context, userID, deviceID string, now time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.revokeDeviceLocked(userID, deviceID, now)
	return nil
}

func (m *Memory) revokeDeviceLocked(userID, deviceID string, now time.Time) {
	device, ok := m.devices[deviceID]
	if !ok || device.UserID != userID {
		return
	}
	device.RevokedAt = &now
	m.devices[deviceID] = device
	for key, session := range m.sessions {
		if session.DeviceID == deviceID {
			session.RevokedAt = &now
			m.sessions[key] = session
		}
	}
}
