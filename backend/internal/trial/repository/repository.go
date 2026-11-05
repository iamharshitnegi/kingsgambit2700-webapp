package repository

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
	"github.com/kingsgambit2700/platform/backend/internal/trial/model"
)

var (
	ErrNotFound = errors.New("trial repository: not found")
	ErrFull     = errors.New("trial repository: slot full")
	ErrConflict = errors.New("trial repository: duplicate booking")
)

type Repository interface {
	ListAvailable(context.Context, model.ListFilter) ([]model.Slot, error)
	CreateBooking(context.Context, model.BookingInput, model.Booking) (model.Booking, error)
}

type Memory struct {
	mu       sync.Mutex
	slots    map[string]model.Slot
	bookings map[string]model.Booking
	keys     map[string]string
	inputs   map[string]model.BookingInput
}

func NewMemory(slots []model.Slot) *Memory {
	repo := &Memory{slots: make(map[string]model.Slot), bookings: make(map[string]model.Booking), keys: make(map[string]string), inputs: make(map[string]model.BookingInput)}
	for _, slot := range slots {
		repo.slots[slot.ID] = slot
	}
	return repo
}

func (m *Memory) ListAvailable(_ context.Context, filter model.ListFilter) ([]model.Slot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]model.Slot, 0)
	for _, slot := range m.slots {
		if !filter.From.IsZero() && slot.StartsAt.Before(filter.From) {
			continue
		}
		if !filter.To.IsZero() && !slot.StartsAt.Before(filter.To) {
			continue
		}
		if filter.Format != "" && slot.Format != filter.Format {
			continue
		}
		if filter.AgeBand != "" && len(slot.AgeBands) > 0 && !slices.Contains(slot.AgeBands, filter.AgeBand) {
			continue
		}
		if slot.AvailableSeats > 0 {
			items = append(items, slot)
		}
	}
	slices.SortFunc(items, func(a, b model.Slot) int { return a.StartsAt.Compare(b.StartsAt) })
	return items, nil
}

