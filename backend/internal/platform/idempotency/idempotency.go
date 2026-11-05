// Package idempotency stores actor-scoped mutation receipts in their transaction.
package idempotency

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrConflict = errors.New("idempotency: request key was used with a different payload")

type contextKey struct{}

func WithKey(ctx context.Context, key string) context.Context {
	return context.WithValue(ctx, contextKey{}, key)
}
func Key(ctx context.Context) string { key, _ := ctx.Value(contextKey{}).(string); return key }

func Replay(ctx context.Context, tx pgx.Tx, actor, operation string, input, output any) (bool, error) {
	key := Key(ctx)
	if key == "" {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, actor+":"+operation+":"+key); err != nil {
		return false, err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return false, err
	}
	digest := sha256.Sum256(encoded)
	var old, body []byte
	err = tx.QueryRow(ctx, `SELECT request_digest,response FROM academy_request_results WHERE actor_id=$1::uuid AND operation=$2 AND idempotency_key=$3`, actor, operation, key).Scan(&old, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if string(old) != string(digest[:]) {
		return false, ErrConflict
	}
	return true, json.Unmarshal(body, output)
}
func Save(ctx context.Context, tx pgx.Tx, actor, operation string, input, output any) error {
	key := Key(ctx)
	if key == "" {
		return nil
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(encoded)
	body, err := json.Marshal(output)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO academy_request_results(actor_id,operation,idempotency_key,request_digest,response,created_at) VALUES($1::uuid,$2,$3,$4,$5,$6)`, actor, operation, key, digest[:], body, time.Now().UTC())
	return err
}
