package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/app"
	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
	"github.com/kingsgambit2700/platform/backend/internal/platform/database"
	"github.com/kingsgambit2700/platform/backend/internal/platform/logging"
	"github.com/kingsgambit2700/platform/backend/internal/platform/ratelimit"
	"github.com/kingsgambit2700/platform/backend/internal/provider"
	"github.com/kingsgambit2700/platform/backend/internal/provider/fake"
	"github.com/kingsgambit2700/platform/backend/internal/provider/resend"
	smtpprovider "github.com/kingsgambit2700/platform/backend/internal/provider/smtp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		fatal("invalid configuration", err)
	}
	logger := logging.New(os.Stdout, cfg.LogLevel, cfg.Environment != "development")
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		fatal("database unavailable", err)
	}
	defer pool.Close()
	limiter, err := ratelimit.NewRedis(cfg.RedisURL)
	if err != nil {
		fatal("redis rate limiter unavailable", err)
	}
	defer func() {
		if err := limiter.Close(); err != nil {
			logger.Error("redis rate limiter close failed", "error", err)
		}
	}()

	email, err := emailProvider(cfg)
	if err != nil {
		fatal("email provider unavailable", err)
	}
	application, err := app.New(cfg, pool, logger, email, limiter)
	if err != nil {
		fatal("application initialization failed", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- application.HTTP.Listen(cfg.APIAddr) }()
	logger.Info("api listening", "address", cfg.APIAddr)

	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped", "error", err)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := application.HTTP.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("api shutdown failed", "error", err)
	}
}

func emailProvider(cfg config.Config) (provider.EmailSender, error) {
	switch cfg.EmailProvider {
	case "fake":
		return &fake.Email{}, nil
	case "smtp":
		return smtpprovider.New(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPFrom)
	default:
		return resend.New(cfg.ResendAPIKey, cfg.ResendFrom, nil)
	}
}

func fatal(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
