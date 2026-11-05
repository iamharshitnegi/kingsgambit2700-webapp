package entitlement

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestService_ReservedConsumptionIsIdempotent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := NewMemoryRepository()
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)

	_, err = service.Grant(ctx, OperationCommand{
		EntryID:        "entry-grant",
		AccountID:      "account-1",
		StudentID:      "student-1",
		IdempotencyKey: "grant:payment-1",
		ReferenceID:    "payment-1",
		Units:          3,
		Reason:         "initial package grant",
		ActorID:        "finance-1",
		OccurredAt:     now,
	})
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	_, err = service.Reserve(ctx, OperationCommand{
		EntryID:        "entry-reserve",
		AccountID:      "account-1",
		StudentID:      "student-1",
		IdempotencyKey: "reserve:session-1",
		ReferenceID:    "session-1",
		Units:          1,
		Reason:         "class seat reserved",
		ActorID:        "system",
		OccurredAt:     now.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}

	command := ConsumeCommand{
		OperationCommand: OperationCommand{
			EntryID:        "entry-consume",
			AccountID:      "account-1",
			StudentID:      "student-1",
			IdempotencyKey: "consume:session-1:student-1",
			ReferenceID:    "session-1",
			Units:          1,
			Reason:         "completed class",
			ActorID:        "system",
			OccurredAt:     now.Add(2 * time.Minute),
		},
		Source: ConsumptionSourceReserved,
	}
	first, err := service.Consume(ctx, command)
	if err != nil {
		t.Fatalf("Consume(first) error = %v", err)
	}
	if !first.Created {
		t.Fatal("Consume(first) Created = false, want true")
	}

	command.EntryID = "entry-consume-retry"
	command.OccurredAt = now.Add(3 * time.Minute)
	second, err := service.Consume(ctx, command)
	if err != nil {
		t.Fatalf("Consume(retry) error = %v", err)
	}
	if second.Created {
		t.Fatal("Consume(retry) Created = true, want false")
	}
	if second.Entry.ID != first.Entry.ID {
		t.Fatalf("Consume(retry) entry id = %q, want %q", second.Entry.ID, first.Entry.ID)
	}

	balance, err := repository.Balance(ctx, "account-1")
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if balance != (Balance{Available: 2, Reserved: 0, Consumed: 1}) {
		t.Fatalf("Balance() = %+v, want available 2, reserved 0, consumed 1", balance)
	}
	entries, err := repository.Entries(ctx, "account-1")
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("Entries() length = %d, want 3", len(entries))
	}
}

func TestService_RejectsIdempotencyKeyReuse(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, err := NewService(NewMemoryRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	command := OperationCommand{
		EntryID:        "entry-1",
		AccountID:      "account-1",
		StudentID:      "student-1",
		IdempotencyKey: "grant:manual-1",
		ReferenceID:    "manual-1",
		Units:          2,
		Reason:         "approved manual grant",
		ActorID:        "finance-1",
		OccurredAt:     now,
	}
	if _, err := service.Grant(ctx, command); err != nil {
		t.Fatalf("Grant(first) error = %v", err)
	}

	command.EntryID = "entry-2"
	command.Units = 3
	_, err = service.Grant(ctx, command)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("Grant(conflict) error = %v, want %v", err, ErrIdempotencyConflict)
	}
}

func TestService_PreventsNegativeBalance(t *testing.T) {
	t.Parallel()

	service, err := NewService(NewMemoryRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	_, err = service.Consume(context.Background(), ConsumeCommand{
		OperationCommand: OperationCommand{
			EntryID:        "entry-consume",
			AccountID:      "account-1",
			StudentID:      "student-1",
			IdempotencyKey: "consume:session-1",
			ReferenceID:    "session-1",
			Units:          1,
			Reason:         "late cancellation",
			ActorID:        "system",
			OccurredAt:     now,
		},
		Source: ConsumptionSourceAvailable,
	})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("Consume() error = %v, want %v", err, ErrInsufficientBalance)
	}
}

