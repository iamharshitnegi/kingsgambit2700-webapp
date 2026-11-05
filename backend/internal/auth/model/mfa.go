package model

import "time"

const MFAMethodTOTP = "totp"

type MFAMethod struct {
	ID              string
	UserID          string
	EncryptedSecret []byte
	VerifiedAt      *time.Time
	LastTOTPStep    *int64
	CreatedAt       time.Time
	DisabledAt      *time.Time
}

func (m MFAMethod) IsVerified() bool {
	return m.VerifiedAt != nil && m.DisabledAt == nil
}

type MFARecoveryCode struct {
	ID        string
	MethodID  string
	CodeHash  []byte
	CreatedAt time.Time
	UsedAt    *time.Time
}

// AuditEvent contains only the security-event fields owned by authentication.
// Repository adapters translate it to the platform-wide append-only audit row.
type AuditEvent struct {
	ID        string
	ActorID   string
	Action    string
	ObjectID  string
	Reason    string
	RequestID string
	Metadata  map[string]any
	CreatedAt time.Time
}
