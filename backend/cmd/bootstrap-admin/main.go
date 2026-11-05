// bootstrap-admin is a one-off, audited first-super-admin initializer. It has
// no default identity or credential and refuses to run after any super-admin
// grant has ever existed.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/mail"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func main() {
	var databaseURL, email, displayName string
	var dryRun bool
	flag.StringVar(&databaseURL, "database-url", os.Getenv("KG_DATABASE_DIRECT_URL"), "direct PostgreSQL URL override (discouraged: visible in process listings; prefer KG_DATABASE_DIRECT_URL)")
	flag.StringVar(&email, "email", "", "first super-admin email (required)")
	flag.StringVar(&displayName, "display-name", "", "first super-admin display name (required)")
	flag.BoolVar(&dryRun, "dry-run", false, "validate and check eligibility without writing")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: KG_DATABASE_DIRECT_URL=... %s -email ADDRESS -display-name NAME [-dry-run]\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)
	if databaseURL == "" || email == "" || displayName == "" {
		flag.Usage()
		os.Exit(2)
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		fail("email must be a normalized mailbox address", nil)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fail("open database", err)
	}
	defer pool.Close()
	if err := bootstrap(ctx, pool, email, displayName, dryRun); err != nil {
		fail("bootstrap refused", err)
	}
	if dryRun {
		fmt.Fprintln(os.Stdout, "dry-run passed: no super-admin exists and no rows were written")
		return
	}
	fmt.Fprintln(os.Stdout, "first super-admin created and audited")
}

func bootstrap(ctx context.Context, pool *pgxpool.Pool, email, displayName string, dryRun bool) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('kg:first-super-admin'))`); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM role_grants WHERE role = 'super_admin')`).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("a super_admin grant already exists")
	}
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE email = $1)`, email).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return errors.New("the requested email already belongs to a user; bootstrap never elevates existing users")
	}
	if dryRun {
		return nil
	}
	now := time.Now().UTC()
	userID := platformid.New()
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, email, display_name, status, created_at, updated_at)
		VALUES ($1::uuid, $2, $3, 'active', $4, $4)`, userID, email, displayName, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO role_grants (id, user_id, role, granted_at)
		VALUES ($1::uuid, $2::uuid, 'super_admin', $3)`, platformid.New(), userID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO audit_events
			(id, occurred_at, actor_type, action, object_type, object_id, reason_code, metadata)
		VALUES ($1::uuid, $2, 'bootstrap_cli', 'bootstrap_super_admin', 'user', $3,
		        'first_super_admin', jsonb_build_object('email', $4::text))`,
		platformid.New(), now, userID, email); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func fail(message string, err error) {
	if err == nil {
		fmt.Fprintln(os.Stderr, message)
	} else {
		fmt.Fprintf(os.Stderr, "%s: %v\n", message, err)
	}
	os.Exit(1)
}
