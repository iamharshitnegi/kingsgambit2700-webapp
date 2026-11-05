package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/lead/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Repository interface {
	Upsert(context.Context, model.Lead, model.Attribution) (model.Lead, bool, error)
}

type Memory struct {
	mu    sync.Mutex
	items map[string]model.Lead
}

func NewMemory() *Memory { return &Memory{items: make(map[string]model.Lead)} }

func (m *Memory) Upsert(_ context.Context, lead model.Lead, _ model.Attribution) (model.Lead, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, existing := range m.items {
		if (lead.Email != "" && existing.Email == lead.Email) || (lead.Phone != "" && existing.Phone == lead.Phone) {
			existing.UpdatedAt = lead.UpdatedAt
			existing.ContactConsent = lead.ContactConsent
			existing.MarketingConsent = lead.MarketingConsent
			m.items[id] = existing
			return existing, false, nil
		}
	}
	m.items[lead.ID] = lead
	return lead, true, nil
}

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) Upsert(ctx context.Context, lead model.Lead, attribution model.Attribution) (model.Lead, bool, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return model.Lead{}, false, fmt.Errorf("lead repository: beginning transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM leads
		WHERE ($1 <> '' AND normalized_email = $1)
		   OR ($2 <> '' AND normalized_phone = $2)
		ORDER BY created_at
		LIMIT 1
		FOR UPDATE`, lead.Email, lead.Phone).Scan(&existingID)
	created := false
	switch {
	case err == nil:
		lead.ID = existingID
		row := tx.QueryRow(ctx, `
			UPDATE leads
			SET guardian_name = $2, student_age_band = $3, experience = $4, rating = $5,
			    country = $6, time_zone = $7, preferred_format = $8, preferred_times = $9,
			    contact_consent = contact_consent OR $10,
			    marketing_consent = marketing_consent OR $11,
			    updated_at = $12
			WHERE id = $1::uuid
			RETURNING id::text, guardian_name, COALESCE(normalized_email, ''), COALESCE(normalized_phone, ''),
			          student_age_band, COALESCE(experience, ''), rating, country, time_zone,
			          COALESCE(preferred_format, ''), preferred_times, contact_consent,
			          marketing_consent, status, created_at, updated_at`,
			lead.ID, lead.GuardianName, lead.StudentAgeBand, lead.Experience, lead.Rating,
			lead.Country, lead.TimeZone, lead.PreferredFormat, lead.PreferredTimes,
			lead.ContactConsent, lead.MarketingConsent, lead.UpdatedAt,
		)
		err = (leadRow{scannable: row}).ScanLead(&lead)
	case errors.Is(err, pgx.ErrNoRows):
		created = true
		_, err = tx.Exec(ctx, `
			INSERT INTO leads
				(id, guardian_name, normalized_email, normalized_phone, student_age_band,
				 experience, rating, country, time_zone, preferred_format, preferred_times,
				 contact_consent, marketing_consent, status, created_at, updated_at)
			VALUES ($1::uuid, $2, NULLIF($3, ''), NULLIF($4, ''), $5, NULLIF($6, ''), $7,
			        $8, $9, NULLIF($10, ''), $11, $12, $13, 'new', $14, $14)`,
			lead.ID, lead.GuardianName, lead.Email, lead.Phone, lead.StudentAgeBand,
			lead.Experience, lead.Rating, lead.Country, lead.TimeZone, lead.PreferredFormat,
			lead.PreferredTimes, lead.ContactConsent, lead.MarketingConsent, lead.CreatedAt,
		)
	default:
		return model.Lead{}, false, fmt.Errorf("lead repository: finding duplicate: %w", err)
	}
	if err != nil {
		return model.Lead{}, false, fmt.Errorf("lead repository: writing lead: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lead_stage_events (id, lead_id, stage, source, occurred_at)
		VALUES ($1::uuid, $2::uuid, $3, 'public_form', $4)`, platformid.New(), lead.ID, lead.Status, lead.UpdatedAt); err != nil {
		return model.Lead{}, false, fmt.Errorf("lead repository: inserting stage event: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO campaign_touches
			(id, lead_id, source, campaign, medium, term, content, landing_page, referrer, occurred_at)
		VALUES ($1::uuid, $2::uuid, NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''),
		        NULLIF($6, ''), NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10)`,
		platformid.New(), lead.ID, attribution.Source, attribution.Campaign, attribution.Medium,
		attribution.Term, attribution.Content, attribution.LandingPage, attribution.Referrer,
		attribution.OccurredAt,
	); err != nil {
		return model.Lead{}, false, fmt.Errorf("lead repository: inserting attribution: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.Lead{}, false, fmt.Errorf("lead repository: committing: %w", err)
	}
	return lead, created, nil
}

type scannable interface{ Scan(...any) error }

type leadRow struct{ scannable }

func (r leadRow) ScanLead(lead *model.Lead) error {
	return r.Scan(
		&lead.ID, &lead.GuardianName, &lead.Email, &lead.Phone, &lead.StudentAgeBand,
		&lead.Experience, &lead.Rating, &lead.Country, &lead.TimeZone,
		&lead.PreferredFormat, &lead.PreferredTimes, &lead.ContactConsent,
		&lead.MarketingConsent, &lead.Status, &lead.CreatedAt, &lead.UpdatedAt,
	)
}
