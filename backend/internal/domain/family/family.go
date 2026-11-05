package family

import (
	"errors"
	"fmt"
	"time"
)

var ErrInvalidMember = errors.New("family: invalid member")

// MemberRole describes a person's role inside one family. It is separate from
// application-wide role grants because family access is relationship-scoped.
type MemberRole string

const (
	MemberRoleUnknown      MemberRole = ""
	MemberRoleGuardian     MemberRole = "guardian"
	MemberRoleStudent      MemberRole = "student"
	MemberRoleAdultLearner MemberRole = "adult_learner"
)

func (r MemberRole) valid() bool {
	switch r {
	case MemberRoleGuardian, MemberRoleStudent, MemberRoleAdultLearner:
		return true
	default:
		return false
	}
}

type MemberStatus string

const (
	MemberStatusUnknown MemberStatus = ""
	MemberStatusActive  MemberStatus = "active"
	MemberStatusRemoved MemberStatus = "removed"
)

// Family is an access and billing group. It does not imply a shared surname or
// physical household.
type Family struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

// Member links a person to a family. Fine-grained guardian access to a student
// is represented by Relationship rather than inferred from membership alone.
type Member struct {
	ID        string
	FamilyID  string
	PersonID  string
	Role      MemberRole
	Status    MemberStatus
	CreatedAt time.Time
	RemovedAt *time.Time
}

func (m Member) Validate() error {
	if m.ID == "" || m.FamilyID == "" || m.PersonID == "" {
		return fmt.Errorf("%w: ids are required", ErrInvalidMember)
	}
	if !m.Role.valid() {
		return fmt.Errorf("%w: unsupported role %q", ErrInvalidMember, m.Role)
	}
	if m.CreatedAt.IsZero() {
		return fmt.Errorf("%w: created at is required", ErrInvalidMember)
	}

	switch m.Status {
	case MemberStatusActive:
		if m.RemovedAt != nil {
			return fmt.Errorf("%w: active member cannot have removal time", ErrInvalidMember)
		}
	case MemberStatusRemoved:
		if m.RemovedAt == nil || m.RemovedAt.IsZero() {
			return fmt.Errorf("%w: removed member requires removal time", ErrInvalidMember)
		}
	default:
		return fmt.Errorf("%w: unsupported status %q", ErrInvalidMember, m.Status)
	}

	return nil
}
