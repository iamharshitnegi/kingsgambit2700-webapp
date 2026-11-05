package scheduling

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidPolicy  = errors.New("scheduling: invalid policy")
	ErrInvalidRequest = errors.New("scheduling: invalid request")
)

const DefaultOneToOneCancellationCutoff = 24 * time.Hour

type ClassFormat string

const (
	ClassFormatUnknown  ClassFormat = ""
	ClassFormatOneToOne ClassFormat = "one_to_one"
	ClassFormatGroup    ClassFormat = "group"
)

func (f ClassFormat) valid() bool {
	return f == ClassFormatOneToOne || f == ClassFormatGroup
}

type CancellationActor string

const (
	CancellationActorUnknown CancellationActor = ""
	CancellationActorFamily  CancellationActor = "family"
	CancellationActorAcademy CancellationActor = "academy"
)

func (a CancellationActor) valid() bool {
	return a == CancellationActorFamily || a == CancellationActorAcademy
}

type CancellationKind string

const (
	CancellationKindUnknown CancellationKind = ""
	CancellationKindRequest CancellationKind = "request"
	CancellationKindAbsence CancellationKind = "absence"
	CancellationKindNoShow  CancellationKind = "no_show"
)

func (k CancellationKind) valid() bool {
	switch k {
	case CancellationKindRequest, CancellationKindAbsence, CancellationKindNoShow:
		return true
	default:
		return false
	}
}

type Eligibility string

const (
	EligibilityUnknown        Eligibility = ""
	EligibilityEligible       Eligibility = "eligible"
	EligibilityNotEligible    Eligibility = "not_eligible"
	EligibilityRequiresReview Eligibility = "requires_review"
)

type EntitlementAction string

const (
	EntitlementActionUnknown EntitlementAction = ""
	EntitlementActionNone    EntitlementAction = "none"
	EntitlementActionRestore EntitlementAction = "restore"
	EntitlementActionConsume EntitlementAction = "consume"
	EntitlementActionHold    EntitlementAction = "hold"
)

type MakeUpAction string

const (
	MakeUpActionUnknown MakeUpAction = ""
	MakeUpActionNone    MakeUpAction = "none"
	MakeUpActionCreate  MakeUpAction = "create_obligation"
)

type ReasonCode string

const (
	ReasonUnknown                  ReasonCode = ""
	ReasonOneToOneBeforeCutoff     ReasonCode = "one_to_one_before_cutoff"
	ReasonOneToOneLateCancellation ReasonCode = "one_to_one_late_cancellation"
	ReasonFamilyNoShow             ReasonCode = "family_no_show"
	ReasonGroupFamilyAbsence       ReasonCode = "group_family_absence"
	ReasonAcademyCancellation      ReasonCode = "academy_cancellation"
	ReasonDelivered                ReasonCode = "delivered"
	ReasonAttendanceUnresolved     ReasonCode = "attendance_unresolved"
	ReasonCoachNotesMissing        ReasonCode = "coach_notes_missing"
	ReasonTechnicalFailure         ReasonCode = "technical_failure"
)

// Policy is the snapshotted scheduling policy accepted with a plan version.
type Policy struct {
	VersionID                  string
	OneToOneCancellationCutoff time.Duration
}

func DefaultPolicy(versionID string) (Policy, error) {
	policy := Policy{
		VersionID:                  versionID,
		OneToOneCancellationCutoff: DefaultOneToOneCancellationCutoff,
	}
	if err := policy.Validate(); err != nil {
		return Policy{}, err
	}
	return policy, nil
}

func (p Policy) Validate() error {
	if p.VersionID == "" {
		return fmt.Errorf("%w: version id is required", ErrInvalidPolicy)
	}
	if p.OneToOneCancellationCutoff <= 0 {
		return fmt.Errorf("%w: cancellation cutoff must be positive", ErrInvalidPolicy)
	}
	return nil
}

