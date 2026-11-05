package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
	"github.com/kingsgambit2700/platform/backend/internal/platform/database"
	"github.com/kingsgambit2700/platform/backend/internal/platform/jobs"
	"github.com/kingsgambit2700/platform/backend/internal/platform/logging"
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
	email, err := emailProvider(cfg)
	if err != nil {
		fatal("email provider unavailable", err)
	}
	client, err := jobs.NewClient(pool, logger, email, cfg.PublicWebURL, cfg.EmailProvider)
	if err != nil {
		fatal("worker initialization failed", err)
	}
	if err := client.Start(ctx); err != nil {
		fatal("worker start failed (ensure River migrations are applied)", err)
	}
	logger.Info("worker started")
	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Stop(shutdownCtx); err != nil {
		logger.Error("worker shutdown failed", "error", err)
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
