package entitlement

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

var ErrInvalidEntry = errors.New("entitlement: invalid entry")

type EntryKind string

const (
	EntryKindUnknown   EntryKind = ""
	EntryKindPurchased EntryKind = "purchased"
	EntryKindGranted   EntryKind = "granted"
	EntryKindReserved  EntryKind = "reserved"
	EntryKindConsumed  EntryKind = "consumed"
	EntryKindReleased  EntryKind = "released"
	EntryKindReversed  EntryKind = "reversed"
	EntryKindExpired   EntryKind = "expired"
	EntryKindRefunded  EntryKind = "refunded"
	EntryKindAdjusted  EntryKind = "adjusted"
)

func (k EntryKind) valid() bool {
	switch k {
	case EntryKindPurchased,
		EntryKindGranted,
		EntryKindReserved,
		EntryKindConsumed,
		EntryKindReleased,
		EntryKindReversed,
		EntryKindExpired,
		EntryKindRefunded,
		EntryKindAdjusted:
		return true
	default:
		return false
	}
}

// Delta changes the available, reserved and lifetime-consumed buckets. Keeping
// the buckets separate prevents a reserved credit from being deducted twice
// when a class completes.
type Delta struct {
	Available int64
	Reserved  int64
	Consumed  int64
}

func (d Delta) isZero() bool {
	return d.Available == 0 && d.Reserved == 0 && d.Consumed == 0
}

type Balance struct {
	Available int64
	Reserved  int64
	Consumed  int64
}

func (b Balance) apply(delta Delta) (Balance, bool) {
	available, ok := safeAdd(b.Available, delta.Available)
	if !ok {
		return Balance{}, false
	}
	reserved, ok := safeAdd(b.Reserved, delta.Reserved)
	if !ok {
		return Balance{}, false
	}
	consumed, ok := safeAdd(b.Consumed, delta.Consumed)
	if !ok {
		return Balance{}, false
	}
	return Balance{Available: available, Reserved: reserved, Consumed: consumed}, true
}

func (b Balance) valid() bool {
	return b.Available >= 0 && b.Reserved >= 0 && b.Consumed >= 0
}

func safeAdd(left, right int64) (int64, bool) {
	if right > 0 && left > math.MaxInt64-right {
		return 0, false
	}
	if right < 0 && left < math.MinInt64-right {
		return 0, false
	}
	return left + right, true
}

// Entry is an immutable ledger fact. IdempotencyKey is unique per account and
// is part of the durable business operation, not merely a job-queue key.
type Entry struct {
	ID             string
	AccountID      string
	StudentID      string
	Kind           EntryKind
	Delta          Delta
	IdempotencyKey string
	RequestDigest  [32]byte
	ReferenceType  string
	ReferenceID    string
	Reason         string
	ActorID        string
	OccurredAt     time.Time
}

func (e Entry) Validate() error {
	if e.ID == "" || e.AccountID == "" || e.StudentID == "" || e.IdempotencyKey == "" {
		return fmt.Errorf("%w: entry and account identity are required", ErrInvalidEntry)
	}
	if len(e.IdempotencyKey) > 255 || !hasRequestDigest(e.RequestDigest) {
		return fmt.Errorf("%w: idempotency evidence is invalid", ErrInvalidEntry)
	}
	if !validReferenceType(e.ReferenceType) || strings.TrimSpace(e.ReferenceID) == "" ||
		len(e.ReferenceID) > 255 {
		return fmt.Errorf("%w: reference is invalid", ErrInvalidEntry)
	}
	if strings.TrimSpace(e.Reason) == "" || len(e.Reason) > 500 ||
		e.ActorID == "" || e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: reference, reason, actor and time are required", ErrInvalidEntry)
	}
	if !e.Kind.valid() || e.Delta.isZero() {
		return fmt.Errorf("%w: kind or delta is invalid", ErrInvalidEntry)
	}
	if err := validateDelta(e.Kind, e.Delta); err != nil {
		return err
	}
	return nil
}

func hasRequestDigest(digest [32]byte) bool {
	var zero [32]byte
	return subtle.ConstantTimeCompare(digest[:], zero[:]) == 0
}

func validReferenceType(value string) bool {
	if len(value) == 0 || len(value) > 64 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for _, character := range value[1:] {
		isLowercase := character >= 'a' && character <= 'z'
		isDigit := character >= '0' && character <= '9'
		if !isLowercase && !isDigit && character != '_' {
			return false
		}
	}
	return true
}

func validateDelta(kind EntryKind, delta Delta) error {
	switch kind {
	case EntryKindPurchased, EntryKindGranted:
		if delta.Available > 0 && delta.Reserved == 0 && delta.Consumed == 0 {
			return nil
		}

	case EntryKindReserved:
		if delta.Available < 0 &&
			delta.Reserved == -delta.Available &&
			delta.Consumed == 0 {
			return nil
		}

	case EntryKindConsumed:
		fromAvailable := delta.Available < 0 &&
			delta.Reserved == 0 &&
			delta.Consumed == -delta.Available
		fromReservation := delta.Reserved < 0 &&
			delta.Available == 0 &&
			delta.Consumed == -delta.Reserved
		if fromAvailable || fromReservation {
			return nil
		}

	case EntryKindReleased:
		if delta.Available > 0 &&
			delta.Reserved == -delta.Available &&
			delta.Consumed == 0 {
			return nil
		}

	case EntryKindReversed:
		if delta.Available > 0 &&
			delta.Reserved == 0 &&
			delta.Consumed == -delta.Available {
			return nil
		}

	case EntryKindExpired, EntryKindRefunded:
		if delta.Available < 0 && delta.Reserved == 0 && delta.Consumed == 0 {
			return nil
		}

	case EntryKindAdjusted:
		if !delta.isZero() {
			return nil
		}
	}

	return fmt.Errorf("%w: delta is incompatible with kind %q", ErrInvalidEntry, kind)
}
