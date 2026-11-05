package scheduling

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPolicy_EvaluateCancellation(t *testing.T) {
	t.Parallel()

	policy, err := DefaultPolicy("plan-policy-v1")
	if err != nil {
		t.Fatalf("DefaultPolicy() error = %v", err)
	}
	startsAt := time.Date(2026, time.August, 30, 10, 0, 0, 0, time.UTC)
	tests := []struct {
		name            string
		input           CancellationInput
		wantEligibility Eligibility
		wantEntitlement EntitlementAction
		wantMakeUp      MakeUpAction
		wantReason      ReasonCode
	}{
		{
			name: "one to one before cutoff restores credit",
			input: CancellationInput{
				SessionID:   "session-1",
				StudentID:   "student-1",
				Format:      ClassFormatOneToOne,
				Actor:       CancellationActorFamily,
				Kind:        CancellationKindRequest,
				StartsAt:    startsAt,
				RequestedAt: startsAt.Add(-25 * time.Hour),
			},
			wantEligibility: EligibilityEligible,
			wantEntitlement: EntitlementActionRestore,
			wantMakeUp:      MakeUpActionNone,
			wantReason:      ReasonOneToOneBeforeCutoff,
		},
		{
			name: "one to one at cutoff restores credit",
			input: CancellationInput{
				SessionID:   "session-1",
				StudentID:   "student-1",
				Format:      ClassFormatOneToOne,
				Actor:       CancellationActorFamily,
				Kind:        CancellationKindRequest,
				StartsAt:    startsAt,
				RequestedAt: startsAt.Add(-DefaultOneToOneCancellationCutoff),
			},
			wantEligibility: EligibilityEligible,
			wantEntitlement: EntitlementActionRestore,
			wantMakeUp:      MakeUpActionNone,
			wantReason:      ReasonOneToOneBeforeCutoff,
		},
		{
			name: "late one to one consumes credit",
			input: CancellationInput{
				SessionID:   "session-1",
				StudentID:   "student-1",
				Format:      ClassFormatOneToOne,
				Actor:       CancellationActorFamily,
				Kind:        CancellationKindRequest,
				StartsAt:    startsAt,
				RequestedAt: startsAt.Add(-23 * time.Hour),
			},
			wantEligibility: EligibilityNotEligible,
			wantEntitlement: EntitlementActionConsume,
			wantMakeUp:      MakeUpActionNone,
			wantReason:      ReasonOneToOneLateCancellation,
		},
		{
			name: "family no show consumes credit",
			input: CancellationInput{
				SessionID:   "session-1",
				StudentID:   "student-1",
				Format:      ClassFormatOneToOne,
				Actor:       CancellationActorFamily,
				Kind:        CancellationKindNoShow,
				StartsAt:    startsAt,
				RequestedAt: startsAt,
			},
			wantEligibility: EligibilityNotEligible,
			wantEntitlement: EntitlementActionConsume,
			wantMakeUp:      MakeUpActionNone,
			wantReason:      ReasonFamilyNoShow,
		},
		{
			name: "group family absence consumes without makeup",
			input: CancellationInput{
				SessionID:   "session-1",
				StudentID:   "student-1",
				Format:      ClassFormatGroup,
				Actor:       CancellationActorFamily,
				Kind:        CancellationKindAbsence,
				StartsAt:    startsAt,
				RequestedAt: startsAt.Add(-7 * 24 * time.Hour),
			},
			wantEligibility: EligibilityNotEligible,
			wantEntitlement: EntitlementActionConsume,
			wantMakeUp:      MakeUpActionNone,
			wantReason:      ReasonGroupFamilyAbsence,
		},
		{
			name: "academy cancellation restores and creates makeup obligation",
			input: CancellationInput{
				SessionID:   "session-1",
				StudentID:   "student-1",
				Format:      ClassFormatGroup,
				Actor:       CancellationActorAcademy,
				Kind:        CancellationKindRequest,
				StartsAt:    startsAt,
				RequestedAt: startsAt.Add(-time.Hour),
			},
			wantEligibility: EligibilityEligible,
			wantEntitlement: EntitlementActionRestore,
			wantMakeUp:      MakeUpActionCreate,
			wantReason:      ReasonAcademyCancellation,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decision, err := policy.EvaluateCancellation(test.input)
			if err != nil {
				t.Fatalf("EvaluateCancellation() error = %v", err)
			}
			if decision.Eligibility != test.wantEligibility {
				t.Errorf("Eligibility = %q, want %q", decision.Eligibility, test.wantEligibility)
			}
			if decision.EntitlementAction != test.wantEntitlement {
				t.Errorf("EntitlementAction = %q, want %q", decision.EntitlementAction, test.wantEntitlement)
			}
			if decision.MakeUpAction != test.wantMakeUp {
				t.Errorf("MakeUpAction = %q, want %q", decision.MakeUpAction, test.wantMakeUp)
			}
			if decision.Reason != test.wantReason {
				t.Errorf("Reason = %q, want %q", decision.Reason, test.wantReason)
			}
		})
	}
}

