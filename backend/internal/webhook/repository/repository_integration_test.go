//go:build integration

package repository

import (
	"bytes"
	"os"
	"testing"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
)

func TestPostgres_Save(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	repository := NewPostgres(pool)
	receivedAt := time.Now().UTC().Truncate(time.Microsecond)
	receipt := model.Receipt{
		ID:              platformid.New(),
		Provider:        model.ProviderRazorpay,
		ProviderAccount: "acc_integration",
		ProviderEventID: "evt_integration",
		PayloadHash:     bytes.Repeat([]byte{1}, 32),
		ReceivedAt:      receivedAt,
	}

	result, err := repository.Save(t.Context(), receipt)
	if err != nil {
		t.Fatalf("first Save() error = %v", err)
	}
	if !result.Inserted || !result.PayloadMatches {
		t.Fatalf("first Save() result = %+v", result)
	}

	receipt.ID = platformid.New()
	result, err = repository.Save(t.Context(), receipt)
	if err != nil {
		t.Fatalf("duplicate Save() error = %v", err)
	}
	if result.Inserted || !result.PayloadMatches {
		t.Fatalf("duplicate Save() result = %+v", result)
	}

	receipt.ID = platformid.New()
	receipt.PayloadHash = bytes.Repeat([]byte{2}, 32)
	result, err = repository.Save(t.Context(), receipt)
	if err != nil {
		t.Fatalf("conflicting Save() error = %v", err)
	}
	if result.Inserted || result.PayloadMatches {
		t.Fatalf("conflicting Save() result = %+v", result)
	}

	var count int
	var storedHash []byte
	var storedAt time.Time
	err = pool.QueryRow(t.Context(), `
		SELECT payload_hash, received_at,
		       (SELECT count(*) FROM webhook_receipts
		        WHERE provider = $1 AND provider_account = $2 AND provider_event_id = $3)
		FROM webhook_receipts
		WHERE provider = $1 AND provider_account = $2 AND provider_event_id = $3`,
		receipt.Provider,
		receipt.ProviderAccount,
		receipt.ProviderEventID,
	).Scan(&storedHash, &storedAt, &count)
	if err != nil {
		t.Fatalf("selecting receipt: %v", err)
	}
	if count != 1 || !bytes.Equal(storedHash, bytes.Repeat([]byte{1}, 32)) || !storedAt.Equal(receivedAt) {
		t.Fatalf("stored count=%d hash=%x receivedAt=%v", count, storedHash, storedAt)
	}
}