type CancellationInput struct {
	SessionID   string
	StudentID   string
	Format      ClassFormat
	Actor       CancellationActor
	Kind        CancellationKind
	StartsAt    time.Time
	RequestedAt time.Time
}

func (i CancellationInput) Validate() error {
	if i.SessionID == "" || i.StudentID == "" {
		return fmt.Errorf("%w: session and student ids are required", ErrInvalidRequest)
	}
	if !i.Format.valid() || !i.Actor.valid() || !i.Kind.valid() {
		return fmt.Errorf("%w: format, actor or kind is unsupported", ErrInvalidRequest)
	}
	if i.StartsAt.IsZero() || i.RequestedAt.IsZero() {
		return fmt.Errorf("%w: start and request times are required", ErrInvalidRequest)
	}
	if i.Actor == CancellationActorAcademy && i.Kind == CancellationKindNoShow {
		return fmt.Errorf("%w: academy cannot be a no-show", ErrInvalidRequest)
	}
	return nil
}

type CancellationDecision struct {
	SessionID         string
	StudentID         string
	PolicyVersionID   string
	Eligibility       Eligibility
	EntitlementAction EntitlementAction
	MakeUpAction      MakeUpAction
	ReleaseCoachSlot  bool
	CutoffAt          *time.Time
	Reason            ReasonCode
}

func (p Policy) EvaluateCancellation(input CancellationInput) (CancellationDecision, error) {
	if err := p.Validate(); err != nil {
		return CancellationDecision{}, err
	}
	if err := input.Validate(); err != nil {
		return CancellationDecision{}, err
	}

	decision := CancellationDecision{
		SessionID:         input.SessionID,
		StudentID:         input.StudentID,
		PolicyVersionID:   p.VersionID,
		Eligibility:       EligibilityNotEligible,
		EntitlementAction: EntitlementActionConsume,
		MakeUpAction:      MakeUpActionNone,
		Reason:            ReasonFamilyNoShow,
	}

	if input.Actor == CancellationActorAcademy {
		decision.Eligibility = EligibilityEligible
		decision.EntitlementAction = EntitlementActionRestore
		decision.MakeUpAction = MakeUpActionCreate
		decision.ReleaseCoachSlot = true
		decision.Reason = ReasonAcademyCancellation
		return decision, nil
	}

	if input.Kind == CancellationKindNoShow {
		return decision, nil
	}

	if input.Format == ClassFormatGroup {
		decision.Reason = ReasonGroupFamilyAbsence
		return decision, nil
	}

	cutoffAt := input.StartsAt.Add(-p.OneToOneCancellationCutoff)
	decision.CutoffAt = &cutoffAt
	if input.RequestedAt.After(cutoffAt) {
		decision.Reason = ReasonOneToOneLateCancellation
		return decision, nil
	}

	decision.Eligibility = EligibilityEligible
	decision.EntitlementAction = EntitlementActionRestore
	decision.ReleaseCoachSlot = true
	decision.Reason = ReasonOneToOneBeforeCutoff
	return decision, nil
}

type SessionOutcome string

const (
	SessionOutcomeUnknown          SessionOutcome = ""
	SessionOutcomeDelivered        SessionOutcome = "delivered"
	SessionOutcomeFamilyCancelled  SessionOutcome = "family_cancelled"
	SessionOutcomeAcademyCancelled SessionOutcome = "academy_cancelled"
	SessionOutcomeNoShow           SessionOutcome = "no_show"
	SessionOutcomeTechnicalFailure SessionOutcome = "technical_failure"
)

func (o SessionOutcome) valid() bool {
	switch o {
	case SessionOutcomeDelivered,
		SessionOutcomeFamilyCancelled,
		SessionOutcomeAcademyCancelled,
		SessionOutcomeNoShow,
		SessionOutcomeTechnicalFailure:
		return true
	default:
		return false
	}
}

type CompletionStatus string

