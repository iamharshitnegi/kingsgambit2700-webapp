package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	databaseURL := os.Getenv("KG_DATABASE_DIRECT_URL")
	if databaseURL == "" {
		fatal("KG_DATABASE_DIRECT_URL is required", nil)
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		fatal("open direct database connection", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		fatal("database unavailable", err)
	}

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		fatal("initialize River migrator", err)
	}
	mode := "up"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	switch mode {
	case "up":
		result, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
		if err != nil {
			fatal("apply River migrations", err)
		}
		slog.Info("River migrations are current", "applied", len(result.Versions))
	case "validate":
		result, err := migrator.Validate(ctx, nil)
		if err != nil {
			fatal("validate River migrations", err)
		}
		if !result.OK {
			message := "River migrations are not current"
			if len(result.Messages) > 0 {
				message = result.Messages[0]
			}
			fatal("River migrations are not current", errors.New(message))
		}
		slog.Info("River migrations validated")
	default:
		fatal("usage: river-migrate [up|validate]", nil)
	}
}

func fatal(message string, err error) {
	if err == nil {
		slog.Error(message)
	} else {
		slog.Error(message, "error", err)
	}
	os.Exit(1)
}