func TestService_ReleaseRestoresReservedCredit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	service, err := NewService(NewMemoryRepository())
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	base := OperationCommand{
		AccountID:   "account-1",
		StudentID:   "student-1",
		ReferenceID: "session-1",
		Units:       1,
		ActorID:     "system",
		OccurredAt:  now,
	}
	grant := base
	grant.EntryID = "entry-grant"
	grant.IdempotencyKey = "grant:payment-1"
	grant.ReferenceID = "payment-1"
	grant.Reason = "package grant"
	if _, err := service.Grant(ctx, grant); err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	reserve := base
	reserve.EntryID = "entry-reserve"
	reserve.IdempotencyKey = "reserve:session-1"
	reserve.Reason = "class reserved"
	if _, err := service.Reserve(ctx, reserve); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	release := base
	release.EntryID = "entry-release"
	release.IdempotencyKey = "release:session-1"
	release.Reason = "cancelled before cutoff"
	result, err := service.Release(ctx, release)
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}
	if result.Balance != (Balance{Available: 1}) {
		t.Fatalf("Release() balance = %+v, want available 1", result.Balance)
	}
}

func TestMemoryRepository_RejectsAccountOwnerChangeAndOverflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := NewMemoryRepository()
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	grant := OperationCommand{
		EntryID:        "entry-1",
		AccountID:      "account-1",
		StudentID:      "student-1",
		IdempotencyKey: "grant:1",
		ReferenceID:    "payment-1",
		Units:          1,
		Reason:         "package grant",
		ActorID:        "finance-1",
		OccurredAt:     now,
	}
	if _, err := service.Grant(ctx, grant); err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	grant.EntryID = "entry-2"
	grant.StudentID = "student-2"
	grant.IdempotencyKey = "grant:2"
	if _, err := service.Grant(ctx, grant); !errors.Is(err, ErrEntryConflict) {
		t.Fatalf("Grant(different student) error = %v, want %v", err, ErrEntryConflict)
	}

	overflowRepository := NewMemoryRepository()
	overflowService, err := NewService(overflowRepository)
	if err != nil {
		t.Fatalf("NewService(overflow) error = %v", err)
	}
	grant.EntryID = "entry-max"
	grant.AccountID = "account-max"
	grant.StudentID = "student-1"
	grant.IdempotencyKey = "grant:max"
	grant.Units = int64(^uint64(0) >> 1)
	if _, err := overflowService.Grant(ctx, grant); err != nil {
		t.Fatalf("Grant(max) error = %v", err)
	}
	grant.EntryID = "entry-overflow"
	grant.IdempotencyKey = "grant:overflow"
	grant.Units = 1
	if _, err := overflowService.Grant(ctx, grant); !errors.Is(err, ErrBalanceOverflow) {
		t.Fatalf("Grant(overflow) error = %v, want %v", err, ErrBalanceOverflow)
	}
}

func TestMemoryRepository_ConcurrentConsumptionAppendsOnce(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repository := NewMemoryRepository()
	service, err := NewService(repository)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	if _, err := service.Grant(ctx, OperationCommand{
		EntryID:        "entry-grant",
		AccountID:      "account-1",
		StudentID:      "student-1",
		IdempotencyKey: "grant:payment-1",
		ReferenceID:    "payment-1",
		Units:          1,
		Reason:         "package grant",
		ActorID:        "finance-1",
		OccurredAt:     now,
	}); err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	const attempts = 16
	results := make(chan ApplyResult, attempts)
	errorsFound := make(chan error, attempts)
	var waitGroup sync.WaitGroup
	for index := range attempts {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			result, err := service.Consume(ctx, ConsumeCommand{
				OperationCommand: OperationCommand{
					EntryID:        fmt.Sprintf("entry-consume-%d", index),
					AccountID:      "account-1",
					StudentID:      "student-1",
					IdempotencyKey: "consume:session-1:student-1",
					ReferenceID:    "session-1",
					Units:          1,
					Reason:         "completed class",
					ActorID:        "system",
					OccurredAt:     now.Add(time.Minute),
				},
				Source: ConsumptionSourceAvailable,
			})
			if err != nil {
				errorsFound <- err
				return
			}
			results <- result
		}()
	}
	waitGroup.Wait()
	close(results)
	close(errorsFound)

	for err := range errorsFound {
		t.Errorf("Consume() concurrent error = %v", err)
	}
	created := 0
	for result := range results {
		if result.Created {
			created++
		}
	}
	if created != 1 {
		t.Fatalf("created consumption entries = %d, want 1", created)
	}
	entries, err := repository.Entries(ctx, "account-1")
	if err != nil {
		t.Fatalf("Entries() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("Entries() length = %d, want grant plus one consumption", len(entries))
	}
}
