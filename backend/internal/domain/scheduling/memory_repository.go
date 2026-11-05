package scheduling

import (
	"context"
	"sync"
)

type MemoryRepository struct {
	mu                sync.RWMutex
	cancellations     map[string]CancellationRecord
	completionRecords map[string]CompletionRecord
	makeUpObligations map[string]MakeUpObligation
	cancellationKeys  map[string]string
	completionKeys    map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		cancellations:     make(map[string]CancellationRecord),
		completionRecords: make(map[string]CompletionRecord),
		makeUpObligations: make(map[string]MakeUpObligation),
		cancellationKeys:  make(map[string]string),
		completionKeys:    make(map[string]string),
	}
}

var _ Repository = (*MemoryRepository)(nil)

func (r *MemoryRepository) StoreCancellation(
	ctx context.Context,
	record CancellationRecord,
) (StoreCancellationResult, error) {
	if err := ctx.Err(); err != nil {
		return StoreCancellationResult{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existingID, exists := r.cancellationKeys[record.IdempotencyKey]; exists {
		existing := r.cancellations[existingID]
		if !sameCancellation(existing, record) {
			return StoreCancellationResult{}, ErrDecisionConflict
		}
		return StoreCancellationResult{Record: cloneCancellation(existing)}, nil
	}
	if _, exists := r.cancellations[record.ID]; exists {
		return StoreCancellationResult{}, ErrDecisionConflict
	}
	if record.MakeUpObligation != nil {
		if _, exists := r.makeUpObligations[record.MakeUpObligation.ID]; exists {
			return StoreCancellationResult{}, ErrDecisionConflict
		}
	}

	r.cancellations[record.ID] = cloneCancellation(record)
	r.cancellationKeys[record.IdempotencyKey] = record.ID
	if record.MakeUpObligation != nil {
		r.makeUpObligations[record.MakeUpObligation.ID] = *record.MakeUpObligation
	}
	return StoreCancellationResult{Record: cloneCancellation(record), Created: true}, nil
}

func (r *MemoryRepository) StoreCompletion(
	ctx context.Context,
	record CompletionRecord,
) (StoreCompletionResult, error) {
	if err := ctx.Err(); err != nil {
		return StoreCompletionResult{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existingID, exists := r.completionKeys[record.IdempotencyKey]; exists {
		existing := r.completionRecords[existingID]
		if !sameCompletion(existing, record) {
			return StoreCompletionResult{}, ErrDecisionConflict
		}
		return StoreCompletionResult{Record: cloneCompletion(existing)}, nil
	}
	if _, exists := r.completionRecords[record.ID]; exists {
		return StoreCompletionResult{}, ErrDecisionConflict
	}

	r.completionRecords[record.ID] = cloneCompletion(record)
	r.completionKeys[record.IdempotencyKey] = record.ID
	return StoreCompletionResult{Record: cloneCompletion(record), Created: true}, nil
}

func (r *MemoryRepository) Cancellation(
	ctx context.Context,
	id string,
) (CancellationRecord, error) {
	if err := ctx.Err(); err != nil {
		return CancellationRecord{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.cancellations[id]
	if !exists {
		return CancellationRecord{}, ErrDecisionNotFound
	}
	return cloneCancellation(record), nil
}

func (r *MemoryRepository) Completion(ctx context.Context, id string) (CompletionRecord, error) {
	if err := ctx.Err(); err != nil {
		return CompletionRecord{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	record, exists := r.completionRecords[id]
	if !exists {
		return CompletionRecord{}, ErrDecisionNotFound
	}
	return cloneCompletion(record), nil
}

func (r *MemoryRepository) MakeUpObligation(
	ctx context.Context,
	id string,
) (MakeUpObligation, error) {
	if err := ctx.Err(); err != nil {
		return MakeUpObligation{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	obligation, exists := r.makeUpObligations[id]
	if !exists {
		return MakeUpObligation{}, ErrDecisionNotFound
	}
	return obligation, nil
}

func sameCancellation(left, right CancellationRecord) bool {
	return left.IdempotencyKey == right.IdempotencyKey &&
		left.ActorID == right.ActorID &&
		left.Input == right.Input &&
		sameCancellationDecision(&left.Decision, &right.Decision) &&
		sameMakeUpObligation(left.MakeUpObligation, right.MakeUpObligation)
}

func sameCompletion(left, right CompletionRecord) bool {
	return left.IdempotencyKey == right.IdempotencyKey &&
		left.Input.SessionID == right.Input.SessionID &&
		left.Input.StudentID == right.Input.StudentID &&
		left.Input.Format == right.Input.Format &&
		left.Input.Outcome == right.Input.Outcome &&
		left.Input.AttendanceResolved == right.Input.AttendanceResolved &&
		left.Input.CoachNotesCompleted == right.Input.CoachNotesCompleted &&
		sameCancellationDecision(left.Input.Cancellation, right.Input.Cancellation) &&
		left.Decision == right.Decision
}

func sameCancellationDecision(left, right *CancellationDecision) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	if left.SessionID != right.SessionID ||
		left.StudentID != right.StudentID ||
		left.PolicyVersionID != right.PolicyVersionID ||
		left.Eligibility != right.Eligibility ||
		left.EntitlementAction != right.EntitlementAction ||
		left.MakeUpAction != right.MakeUpAction ||
		left.ReleaseCoachSlot != right.ReleaseCoachSlot ||
		left.Reason != right.Reason {
		return false
	}
	if left.CutoffAt == nil || right.CutoffAt == nil {
		return left.CutoffAt == nil && right.CutoffAt == nil
	}
	return left.CutoffAt.Equal(*right.CutoffAt)
}

func sameMakeUpObligation(left, right *MakeUpObligation) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func cloneCancellation(record CancellationRecord) CancellationRecord {
	cloned := record
	if record.Decision.CutoffAt != nil {
		cutoffAt := *record.Decision.CutoffAt
		cloned.Decision.CutoffAt = &cutoffAt
	}
	if record.MakeUpObligation != nil {
		obligation := *record.MakeUpObligation
		cloned.MakeUpObligation = &obligation
	}
	return cloned
}

func cloneCompletion(record CompletionRecord) CompletionRecord {
	cloned := record
	if record.Input.Cancellation != nil {
		decision := *record.Input.Cancellation
		if decision.CutoffAt != nil {
			cutoffAt := *decision.CutoffAt
			decision.CutoffAt = &cutoffAt
		}
		cloned.Input.Cancellation = &decision
	}
	return cloned
}
