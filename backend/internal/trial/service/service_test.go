package service

import (
	"context"
	"errors"
	"testing"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/trial/model"
	"github.com/kingsgambit2700/platform/backend/internal/trial/repository"
)

func TestBookIsIdempotentAndCapacitySafe(t *testing.T) {
	start := time.Now().UTC().Add(time.Hour)
	slot := model.Slot{ID: platformid.New(), StartsAt: start, EndsAt: start.Add(time.Hour), TimeZone: "Asia/Kolkata", Format: "group", AvailableSeats: 1}
	svc := New(repository.NewMemory([]model.Slot{slot}))
	input := model.BookingInput{
		SlotID: slot.ID, LeadID: platformid.New(), GuardianName: "Parent",
		Email: "parent@example.com", StudentAgeBand: "8_12", TimeZone: "Asia/Kolkata",
		ContactConsent: true, IdempotencyKey: "0123456789abcdef",
	}
	first, err := svc.Book(context.Background(), input)
	if err != nil {
		t.Fatalf("first Book() error = %v", err)
	}
	if !first.StartsAt.Equal(slot.StartsAt) || !first.EndsAt.Equal(slot.EndsAt) {
		t.Fatalf("booking times = %v–%v, want %v–%v", first.StartsAt, first.EndsAt, slot.StartsAt, slot.EndsAt)
	}
	second, err := svc.Book(context.Background(), input)
	if err != nil || second.ID != first.ID {
		t.Fatalf("idempotent Book() = (%q, %v), want %q", second.ID, err, first.ID)
	}
	input.IdempotencyKey = "fedcba9876543210"
	if _, err := svc.Book(context.Background(), input); !errors.Is(err, repository.ErrFull) {
		t.Fatalf("capacity Book() error = %v, want full", err)
	}
}

func TestBookRejectsChangedIdempotencyPayload(t *testing.T) {
	slot := model.Slot{ID: platformid.New(), StartsAt: time.Now().Add(time.Hour), AvailableSeats: 2}
	svc := New(repository.NewMemory([]model.Slot{slot}))
	input := model.BookingInput{
		SlotID: slot.ID, LeadID: platformid.New(), GuardianName: "Parent",
		Email: "parent@example.com", StudentAgeBand: "8_12", TimeZone: "UTC",
		ContactConsent: true, IdempotencyKey: "0123456789abcdef",
	}
	if _, err := svc.Book(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	input.Email = "different@example.com"
	if _, err := svc.Book(t.Context(), input); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("changed request error = %v, want conflict", err)
	}
}

func TestBookRejectsUnavailableOrIncompatibleSlots(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name     string
		start    time.Time
		ageBands []string
	}{
		{name: "past slot", start: now.Add(-time.Minute)},
		{name: "already starting", start: now},
		{name: "incompatible age", start: now.Add(time.Hour), ageBands: []string{"13_17"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			slot := model.Slot{ID: platformid.New(), StartsAt: test.start, AgeBands: test.ageBands, AvailableSeats: 2}
			svc := New(repository.NewMemory([]model.Slot{slot}))
			svc.now = func() time.Time { return now }
			_, err := svc.Book(t.Context(), model.BookingInput{
				SlotID: slot.ID, LeadID: platformid.New(), GuardianName: "Parent",
				Email: "parent@example.com", StudentAgeBand: "8_12", TimeZone: "UTC",
				ContactConsent: true, IdempotencyKey: "0123456789abcdef",
			})
			if !errors.Is(err, repository.ErrNotFound) {
				t.Fatalf("Book() error = %v, want unavailable slot", err)
			}
		})
	}
}

func TestBookRejectsMalformedIDsBeforeRepositoryAccess(t *testing.T) {
	svc := New(nil)
	_, err := svc.Book(t.Context(), model.BookingInput{
		SlotID: "bad-slot", LeadID: "bad-lead", GuardianName: "Parent",
		Email: "parent@example.com", StudentAgeBand: "8_12", TimeZone: "UTC",
		ContactConsent: true, IdempotencyKey: "0123456789abcdef",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Book() error = %v, want invalid input", err)
	}
}

func TestListFiltersAgeAndReturnsChronologicalSlots(t *testing.T) {
	now := time.Now().UTC()
	slots := []model.Slot{
		{ID: "later", StartsAt: now.Add(3 * time.Hour), AgeBands: []string{"8_12"}, AvailableSeats: 1},
		{ID: "wrong-age", StartsAt: now.Add(time.Hour), AgeBands: []string{"13_17"}, AvailableSeats: 1},
		{ID: "earlier", StartsAt: now.Add(2 * time.Hour), AvailableSeats: 1},
	}
	svc := New(repository.NewMemory(slots))
	items, err := svc.List(t.Context(), model.ListFilter{AgeBand: "8_12"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != "earlier" || items[1].ID != "later" {
		t.Fatalf("List() = %+v, want compatible chronological slots", items)
	}
}