func TestPolicy_EvaluateCompletion(t *testing.T) {
	t.Parallel()

	policy, err := DefaultPolicy("plan-policy-v1")
	if err != nil {
		t.Fatalf("DefaultPolicy() error = %v", err)
	}
	tests := []struct {
		name            string
		input           CompletionInput
		wantStatus      CompletionStatus
		wantEntitlement EntitlementAction
		wantReason      ReasonCode
	}{
		{
			name: "attendance must be resolved",
			input: CompletionInput{
				SessionID:           "session-1",
				StudentID:           "student-1",
				Format:              ClassFormatGroup,
				Outcome:             SessionOutcomeDelivered,
				AttendanceResolved:  false,
				CoachNotesCompleted: true,
			},
			wantStatus:      CompletionStatusBlocked,
			wantEntitlement: EntitlementActionHold,
			wantReason:      ReasonAttendanceUnresolved,
		},
		{
			name: "coach notes must be completed",
			input: CompletionInput{
				SessionID:           "session-1",
				StudentID:           "student-1",
				Format:              ClassFormatGroup,
				Outcome:             SessionOutcomeDelivered,
				AttendanceResolved:  true,
				CoachNotesCompleted: false,
			},
			wantStatus:      CompletionStatusBlocked,
			wantEntitlement: EntitlementActionHold,
			wantReason:      ReasonCoachNotesMissing,
		},
		{
			name: "resolved delivery consumes entitlement",
			input: CompletionInput{
				SessionID:           "session-1",
				StudentID:           "student-1",
				Format:              ClassFormatGroup,
				Outcome:             SessionOutcomeDelivered,
				AttendanceResolved:  true,
				CoachNotesCompleted: true,
			},
			wantStatus:      CompletionStatusReady,
			wantEntitlement: EntitlementActionConsume,
			wantReason:      ReasonDelivered,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			decision, err := policy.EvaluateCompletion(test.input)
			if err != nil {
				t.Fatalf("EvaluateCompletion() error = %v", err)
			}
			if decision.Status != test.wantStatus ||
				decision.EntitlementAction != test.wantEntitlement ||
				decision.Reason != test.wantReason {
				t.Fatalf(
					"EvaluateCompletion() = status %q, action %q, reason %q; want %q, %q, %q",
					decision.Status,
					decision.EntitlementAction,
					decision.Reason,
					test.wantStatus,
					test.wantEntitlement,
					test.wantReason,
				)
			}
		})
	}
}

func TestService_CancellationIsIdempotent(t *testing.T) {
	t.Parallel()

	policy, err := DefaultPolicy("plan-policy-v1")
	if err != nil {
		t.Fatalf("DefaultPolicy() error = %v", err)
	}
	service, err := NewService(policy, NewMemoryRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	command := CancellationCommand{
		ID:             "decision-1",
		IdempotencyKey: "cancel:session-1:student-1",
		ActorID:        "guardian-1",
		Input: CancellationInput{
			SessionID:   "session-1",
			StudentID:   "student-1",
			Format:      ClassFormatOneToOne,
			Actor:       CancellationActorFamily,
			Kind:        CancellationKindRequest,
			StartsAt:    now.Add(48 * time.Hour),
			RequestedAt: now,
		},
		CreatedAt: now,
	}

	first, err := service.DecideCancellation(context.Background(), command)
	if err != nil {
		t.Fatalf("DecideCancellation(first) error = %v", err)
	}
	if !first.Created {
		t.Fatal("DecideCancellation(first) Created = false, want true")
	}
	command.ID = "decision-retry"
	command.CreatedAt = now.Add(time.Minute)
	second, err := service.DecideCancellation(context.Background(), command)
	if err != nil {
		t.Fatalf("DecideCancellation(retry) error = %v", err)
	}
	if second.Created {
		t.Fatal("DecideCancellation(retry) Created = true, want false")
	}
	if second.Record.ID != first.Record.ID {
		t.Fatalf("DecideCancellation(retry) ID = %q, want %q", second.Record.ID, first.Record.ID)
	}

	command.Input.RequestedAt = now.Add(25 * time.Hour)
	_, err = service.DecideCancellation(context.Background(), command)
	if !errors.Is(err, ErrDecisionConflict) {
		t.Fatalf("DecideCancellation(conflict) error = %v, want %v", err, ErrDecisionConflict)
	}
}

func TestService_AcademyCancellationCreatesMakeUpObligation(t *testing.T) {
	t.Parallel()

	policy, err := DefaultPolicy("plan-policy-v1")
	if err != nil {
		t.Fatalf("DefaultPolicy() error = %v", err)
	}
	repository := NewMemoryRepository()
	service, err := NewService(policy, repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	result, err := service.DecideCancellation(context.Background(), CancellationCommand{
		ID:                 "decision-1",
		IdempotencyKey:     "academy-cancel:session-1:student-1",
		ActorID:            "operations-1",
		MakeUpObligationID: "make-up-1",
		Input: CancellationInput{
			SessionID:   "session-1",
			StudentID:   "student-1",
			Format:      ClassFormatGroup,
			Actor:       CancellationActorAcademy,
			Kind:        CancellationKindRequest,
			StartsAt:    now.Add(time.Hour),
			RequestedAt: now,
		},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("DecideCancellation() error = %v", err)
	}
	if result.Record.MakeUpObligation == nil {
		t.Fatal("DecideCancellation() did not create a make-up obligation")
	}
	obligation, err := repository.MakeUpObligation(context.Background(), "make-up-1")
	if err != nil {
		t.Fatalf("MakeUpObligation() error = %v", err)
	}
	if obligation.Status != MakeUpStatusOpen || obligation.SessionID != "session-1" {
		t.Fatalf("MakeUpObligation() = %+v, want open obligation for session-1", obligation)
	}
}