func (m *Memory) CreateBooking(_ context.Context, input model.BookingInput, booking model.Booking) (model.Booking, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.keys[input.IdempotencyKey]; ok {
		if m.inputs[input.IdempotencyKey] != input {
			return model.Booking{}, ErrConflict
		}
		return m.bookings[existing], nil
	}
	slot, ok := m.slots[input.SlotID]
	if !ok || !slot.StartsAt.After(booking.CreatedAt) || (len(slot.AgeBands) > 0 && !slices.Contains(slot.AgeBands, input.StudentAgeBand)) {
		return model.Booking{}, ErrNotFound
	}
	if slot.AvailableSeats <= 0 {
		return model.Booking{}, ErrFull
	}
	slot.AvailableSeats--
	booking.StartsAt = slot.StartsAt
	booking.EndsAt = slot.EndsAt
	m.slots[input.SlotID] = slot
	m.bookings[booking.ID] = booking
	m.keys[input.IdempotencyKey] = booking.ID
	m.inputs[input.IdempotencyKey] = input
	return booking, nil
}

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) ListAvailable(ctx context.Context, filter model.ListFilter) ([]model.Slot, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT s.id::text, s.starts_at, s.ends_at, s.source_time_zone, s.format,
		       s.age_bands, s.level_slugs,
		       s.capacity - COUNT(b.id) FILTER (WHERE b.status IN ('confirmed', 'attended')) AS available_seats
		FROM trial_slots s
		LEFT JOIN trial_bookings b ON b.slot_id = s.id
		WHERE s.status = 'published'
		  AND s.starts_at >= $1 AND s.starts_at < $2
		  AND ($3 = '' OR s.format = $3)
		  AND ($4 = '' OR cardinality(s.age_bands) = 0 OR $4 = ANY(s.age_bands))
		GROUP BY s.id
		HAVING s.capacity - COUNT(b.id) FILTER (WHERE b.status IN ('confirmed', 'attended')) > 0
		ORDER BY s.starts_at`, filter.From, filter.To, filter.Format, filter.AgeBand)
	if err != nil {
		return nil, fmt.Errorf("trial repository: listing slots: %w", err)
	}
	defer rows.Close()
	items := make([]model.Slot, 0)
	for rows.Next() {
		var slot model.Slot
		if err := rows.Scan(&slot.ID, &slot.StartsAt, &slot.EndsAt, &slot.TimeZone, &slot.Format, &slot.AgeBands, &slot.LevelSlugs, &slot.AvailableSeats); err != nil {
			return nil, fmt.Errorf("trial repository: scanning slot: %w", err)
		}
		items = append(items, slot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trial repository: iterating slots: %w", err)
	}
	return items, nil
}

func (p *Postgres) CreateBooking(ctx context.Context, input model.BookingInput, booking model.Booking) (model.Booking, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: beginning booking: %w", err)
	}
	defer tx.Rollback(ctx)
	// Serialize retries before reading their result; the slot row separately
	// serializes capacity checks across different idempotency keys.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, input.IdempotencyKey); err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: locking idempotency key: %w", err)
	}

	var existing model.Booking
	var matches bool
	err = tx.QueryRow(ctx, `
		SELECT id::text, slot_id::text, lead_id::text, status, starts_at, ends_at, display_time_zone, created_at,
		       slot_id = $2::uuid AND lead_id = $3::uuid AND guardian_name = $4
		       AND email = $5 AND COALESCE(phone, '') = $6 AND COALESCE(student_name, '') = $7
		       AND student_age_band = $8 AND display_time_zone = $9 AND contact_consent = $10
		FROM trial_bookings WHERE idempotency_key = $1`, input.IdempotencyKey,
		input.SlotID, input.LeadID, input.GuardianName, input.Email, input.Phone,
		input.StudentName, input.StudentAgeBand, input.TimeZone, input.ContactConsent,
	).Scan(&existing.ID, &existing.SlotID, &existing.LeadID, &existing.Status, &existing.StartsAt, &existing.EndsAt, &existing.TimeZone, &existing.CreatedAt, &matches)
	if err == nil {
		if !matches {
			return model.Booking{}, ErrConflict
		}
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.Booking{}, fmt.Errorf("trial repository: checking idempotency: %w", err)
	}

	var startsAt, endsAt time.Time
	var capacity, booked int
	err = tx.QueryRow(ctx, `
		SELECT starts_at, ends_at, capacity
		FROM trial_slots
		WHERE id = $1::uuid AND status = 'published' AND starts_at > clock_timestamp()
		  AND (cardinality(age_bands) = 0 OR $2 = ANY(age_bands))
		FOR UPDATE`, input.SlotID, input.StudentAgeBand).Scan(&startsAt, &endsAt, &capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Booking{}, ErrNotFound
	}
	if err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: locking slot: %w", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*) FROM trial_bookings
		WHERE slot_id = $1::uuid AND status IN ('confirmed', 'attended')`, input.SlotID).Scan(&booked); err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: counting bookings: %w", err)
	}
	if booked >= capacity {
		return model.Booking{}, ErrFull
	}
	booking.StartsAt = startsAt
	booking.EndsAt = endsAt
	_, err = tx.Exec(ctx, `
		INSERT INTO trial_bookings
			(id, slot_id, lead_id, guardian_name, email, phone, student_name, student_age_band,
			 display_time_zone, contact_consent, idempotency_key, status, starts_at, ends_at, created_at, updated_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8,
		        $9, $10, $11, 'confirmed', $12, $13, $14, $14)`,
		booking.ID, input.SlotID, input.LeadID, input.GuardianName, input.Email, input.Phone,
		input.StudentName, input.StudentAgeBand, input.TimeZone, input.ContactConsent,
		input.IdempotencyKey, startsAt, endsAt, booking.CreatedAt)
	if err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: inserting booking: %w", err)
	}
	if err := notification.EnqueueTrial(ctx, tx, booking.ID, "Academy trial confirmed", "trial-confirmed:"+booking.ID, booking.CreatedAt, true, ""); err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: queuing confirmation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Booking{}, fmt.Errorf("trial repository: committing booking: %w", err)
	}
	return booking, nil
}
