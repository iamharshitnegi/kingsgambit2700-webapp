package model

import "time"

type Role string

const (
	RoleGuardian        Role = "guardian"
	RoleStudent         Role = "student"
	RoleCoach           Role = "coach"
	RoleAcademicManager Role = "academic_manager"
	RoleOperationsAdmin Role = "operations_admin"
	RoleFinanceAdmin    Role = "finance_admin"
	RoleSuperAdmin      Role = "super_admin"
)

type User struct {
	ID          string    `json:"id"`
	Email       string    `json:"email,omitempty"`
	DisplayName string    `json:"displayName"`
	Roles       []Role    `json:"roles"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Device is a revocable remembered browser credential. It has no application
// inactivity expiry; browser cookie retention bounds how long it can be used.
type Device struct {
	ID            string
	UserID        string
	TokenHash     []byte
	CreatedAt     time.Time
	LastSeenAt    time.Time
	RevokedAt     *time.Time
	MFAVerifiedAt *time.Time
	MFAMethod     string
}

type Session struct {
	DeviceID          string
	ID                string
	UserID            string
	TokenHash         []byte
	CSRFHash          []byte
	CSRFExpiresAt     time.Time
	CreatedAt         time.Time
	LastSeenAt        time.Time
	IdleExpiresAt     time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         *time.Time
	UserAgentHash     []byte
	IPAddressHash     []byte
	MFAVerifiedAt     *time.Time
	MFAMethod         string
}

func (s Session) IsActive(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.IdleExpiresAt) && now.Before(s.AbsoluteExpiresAt)
}

// SessionSummary is the safe, user-facing view of an authenticated device.
// It intentionally excludes session, CSRF, IP-address, and user-agent hashes.
type SessionSummary struct {
	Remembered        bool      `json:"remembered"`
	ID                string    `json:"id"`
	CreatedAt         time.Time `json:"createdAt"`
	LastSeenAt        time.Time `json:"lastSeenAt"`
	IdleExpiresAt     time.Time `json:"idleExpiresAt"`
	AbsoluteExpiresAt time.Time `json:"absoluteExpiresAt"`
	DeviceFingerprint string    `json:"deviceFingerprint"`
	Current           bool      `json:"current"`
}

type MagicLink struct {
	ID           string
	Email        string
	TokenHash    []byte
	RedirectPath string
	ExpiresAt    time.Time
	ConsumedAt   *time.Time
	CreatedAt    time.Time
}

type StudentCode struct {
	ID            string
	StudentUserID string
	CodeHash      []byte
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
	CreatedBy     string
}

type Principal struct {
	DeviceID      string
	SessionID     string
	User          User
	MFAVerifiedAt *time.Time
}

func (p Principal) HasRole(role Role) bool {
	for _, candidate := range p.User.Roles {
		if candidate == role {
			return true
		}
	}
	return false
}

// RequiresMFA reports whether the user has any role that is forbidden from
// using a primary-factor-only session.
func (u User) RequiresMFA() bool {
	for _, role := range u.Roles {
		switch role {
		case RoleCoach, RoleAcademicManager, RoleOperationsAdmin, RoleFinanceAdmin, RoleSuperAdmin:
			return true
		}
	}
	return false
}
