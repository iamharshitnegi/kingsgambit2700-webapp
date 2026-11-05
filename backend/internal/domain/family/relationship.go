package family

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

var (
	ErrInvalidRelationship  = errors.New("family: invalid relationship")
	ErrInactiveRelationship = errors.New("family: inactive relationship")
)

// Permission is a capability a guardian has for one specific student.
type Permission string

const (
	PermissionUnknown                Permission = ""
	PermissionViewProfile            Permission = "view_profile"
	PermissionManageProfile          Permission = "manage_profile"
	PermissionViewSchedule           Permission = "view_schedule"
	PermissionManageSchedule         Permission = "manage_schedule"
	PermissionJoinClass              Permission = "join_class"
	PermissionViewProgress           Permission = "view_progress"
	PermissionManageConsent          Permission = "manage_consent"
	PermissionManageBilling          Permission = "manage_billing"
	PermissionManageGuardians        Permission = "manage_guardians"
	PermissionCommunicateWithAcademy Permission = "communicate_with_academy"
)

func (p Permission) valid() bool {
	switch p {
	case PermissionViewProfile,
		PermissionManageProfile,
		PermissionViewSchedule,
		PermissionManageSchedule,
		PermissionJoinClass,
		PermissionViewProgress,
		PermissionManageConsent,
		PermissionManageBilling,
		PermissionManageGuardians,
		PermissionCommunicateWithAcademy:
		return true
	default:
		return false
	}
}

type RelationshipStatus string

const (
	RelationshipStatusUnknown RelationshipStatus = ""
	RelationshipStatusActive  RelationshipStatus = "active"
	RelationshipStatusRevoked RelationshipStatus = "revoked"
)

// RelationshipRecord is the persistence representation used to reconstruct a
// relationship without exposing its permission slice for mutation.
type RelationshipRecord struct {
	ID           string
	FamilyID     string
	GuardianID   string
	StudentID    string
	Permissions  []Permission
	Status       RelationshipStatus
	GrantedBy    string
	GrantedAt    time.Time
	RevokedBy    string
	RevokedAt    *time.Time
	RevokeReason string
	Revision     uint64
}

type Relationship struct {
	id           string
	familyID     string
	guardianID   string
	studentID    string
	permissions  []Permission
	status       RelationshipStatus
	grantedBy    string
	grantedAt    time.Time
	revokedBy    string
	revokedAt    *time.Time
	revokeReason string
	revision     uint64
}

type NewRelationshipParams struct {
	ID          string
	FamilyID    string
	GuardianID  string
	StudentID   string
	Permissions []Permission
	GrantedBy   string
	GrantedAt   time.Time
}

func NewRelationship(params NewRelationshipParams) (Relationship, error) {
	permissions, err := normalizePermissions(params.Permissions)
	if err != nil {
		return Relationship{}, err
	}

	relationship := Relationship{
		id:          params.ID,
		familyID:    params.FamilyID,
		guardianID:  params.GuardianID,
		studentID:   params.StudentID,
		permissions: permissions,
		status:      RelationshipStatusActive,
		grantedBy:   params.GrantedBy,
		grantedAt:   params.GrantedAt,
		revision:    1,
	}
	if err := relationship.validate(); err != nil {
		return Relationship{}, err
	}

	return relationship, nil
}

func RestoreRelationship(record RelationshipRecord) (Relationship, error) {
	permissions, err := normalizePermissions(record.Permissions)
	if err != nil {
		return Relationship{}, err
	}

	relationship := Relationship{
		id:           record.ID,
		familyID:     record.FamilyID,
		guardianID:   record.GuardianID,
		studentID:    record.StudentID,
		permissions:  permissions,
		status:       record.Status,
		grantedBy:    record.GrantedBy,
		grantedAt:    record.GrantedAt,
		revokedBy:    record.RevokedBy,
		revokedAt:    cloneTime(record.RevokedAt),
		revokeReason: record.RevokeReason,
		revision:     record.Revision,
	}
	if err := relationship.validate(); err != nil {
		return Relationship{}, err
	}

	return relationship, nil
}

func (r Relationship) ID() string                 { return r.id }
func (r Relationship) FamilyID() string           { return r.familyID }
func (r Relationship) GuardianID() string         { return r.guardianID }
func (r Relationship) StudentID() string          { return r.studentID }
func (r Relationship) Status() RelationshipStatus { return r.status }
func (r Relationship) Revision() uint64           { return r.revision }
func (r Relationship) IsActive() bool             { return r.status == RelationshipStatusActive }
func (r Relationship) Permissions() []Permission  { return slices.Clone(r.permissions) }

