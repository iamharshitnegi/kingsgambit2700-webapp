// local-passwords provisions passwords only for the seven local test accounts.
package main

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/auth/password"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	databaseURL := os.Getenv("KG_DATABASE_DIRECT_URL")
	target, err := url.Parse(databaseURL)
	if err != nil || os.Getenv("KG_ENV") != "development" ||
		(target.Hostname() != "localhost" && target.Hostname() != "127.0.0.1" && target.Hostname() != "::1") || target.Path != "/kg2700" {
		return fmt.Errorf("requires KG_ENV=development and the loopback kg2700 database")
	}
	value := os.Getenv("KG_LOCAL_TEST_PASSWORD")
	if !password.Valid(value) {
		return fmt.Errorf("set KG_LOCAL_TEST_PASSWORD to a 12–128 character local test password")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	usernames := []string{"guardian", "student", "coach", "academics", "operations", "finance", "admin"}
	err = pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
		for _, username := range usernames {
			var userID string
			if err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE email = $1 AND status = 'active' FOR UPDATE`, username+"@example.test").Scan(&userID); err != nil {
				return fmt.Errorf("finding local account %s: %w", username, err)
			}
			hash, err := password.Hash(value)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO auth_password_credentials (user_id, username, password_hash, updated_at)
				VALUES ($1::uuid, $2, $3, now()) ON CONFLICT (user_id) DO UPDATE
				SET username = EXCLUDED.username, password_hash = EXCLUDED.password_hash, updated_at = EXCLUDED.updated_at`, userID, username, hash)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE auth_devices SET revoked_at=now() WHERE user_id=$1::uuid AND revoked_at IS NULL`, userID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at = now() WHERE user_id = $1::uuid AND revoked_at IS NULL`, userID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO audit_events (id, occurred_at, actor_type, action, object_type, object_id, reason_code)
				VALUES ($1::uuid, now(), 'local_cli', 'local_password_set', 'user', $2, 'local_development')`, platformid.New(), userID)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("Password login enabled for the seven local accounts:")
	for _, username := range usernames {
		fmt.Printf("%s\t%s@example.test\n", username, username)
	}
	return nil
}
