//go:build integration

package integrationtest

import (
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
)

// OpenMigratedSchema creates an isolated schema, applies every application up
// migration in order, and drops the schema after the caller's test. It avoids
// weakening append-only production triggers just to clean integration data.
func OpenMigratedSchema(t testing.TB, databaseURL string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	admin, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("opening integration admin pool: %v", err)
	}
	t.Cleanup(admin.Close)
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("pinging integration database: %v", err)
	}

	schema := "kg_it_" + strings.ReplaceAll(platformid.New(), "-", "")
	identifier := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+identifier); err != nil {
		t.Fatalf("creating integration schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+identifier+" CASCADE"); err != nil {
			t.Errorf("dropping integration schema: %v", err)
		}
	})

	migrationConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parsing integration database URL: %v", err)
	}
	migrationConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	migrationConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+identifier+", public")
		return err
	}
	migrationPool, err := pgxpool.NewWithConfig(ctx, migrationConfig)
	if err != nil {
		t.Fatalf("opening integration schema pool: %v", err)
	}

	_, source, _, _ := runtime.Caller(0)
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", "migrations"))
	migrations, err := filepath.Glob(filepath.Join(migrationDir, "*.up.sql"))
	if err != nil {
		t.Fatalf("listing integration migrations: %v", err)
	}
	sort.Strings(migrations)
	if len(migrations) == 0 {
		t.Fatal("no integration migrations found")
	}
	for _, migration := range migrations {
		script, err := os.ReadFile(migration)
		if err != nil {
			t.Fatalf("reading %s: %v", filepath.Base(migration), err)
		}
		if _, err := migrationPool.Exec(ctx, string(script)); err != nil {
			migrationPool.Close()
			t.Fatalf("applying %s: %v", filepath.Base(migration), err)
		}
	}
	migrationPool.Close()

	applicationConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parsing integration application database URL: %v", err)
	}
	applicationConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET search_path TO "+identifier+", public")
		return err
	}
	applicationPool, err := pgxpool.NewWithConfig(ctx, applicationConfig)
	if err != nil {
		t.Fatalf("opening integration application pool: %v", err)
	}
	t.Cleanup(applicationPool.Close)
	return applicationPool
}
