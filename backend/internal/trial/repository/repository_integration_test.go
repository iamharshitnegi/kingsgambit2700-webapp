//go:build integration

package repository

import (
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
	"github.com/kingsgambit2700/platform/backend/internal/trial/model"
)

func trialDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL is not set")
	}
	return integrationtest.OpenMigratedSchema(t, databaseURL)
}

func trialFixture(t *testing.T, pool *pgxpool.Pool, start time.Time, capacity int) model.BookingInput {
	t.Helper()
	input := model.BookingInput{
		SlotID: platformid.New(), LeadID: platformid.New(), GuardianName: "Parent",
		Email: platformid.New() + "@example.com", StudentAgeBand: "8_12", TimeZone: "UTC",
		ContactConsent: true, IdempotencyKey: platformid.New(),
	}
	_, err := pool.Exec(t.Context(), `
		INSERT INTO leads (id, guardian_name, normalized_email, student_age_band, country, time_zone,
		                   contact_consent, status, created_at, updated_at)
		VALUES ($1::uuid, 'Parent', $2, '8_12', 'IN', 'UTC', true, 'new', now(), now())`, input.LeadID, input.Email)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(t.Context(), `
		INSERT INTO trial_slots (id, starts_at, ends_at, source_time_zone, format, age_bands,
		                         capacity, status, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, 'UTC', 'group', ARRAY['8_12'], $4, 'published', now(), now())`,
		input.SlotID, start, start.Add(time.Hour), capacity)
	if err != nil {
		t.Fatal(err)
	}
	return input
}

func trialBooking(input model.BookingInput) model.Booking {
	return model.Booking{ID: platformid.New(), SlotID: input.SlotID, LeadID: input.LeadID, Status: "confirmed", TimeZone: input.TimeZone, CreatedAt: time.Now().UTC()}
}

func TestPostgresConcurrentTrialRetriesPreserveOneBooking(t *testing.T) {
	pool := trialDatabase(t)
	repo := NewPostgres(pool)
	input := trialFixture(t, pool, time.Now().Add(time.Hour), 1)
	type result struct {
		booking model.Booking
		err     error
	}
	results := make(chan result, 8)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			<-start
			booking, err := repo.CreateBooking(t.Context(), input, trialBooking(input))
			results <- result{booking, err}
		})
	}
	close(start)
	workers.Wait()
	close(results)
	var bookingID string
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent retry failed: %v", result.err)
		}
		if bookingID != "" && result.booking.ID != bookingID {
			t.Fatalf("retry created a second booking: %s, %s", bookingID, result.booking.ID)
		}
		bookingID = result.booking.ID
	}
	input.StudentName = "Changed child"
	if _, err := repo.CreateBooking(t.Context(), input, trialBooking(input)); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed retry error = %v, want conflict", err)
	}
}

func TestPostgresConcurrentBookingsRespectCapacity(t *testing.T) {
	pool := trialDatabase(t)
	repo := NewPostgres(pool)
	input := trialFixture(t, pool, time.Now().Add(time.Hour), 2)
	results := make(chan error, 8)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			request := input
			request.IdempotencyKey = platformid.New()
			<-start
			_, err := repo.CreateBooking(t.Context(), request, trialBooking(request))
			results <- err
		})
	}
	close(start)
	workers.Wait()
	close(results)
	confirmed := 0
	for err := range results {
		if err == nil {
			confirmed++
		} else if !errors.Is(err, ErrFull) {
			t.Fatalf("booking returned unexpected error instead of full: %v", err)
		}
	}
	if confirmed != 2 {
		t.Fatalf("confirmed = %d, want capacity 2", confirmed)
	}
}

func TestPostgresTrialRejectsPastAndIncompatibleSlots(t *testing.T) {
	pool := trialDatabase(t)
	repo := NewPostgres(pool)
	past := trialFixture(t, pool, time.Now().Add(-time.Hour), 2)
	if _, err := repo.CreateBooking(t.Context(), past, trialBooking(past)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("past slot error = %v, want unavailable", err)
	}
	incompatible := trialFixture(t, pool, time.Now().Add(time.Hour), 2)
	incompatible.StudentAgeBand = "13_17"
	if _, err := repo.CreateBooking(t.Context(), incompatible, trialBooking(incompatible)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("incompatible slot error = %v, want unavailable", err)
	}
}
