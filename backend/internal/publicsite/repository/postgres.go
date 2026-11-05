package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/publicsite/model"
)

type Postgres struct{ pool *pgxpool.Pool }

func NewPostgres(pool *pgxpool.Pool) *Postgres { return &Postgres{pool: pool} }

func (p *Postgres) Organization(ctx context.Context) (model.Organization, error) {
	var value model.Organization
	var social []byte
	err := p.pool.QueryRow(ctx, `
		SELECT name, COALESCE(legal_name, ''), COALESCE(email, ''), COALESCE(phone, ''),
		       COALESCE(whatsapp, ''), COALESCE(address, ''), canonical_url, social_links, updated_at
		FROM organization_settings WHERE singleton = true`,
	).Scan(&value.Name, &value.LegalName, &value.Email, &value.Phone, &value.WhatsApp, &value.Address, &value.CanonicalURL, &social, &value.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Organization{}, ErrNotFound
	}
	if err != nil {
		return model.Organization{}, fmt.Errorf("public repository: selecting organization: %w", err)
	}
	if err := json.Unmarshal(social, &value.SocialLinks); err != nil {
		return model.Organization{}, fmt.Errorf("public repository: decoding social links: %w", err)
	}
	return value, nil
}

func (p *Postgres) Policies(ctx context.Context) ([]model.PolicySummary, error) {
	rows, err := p.pool.Query(ctx, `
		SELECT DISTINCT ON (p.slug) p.slug, p.title, v.version, v.effective_at, v.purpose
		FROM policies p
		JOIN policy_versions v ON v.policy_id = p.id
		WHERE v.status = 'published' AND v.effective_at <= now()
		ORDER BY p.slug, v.effective_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("public repository: listing policies: %w", err)
	}
	defer rows.Close()
	items := make([]model.PolicySummary, 0)
	for rows.Next() {
		var item model.PolicySummary
		if err := rows.Scan(&item.Slug, &item.Title, &item.Version, &item.EffectiveAt, &item.Purpose); err != nil {
			return nil, fmt.Errorf("public repository: scanning policy: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (p *Postgres) PolicyBySlug(ctx context.Context, slug string) (model.Policy, error) {
	var value model.Policy
	err := p.pool.QueryRow(ctx, `
		SELECT p.slug, p.title, v.version, v.effective_at, v.purpose,
		       v.content_html, encode(v.content_hash, 'hex'), v.locale
		FROM policies p
		JOIN policy_versions v ON v.policy_id = p.id
		WHERE p.slug = $1 AND v.status = 'published' AND v.effective_at <= now()
		ORDER BY v.effective_at DESC LIMIT 1`, slug,
	).Scan(&value.Slug, &value.Title, &value.Version, &value.EffectiveAt, &value.Purpose, &value.ContentHTML, &value.ContentHash, &value.Locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Policy{}, ErrNotFound
	}
	if err != nil {
		return model.Policy{}, fmt.Errorf("public repository: selecting policy: %w", err)
	}
	return value, nil
}

func (p *Postgres) PolicyVersion(ctx context.Context, slug, version string) (model.Policy, error) {
	var value model.Policy
	err := p.pool.QueryRow(ctx, `
		SELECT p.slug, p.title, v.version, v.effective_at, v.purpose,
		       v.content_html, encode(v.content_hash, 'hex'), v.locale
		FROM policies p
		JOIN policy_versions v ON v.policy_id = p.id
		WHERE p.slug = $1 AND v.version = $2 AND v.status = 'published'
		LIMIT 1`, slug, version,
	).Scan(&value.Slug, &value.Title, &value.Version, &value.EffectiveAt, &value.Purpose, &value.ContentHTML, &value.ContentHash, &value.Locale)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Policy{}, ErrNotFound
	}
	if err != nil {
		return model.Policy{}, fmt.Errorf("public repository: selecting policy version: %w", err)
	}
	return value, nil
}

func (p *Postgres) Programs(ctx context.Context) ([]model.Program, error) {
	return queryJSONRows[model.Program](ctx, p.pool, `SELECT public_json FROM programs WHERE status = 'active' ORDER BY display_order, slug`)
}

func (p *Postgres) Coaches(ctx context.Context) ([]model.Coach, error) {
	return queryJSONRows[model.Coach](ctx, p.pool, `
		SELECT public_json FROM coach_profiles
		WHERE status = 'active' AND is_public = true AND verified_at IS NOT NULL
		ORDER BY display_order, slug`)
}

func (p *Postgres) Curriculum(ctx context.Context) (model.Curriculum, error) {
	var raw []byte
	err := p.pool.QueryRow(ctx, `
		SELECT public_json FROM curriculum_versions
		WHERE status = 'published' AND effective_at <= now()
		ORDER BY effective_at DESC LIMIT 1`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Curriculum{Tracks: []model.Track{}}, nil
	}
	if err != nil {
		return model.Curriculum{}, fmt.Errorf("public repository: selecting curriculum: %w", err)
	}
	var value model.Curriculum
	if err := json.Unmarshal(raw, &value); err != nil {
		return model.Curriculum{}, fmt.Errorf("public repository: decoding curriculum: %w", err)
	}
	return value, nil
}

func (p *Postgres) Prices(ctx context.Context, currency string) ([]model.Price, error) {
	query := `SELECT public_json FROM plans WHERE status = 'active' AND currency = $1 ORDER BY display_order, name`
	if strings.TrimSpace(currency) == "" {
		currency = "INR"
	}
	return queryJSONRows[model.Price](ctx, p.pool, query, strings.ToUpper(currency))
}

type jsonQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryJSONRows[T any](ctx context.Context, db jsonQuerier, query string, args ...any) ([]T, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("public repository: querying json rows: %w", err)
	}
	defer rows.Close()
	items := make([]T, 0)
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("public repository: scanning json: %w", err)
		}
		var item T
		if err := json.Unmarshal(raw, &item); err != nil {
			return nil, fmt.Errorf("public repository: decoding json: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("public repository: iterating json: %w", err)
	}
	return items, nil
}
