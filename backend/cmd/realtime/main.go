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

	"github.com/gofiber/fiber/v3"
	authrepository "github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/domain/classroom"
	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
	"github.com/kingsgambit2700/platform/backend/internal/platform/database"
	"github.com/kingsgambit2700/platform/backend/internal/platform/logging"
	platformmiddleware "github.com/kingsgambit2700/platform/backend/internal/platform/middleware"
	"github.com/kingsgambit2700/platform/backend/internal/provider/fake"
	"github.com/kingsgambit2700/platform/backend/internal/realtime/broadcast"
	realtimecontroller "github.com/kingsgambit2700/platform/backend/internal/realtime/controller"
	"github.com/kingsgambit2700/platform/backend/internal/realtime/ticket"
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
	bus, err := broadcast.NewRedis(cfg.RedisURL)
	if err != nil {
		fatal("redis initialization failed", err)
	}
	defer bus.Close()
	repository, err := classroom.NewPostgresRepository(pool, classroom.DefaultSnapshotInterval)
	if err != nil {
		fatal("classroom repository initialization failed", err)
	}
	classroomService, err := classroom.NewService(repository, classroom.ChessRules{})
	if err != nil {
		fatal("classroom service initialization failed", err)
	}
	authService, err := authservice.New(
		authrepository.NewPostgres(pool),
		authservice.NewProviderEmailSender(&fake.Email{}),
		authservice.Config{
			PublicAPIURL:       cfg.PublicAPIURL,
			PublicWebURL:       cfg.PublicWebURL,
			MagicLinkTTL:       cfg.MagicLinkTTL,
			SessionIdleTTL:     cfg.SessionIdleTTL,
			SessionAbsoluteTTL: cfg.SessionAbsoluteTTL,
			CSRFTTL:            cfg.CSRFTTL,
		},
	)
	if err != nil {
		fatal("auth service initialization failed", err)
	}
	app := fiber.New(fiber.Config{
		AppName:        "KingsGambit2700 Realtime",
		BodyLimit:      64 << 10,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    75 * time.Second,
		Concurrency:    32 * 1024,
		ReadBufferSize: 16 * 1024,
	})
	app.Use(platformmiddleware.RequestID())
	app.Use(platformmiddleware.SecurityHeaders())
	controller := realtimecontroller.New(classroomService, authService, pool, bus, ticket.NewService(pool), logger, cfg.CookieName, cfg.AllowedOrigins)
	controller.Register(app)

	go func() {
		if err := controller.RunSubscriber(ctx); err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("realtime subscriber stopped", "error", err)
			stop()
		}
	}()
	serveErr := make(chan error, 1)
	go func() { serveErr <- app.Listen(cfg.RealtimeAddr) }()
	logger.Info("realtime listening", "address", cfg.RealtimeAddr)
	select {
	case <-ctx.Done():
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("realtime stopped", "error", err)
		}
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("realtime shutdown failed", "error", err)
	}
}

func fatal(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
