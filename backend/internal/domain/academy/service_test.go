package academy

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOccurrenceStarts(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	base := ClassInput{BatchID: "01990000-0000-7000-8000-000000000001", Title: "Tactics", LocalStart: "2027-03-21T10:00", TimeZone: "Europe/London", DurationMinutes: 60, Occurrences: 3}
	tests := []struct {
		name      string
		change    func(*ClassInput)
		wantError bool
	}{
		{"weekly local time across DST", func(*ClassInput) {}, false},
		{"nonexistent spring time", func(in *ClassInput) { in.LocalStart = "2027-03-28T01:30" }, true},
		{"ambiguous autumn time", func(in *ClassInput) { in.LocalStart = "2026-10-25T01:30" }, true},
		{"past class", func(in *ClassInput) { in.LocalStart = "2026-08-30T10:00" }, true},
		{"unsafe meeting scheme", func(in *ClassInput) { in.MeetingURL = "javascript:alert(1)" }, true},
		{"excessive recurrence", func(in *ClassInput) { in.Occurrences = 53 }, true},
		{"end date before start", func(in *ClassInput) { in.RecurringUntil = "2027-03-20" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.change(&in)
			starts, err := OccurrenceStarts(in, now)
			if tt.wantError {
				if !errors.Is(err, ErrInvalid) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(starts) != 3 {
				t.Fatal(starts)
			}
			if starts[1].Sub(starts[0]) != 167*time.Hour {
				t.Fatalf("DST interval=%v", starts[1].Sub(starts[0]))
			}
			zone, _ := time.LoadLocation(in.TimeZone)
			for _, start := range starts {
				if start.In(zone).Hour() != 10 {
					t.Fatal("wall-clock time drifted")
				}
			}
		})
	}
}
func TestOccurrenceStarts_ExplicitEndDate(t *testing.T) {
	in := ClassInput{BatchID: "01990000-0000-7000-8000-000000000001", Title: "Endgames", LocalStart: "2026-10-01T18:30", TimeZone: "Asia/Kolkata", DurationMinutes: 60, Occurrences: 1, RecurringUntil: "2026-10-29"}
	starts, err := OccurrenceStarts(in, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(starts) != 5 {
		t.Fatalf("occurrences=%d", len(starts))
	}
}

type batchValidationRepository struct {
	Repository
	input BatchInput
}

func (r *batchValidationRepository) CreateBatch(_ context.Context, _ string, in BatchInput, _ time.Time) (Batch, error) {
	r.input = in
	return Batch{Name: in.Name, CoachID: in.CoachID}, nil
}

func TestCreateBatchWithExistingCoach(t *testing.T) {
	repo := &batchValidationRepository{}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	input := BatchInput{Name: "Batch", CoachID: "b47c153f-1877-4992-8d41-d8b46cbd0f7b", Format: "group", Level: "foundation-2", TimeZone: "UTC", Capacity: 4}
	if _, err = service.CreateBatch(context.Background(), "01990000-0000-7000-8000-000000000001", input); err != nil {
		t.Fatalf("valid existing coach rejected: %v", err)
	}
	if repo.input != input {
		t.Fatal("batch details changed before persistence")
	}
}
