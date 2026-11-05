//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	publicrepository "github.com/kingsgambit2700/platform/backend/internal/publicsite/repository"
)

func TestMigrationsAndPublicRepository(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("opening admin pool: %v", err)
	}
	defer admin.Close()
	schema := "kg_it_" + strings.ReplaceAll(platformid.New(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("creating test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = admin.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE")
	})

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parsing test database URL: %v", err)
	}
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+identifier+", public")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatalf("opening schema pool: %v", err)
	}
	defer pool.Close()

	_, source, _, _ := runtime.Caller(0)
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "migrations"))
	migrationFiles, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("listing migrations: %v", err)
	}
	sort.Strings(migrationFiles)
	if len(migrationFiles) == 0 {
		t.Fatal("no application migrations found")
	}
	for _, migrationFile := range migrationFiles {
		name := filepath.Base(migrationFile)
		script, err := os.ReadFile(migrationFile)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if _, err := pool.Exec(ctx, string(script)); err != nil {
			t.Fatalf("applying %s: %v", name, err)
		}
	}

	repository := publicrepository.NewPostgres(pool)
	organization, err := repository.Organization(ctx)
	if err != nil {
		t.Fatalf("loading seeded organization: %v", err)
	}
	if organization.Name != "KingsGambit2700" {
		t.Fatalf("organization name = %q", organization.Name)
	}
	programs, err := repository.Programs(ctx)
	if err != nil {
		t.Fatalf("loading seeded programs: %v", err)
	}
	if len(programs) != 2 {
		t.Fatalf("program count = %d, want 2", len(programs))
	}

	if _, err := pool.Exec(ctx, `
		UPDATE curriculum_versions
		SET public_json = '{"tampered":true}'::jsonb
		WHERE status = 'published'`); err == nil {
		t.Fatal("published curriculum version accepted an in-place update")
	}
	auditID := platformid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, action, object_type, object_id, metadata)
		VALUES ($1::uuid, now(), 'system', 'integration.created', 'integration', $1, '{}'::jsonb)`,
		auditID,
	); err != nil {
		t.Fatalf("inserting audit event: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE audit_events
		SET metadata = '{"tampered":true}'::jsonb
		WHERE id = $1::uuid`, auditID); err == nil {
		t.Fatal("append-only audit event accepted an update")
	}

	receiptID := platformid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO webhook_receipts
			(id, provider, provider_event_id, payload_hash, received_at)
		VALUES ($1::uuid, 'razorpay', 'evt_integrity', $2, now())`,
		receiptID,
		bytes.Repeat([]byte{1}, 32),
	); err != nil {
		t.Fatalf("inserting webhook receipt: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE webhook_receipts
		SET payload_hash = $2
		WHERE id = $1::uuid`, receiptID, bytes.Repeat([]byte{2}, 32)); err == nil {
		t.Fatal("append-only webhook receipt accepted an update")
	}
}
