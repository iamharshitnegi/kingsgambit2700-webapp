package repository

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
)

type Repository interface {
	Save(context.Context, model.Receipt) (model.SaveResult, error)
}

type Postgres struct {
	pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// Save records a verified delivery exactly once. Replays with the same key
// must also have the same payload digest; an existing key is never overwritten.
func (p *Postgres) Save(ctx context.Context, receipt model.Receipt) (model.SaveResult, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return model.SaveResult{}, fmt.Errorf("webhook repository: beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var insertedID string
	err = tx.QueryRow(ctx, `
		INSERT INTO webhook_receipts
			(id, provider, provider_account, provider_event_id, payload_hash, received_at)
		VALUES ($1::uuid, $2, $3, $4, $5, $6)
		ON CONFLICT (provider, provider_account, provider_event_id) DO NOTHING
		RETURNING id::text`,
		receipt.ID,
		receipt.Provider,
		receipt.ProviderAccount,
		receipt.ProviderEventID,
		receipt.PayloadHash,
		receipt.ReceivedAt,
	).Scan(&insertedID)
	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return model.SaveResult{}, fmt.Errorf("webhook repository: committing insert: %w", err)
		}
		return model.SaveResult{Inserted: true, PayloadMatches: true}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return model.SaveResult{}, fmt.Errorf("webhook repository: inserting receipt: %w", err)
	}

	var existingHash []byte
	err = tx.QueryRow(ctx, `
		SELECT payload_hash
		FROM webhook_receipts
		WHERE provider = $1 AND provider_account = $2 AND provider_event_id = $3`,
		receipt.Provider,
		receipt.ProviderAccount,
		receipt.ProviderEventID,
	).Scan(&existingHash)
	if err != nil {
		return model.SaveResult{}, fmt.Errorf("webhook repository: selecting duplicate: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return model.SaveResult{}, fmt.Errorf("webhook repository: committing duplicate check: %w", err)
	}
	return model.SaveResult{
		Inserted:       false,
		PayloadMatches: subtle.ConstantTimeCompare(existingHash, receipt.PayloadHash) == 1,
	}, nil
}