func (r Relationship) Record() RelationshipRecord {
	return RelationshipRecord{
		ID:           r.id,
		FamilyID:     r.familyID,
		GuardianID:   r.guardianID,
		StudentID:    r.studentID,
		Permissions:  r.Permissions(),
		Status:       r.status,
		GrantedBy:    r.grantedBy,
		GrantedAt:    r.grantedAt,
		RevokedBy:    r.revokedBy,
		RevokedAt:    cloneTime(r.revokedAt),
		RevokeReason: r.revokeReason,
		Revision:     r.revision,
	}
}

func (r Relationship) Allows(permission Permission) bool {
	if !r.IsActive() || !permission.valid() {
		return false
	}

	return slices.Contains(r.permissions, permission)
}

func (r Relationship) WithPermissions(permissions []Permission) (Relationship, error) {
	if !r.IsActive() {
		return Relationship{}, ErrInactiveRelationship
	}

	normalized, err := normalizePermissions(permissions)
	if err != nil {
		return Relationship{}, err
	}
	if slices.Equal(r.permissions, normalized) {
		return Relationship{}, fmt.Errorf("%w: permissions are unchanged", ErrInvalidRelationship)
	}

	updated := r.clone()
	updated.permissions = normalized
	updated.revision++
	return updated, nil
}

func (r Relationship) Revoke(actorID, reason string, at time.Time) (Relationship, error) {
	if !r.IsActive() {
		return Relationship{}, ErrInactiveRelationship
	}
	if actorID == "" || reason == "" || at.IsZero() || at.Before(r.grantedAt) {
		return Relationship{}, fmt.Errorf("%w: valid actor, reason and time are required", ErrInvalidRelationship)
	}

	updated := r.clone()
	updated.status = RelationshipStatusRevoked
	updated.revokedBy = actorID
	updated.revokedAt = cloneTime(&at)
	updated.revokeReason = reason
	updated.revision++
	return updated, nil
}

func (r Relationship) clone() Relationship {
	cloned := r
	cloned.permissions = slices.Clone(r.permissions)
	cloned.revokedAt = cloneTime(r.revokedAt)
	return cloned
}

func (r Relationship) validate() error {
	if r.id == "" || r.familyID == "" || r.guardianID == "" || r.studentID == "" {
		return fmt.Errorf("%w: ids are required", ErrInvalidRelationship)
	}
	if r.guardianID == r.studentID {
		return fmt.Errorf("%w: guardian and student must differ", ErrInvalidRelationship)
	}
	if r.grantedBy == "" || r.grantedAt.IsZero() || r.revision == 0 {
		return fmt.Errorf("%w: grant metadata is required", ErrInvalidRelationship)
	}
	if len(r.permissions) == 0 {
		return fmt.Errorf("%w: at least one permission is required", ErrInvalidRelationship)
	}

	switch r.status {
	case RelationshipStatusActive:
		if r.revokedBy != "" || r.revokedAt != nil || r.revokeReason != "" {
			return fmt.Errorf("%w: active relationship cannot contain revocation metadata", ErrInvalidRelationship)
		}
	case RelationshipStatusRevoked:
		if r.revokedBy == "" || r.revokedAt == nil || r.revokeReason == "" {
			return fmt.Errorf("%w: revoked relationship requires revocation metadata", ErrInvalidRelationship)
		}
		if r.revokedAt.Before(r.grantedAt) {
			return fmt.Errorf("%w: revocation cannot precede grant", ErrInvalidRelationship)
		}
	default:
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidRelationship, r.status)
	}

	return nil
}

func normalizePermissions(permissions []Permission) ([]Permission, error) {
	if len(permissions) == 0 {
		return nil, fmt.Errorf("%w: at least one permission is required", ErrInvalidRelationship)
	}

	normalized := slices.Clone(permissions)
	slices.Sort(normalized)
	for i, permission := range normalized {
		if !permission.valid() {
			return nil, fmt.Errorf("%w: unsupported permission %q", ErrInvalidRelationship, permission)
		}
		if i > 0 && normalized[i-1] == permission {
			return nil, fmt.Errorf("%w: duplicate permission %q", ErrInvalidRelationship, permission)
		}
	}

	return normalized, nil
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