const (
	CompletionStatusUnknown CompletionStatus = ""
	CompletionStatusReady   CompletionStatus = "ready"
	CompletionStatusBlocked CompletionStatus = "blocked"
	CompletionStatusReview  CompletionStatus = "requires_review"
)

type CompletionInput struct {
	SessionID           string
	StudentID           string
	Format              ClassFormat
	Outcome             SessionOutcome
	AttendanceResolved  bool
	CoachNotesCompleted bool
	Cancellation        *CancellationDecision
}

func (i CompletionInput) Validate() error {
	if i.SessionID == "" || i.StudentID == "" || !i.Format.valid() || !i.Outcome.valid() {
		return fmt.Errorf("%w: completion identity or outcome is invalid", ErrInvalidRequest)
	}
	return nil
}

type CompletionDecision struct {
	SessionID         string
	StudentID         string
	PolicyVersionID   string
	Status            CompletionStatus
	EntitlementAction EntitlementAction
	MakeUpAction      MakeUpAction
	Reason            ReasonCode
}

func (p Policy) EvaluateCompletion(input CompletionInput) (CompletionDecision, error) {
	if err := p.Validate(); err != nil {
		return CompletionDecision{}, err
	}
	if err := input.Validate(); err != nil {
		return CompletionDecision{}, err
	}

	decision := CompletionDecision{
		SessionID:         input.SessionID,
		StudentID:         input.StudentID,
		PolicyVersionID:   p.VersionID,
		Status:            CompletionStatusReady,
		EntitlementAction: EntitlementActionConsume,
		MakeUpAction:      MakeUpActionNone,
		Reason:            ReasonDelivered,
	}

	switch input.Outcome {
	case SessionOutcomeDelivered:
		if !input.AttendanceResolved {
			decision.Status = CompletionStatusBlocked
			decision.EntitlementAction = EntitlementActionHold
			decision.Reason = ReasonAttendanceUnresolved
			return decision, nil
		}
		if !input.CoachNotesCompleted {
			decision.Status = CompletionStatusBlocked
			decision.EntitlementAction = EntitlementActionHold
			decision.Reason = ReasonCoachNotesMissing
		}
		return decision, nil

	case SessionOutcomeNoShow:
		if !input.AttendanceResolved {
			decision.Status = CompletionStatusBlocked
			decision.EntitlementAction = EntitlementActionHold
			decision.Reason = ReasonAttendanceUnresolved
			return decision, nil
		}
		decision.Reason = ReasonFamilyNoShow
		return decision, nil

	case SessionOutcomeTechnicalFailure:
		decision.Status = CompletionStatusReview
		decision.EntitlementAction = EntitlementActionHold
		decision.Reason = ReasonTechnicalFailure
		return decision, nil

	case SessionOutcomeFamilyCancelled, SessionOutcomeAcademyCancelled:
		if input.Cancellation == nil {
			return CompletionDecision{}, fmt.Errorf("%w: cancellation decision is required", ErrInvalidRequest)
		}
		cancellation := input.Cancellation
		if cancellation.SessionID != input.SessionID ||
			cancellation.StudentID != input.StudentID ||
			cancellation.PolicyVersionID != p.VersionID {
			return CompletionDecision{}, fmt.Errorf("%w: cancellation decision does not match completion", ErrInvalidRequest)
		}
		isAcademyDecision := cancellation.Reason == ReasonAcademyCancellation
		isAcademyOutcome := input.Outcome == SessionOutcomeAcademyCancelled
		if isAcademyDecision != isAcademyOutcome {
			return CompletionDecision{}, fmt.Errorf("%w: cancellation actor does not match completion", ErrInvalidRequest)
		}
		decision.EntitlementAction = cancellation.EntitlementAction
		decision.MakeUpAction = cancellation.MakeUpAction
		decision.Reason = cancellation.Reason
		return decision, nil
	}

	return CompletionDecision{}, fmt.Errorf("%w: unsupported completion outcome", ErrInvalidRequest)
}
