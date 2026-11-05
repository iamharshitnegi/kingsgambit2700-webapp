package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
	authcontroller "github.com/kingsgambit2700/platform/backend/internal/auth/controller"
	authrepository "github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/authorization"
	"github.com/kingsgambit2700/platform/backend/internal/domain/academy"
	academycontroller "github.com/kingsgambit2700/platform/backend/internal/domain/academy/controller"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	entitlementcontroller "github.com/kingsgambit2700/platform/backend/internal/domain/entitlement/controller"
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	familycontroller "github.com/kingsgambit2700/platform/backend/internal/domain/family/controller"
	"github.com/kingsgambit2700/platform/backend/internal/domain/learning"
	learningcontroller "github.com/kingsgambit2700/platform/backend/internal/domain/learning/controller"
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	reportcontroller "github.com/kingsgambit2700/platform/backend/internal/domain/report/controller"
	"github.com/kingsgambit2700/platform/backend/internal/domain/support"
	supportcontroller "github.com/kingsgambit2700/platform/backend/internal/domain/support/controller"
	leadcontroller "github.com/kingsgambit2700/platform/backend/internal/lead/controller"
	leadrepository "github.com/kingsgambit2700/platform/backend/internal/lead/repository"
	leadservice "github.com/kingsgambit2700/platform/backend/internal/lead/service"
	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
	platformcontroller "github.com/kingsgambit2700/platform/backend/internal/platform/controller"
	"github.com/kingsgambit2700/platform/backend/internal/platform/health"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/metrics"
	platformmiddleware "github.com/kingsgambit2700/platform/backend/internal/platform/middleware"
	"github.com/kingsgambit2700/platform/backend/internal/platform/ratelimit"
	"github.com/kingsgambit2700/platform/backend/internal/provider"
	"github.com/kingsgambit2700/platform/backend/internal/provider/learningfiles"
	publiccontroller "github.com/kingsgambit2700/platform/backend/internal/publicsite/controller"
	publicrepository "github.com/kingsgambit2700/platform/backend/internal/publicsite/repository"
	publicservice "github.com/kingsgambit2700/platform/backend/internal/publicsite/service"
	"github.com/kingsgambit2700/platform/backend/internal/realtime/ticket"
	trialcontroller "github.com/kingsgambit2700/platform/backend/internal/trial/controller"
	trialrepository "github.com/kingsgambit2700/platform/backend/internal/trial/repository"
	trialservice "github.com/kingsgambit2700/platform/backend/internal/trial/service"
	webhookcontroller "github.com/kingsgambit2700/platform/backend/internal/webhook/controller"
	webhookmodel "github.com/kingsgambit2700/platform/backend/internal/webhook/model"
	webhookrepository "github.com/kingsgambit2700/platform/backend/internal/webhook/repository"
	webhookservice "github.com/kingsgambit2700/platform/backend/internal/webhook/service"
)

type Application struct {
	HTTP *fiber.App
}

func New(
	cfg config.Config,
	pool *pgxpool.Pool,
	logger *slog.Logger,
	email provider.EmailSender,
	limiter ratelimit.Limiter,
) (*Application, error) {
	if limiter == nil {
		return nil, errors.New("application: rate limiter is required")
	}
	registry := metrics.New()
	readiness := health.New()
	readiness.Add("postgres", func(ctx context.Context) error { return pool.Ping(ctx) })
	readiness.Add("redis", limiter.Ping)

	httpApp := fiber.New(platformmiddleware.ConfigureTrustedProxies(fiber.Config{
		AppName:        "KingsGambit2700 API",
		BodyLimit:      21 << 20,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   30 * time.Second,
		IdleTimeout:    60 * time.Second,
		Concurrency:    64 * 1024,
		ReadBufferSize: 16 * 1024,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return httpx.WriteProblem(c, err)
		},
	}, cfg.TrustedProxies))
	httpApp.Use(platformmiddleware.RequestID())
	httpApp.Use(func(c fiber.Ctx) error {
		if !(c.Method() == fiber.MethodPost && c.Path() == "/api/v1/learning/files") && len(c.Body()) > 1<<20 {
			return httpx.WriteProblem(c, httpx.NewProblem(413, "body_too_large", "Request too large", "JSON requests must be smaller than 1 MB"))
		}
		return c.Next()
	})
	httpApp.Use(platformmiddleware.Recover(logger))
	httpApp.Use(platformmiddleware.RequestLog(logger, registry))
	httpApp.Use(platformmiddleware.SecurityHeaders())
	httpApp.Use(platformmiddleware.CredentialedCORS(cfg.AllowedOrigins))
	httpApp.Use(platformmiddleware.RequireSafeOrigin(
		cfg.AllowedOrigins,
		"/api/v1/webhooks/",
	))

	platformcontroller.NewOperational(readiness, registry).Register(httpApp)
	api := httpApp.Group("/api/v1")
	api.Use(platformmiddleware.RateLimitRoutes(limiter, map[string]platformmiddleware.RateRule{
		"POST /api/v1/auth/password/login":    {Name: "auth_password_login", Limit: 20, Window: 15 * time.Minute},
		"POST /api/v1/auth/password/register": {Name: "auth_password_register", Limit: 5, Window: 15 * time.Minute},
		"GET /api/v1/auth/google/config":      {Name: "auth_google_config", Limit: 30, Window: 15 * time.Minute},
		"POST /api/v1/auth/google":            {Name: "auth_google_login", Limit: 20, Window: 15 * time.Minute},
		"POST /api/v1/auth/magic-links": {
			Name: "auth_magic_link", Limit: 5, Window: 15 * time.Minute,
		},
		"POST /api/v1/auth/student-codes/exchange": {
			Name: "auth_student_code_exchange", Limit: 10, Window: 10 * time.Minute,
		},
		"POST /api/v1/auth/mfa/totp/enrollment/confirm": {
			Name: "auth_mfa_enrollment_confirm", Limit: 10, Window: 10 * time.Minute,
		},
		"POST /api/v1/auth/mfa/challenge": {
			Name: "auth_mfa_challenge", Limit: 10, Window: 10 * time.Minute,
		},
		"POST /api/v1/public/leads": {
			Name: "public_lead", Limit: 10, Window: time.Hour,
		},
		"POST /api/v1/public/trial-bookings": {
			Name: "public_trial_booking", Limit: 10, Window: time.Hour,
		},
		"POST /api/v1/realtime/join-tickets": {
			Name: "realtime_join_ticket", Limit: 60, Window: time.Minute,
		},
		"POST /api/v1/support/cases": {
			Name: "support_case_create", Limit: 20, Window: time.Hour,
		},
	}))

	authorizer, err := authorization.NewPostgres(pool)
	if err != nil {
		return nil, fmt.Errorf("creating authorizer: %w", err)
	}

	authRepo := authrepository.NewPostgres(pool)
	authService, err := authservice.New(authRepo, authservice.NewProviderEmailSender(email), authservice.Config{
		GoogleClientID:     cfg.GoogleClientID,
		PublicAPIURL:       cfg.PublicAPIURL,
		PublicWebURL:       cfg.PublicWebURL,
		MagicLinkTTL:       cfg.MagicLinkTTL,
		SessionIdleTTL:     cfg.SessionIdleTTL,
		SessionAbsoluteTTL: cfg.SessionAbsoluteTTL,
		CSRFTTL:            cfg.CSRFTTL,
		TOTPEncryptionKey:  cfg.TOTPEncryptionKey,
		MFAChallengeTTL:    cfg.MFAChallengeTTL,
		MFARecentTTL:       cfg.MFARecentTTL,
		TOTPIssuer:         "KingsGambit2700",
	}, authservice.WithMFAResetAuthorizer(authorizer))
	if err != nil {
		return nil, err
	}
	authController := authcontroller.New(authService, authcontroller.Config{
		CookieName:     cfg.CookieName,
		CookieSecure:   cfg.CookieSecure,
		CookieTTL:      cfg.SessionAbsoluteTTL,
		PublicWebURL:   cfg.PublicWebURL,
		AllowedOrigins: cfg.AllowedOrigins,
	})
	authController.Register(api)
	ticket.NewController(ticket.NewService(pool)).Register(api, authController.RequireAuth, authController.RequireCSRF)

	razorpayVerifier, err := webhookservice.NewRazorpayVerifier(cfg.RazorpayWebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("creating razorpay webhook verifier: %w", err)
	}
	resendVerifier, err := webhookservice.NewResendVerifier(
		cfg.ResendWebhookSecret,
		5*time.Minute,
	)
	if err != nil {
		return nil, fmt.Errorf("creating resend webhook verifier: %w", err)
	}
	dailyVerifier, err := webhookservice.NewDailyVerifier(cfg.DailyWebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("creating daily webhook verifier: %w", err)
	}
	webhookService, err := webhookservice.New(webhookservice.Dependencies{
		Repository: webhookrepository.NewPostgres(pool),
		Verifiers: map[webhookmodel.Provider]webhookservice.Verifier{
			webhookmodel.ProviderRazorpay: razorpayVerifier,
			webhookmodel.ProviderResend:   resendVerifier,
			webhookmodel.ProviderDaily:    dailyVerifier,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating webhook service: %w", err)
	}
	webhookController, err := webhookcontroller.New(webhookService, 256<<10)
	if err != nil {
		return nil, fmt.Errorf("creating webhook controller: %w", err)
	}
	webhookController.Register(api)

	familyRepository, err := family.NewPostgresRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("creating family repository: %w", err)
	}
	familyService, err := family.NewService(familyRepository, authorizer)
	if err != nil {
		return nil, fmt.Errorf("creating family service: %w", err)
	}
	familyController, err := familycontroller.New(familycontroller.Dependencies{
		Service: familyService,
		Actors:  principalActorResolver{},
		NewID:   platformid.New,
		Now:     time.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("creating family controller: %w", err)
	}

	reportRepository, err := report.NewPostgresRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("creating report repository: %w", err)
	}
	reportService, err := report.NewService(reportRepository, authorizer)
	if err != nil {
		return nil, fmt.Errorf("creating report service: %w", err)
	}
	reportController, err := reportcontroller.New(reportcontroller.Dependencies{
		Service: reportService,
		Actors:  principalActorResolver{},
		NewID:   platformid.New,
		Now:     time.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("creating report controller: %w", err)
	}

	entitlementRepository, err := entitlement.NewPostgresRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("creating entitlement repository: %w", err)
	}
	entitlementService, err := entitlement.NewService(
		entitlementRepository,
		entitlement.WithAuthorizer(authorizer),
	)
	if err != nil {
		return nil, fmt.Errorf("creating entitlement service: %w", err)
	}
	entitlementController, err := entitlementcontroller.New(entitlementcontroller.Dependencies{
		Service: entitlementService,
		Actors:  principalActorResolver{},
		NewID:   platformid.New,
		Now:     time.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("creating entitlement controller: %w", err)
	}

	supportRepository, err := support.NewPostgresRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("creating support repository: %w", err)
	}
	supportService, err := support.NewService(
		supportRepository,
		authorizer,
		support.DefaultConfig(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating support service: %w", err)
	}
	supportController, err := supportcontroller.New(supportcontroller.Dependencies{
		Service: supportService,
		Actors:  principalActorResolver{},
		NewID:   platformid.New,
		Now:     time.Now,
	})
	if err != nil {
		return nil, fmt.Errorf("creating support controller: %w", err)
	}

	publiccontroller.New(publicservice.New(publicrepository.NewPostgres(pool))).Register(api)
	leadcontroller.New(leadservice.New(leadrepository.NewPostgres(pool))).Register(api)
	trialcontroller.New(trialservice.New(trialrepository.NewPostgres(pool))).Register(api)

	// Keep authorization middleware path-scoped. An empty-prefix Fiber group
	// would also intercept public and unknown API routes registered after it.
	api.Use("/families", authController.RequireAuth, authController.RequireCSRF)
	familyController.Register(api)
	api.Use("/reports", authController.RequireAuth, authController.RequireCSRF)
	reportController.Register(api)
	api.Use(
		"/entitlements",
		authController.RequireAuth,
		unsafeMethodsOnly(authController.RequireCSRF),
		unsafeMethodsOnly(authController.RequireRecentMFA),
	)
	entitlementController.Register(api)
	api.Use(
		"/support",
		authController.RequireAuth,
		unsafeMethodsOnly(authController.RequireCSRF),
	)
	supportController.Register(api)
	academyRepository, err := academy.NewPostgresRepository(pool)
	if err != nil {
		return nil, fmt.Errorf("creating academy repository: %w", err)
	}
	academyService, err := academy.NewService(academyRepository)
	if err != nil {
		return nil, fmt.Errorf("creating academy service: %w", err)
	}
	academyController, err := academycontroller.New(academyService, principalActorResolver{})
	if err != nil {
		return nil, fmt.Errorf("creating academy controller: %w", err)
	}
	api.Use("/academy", authController.RequireAuth, unsafeMethodsOnly(authController.RequireCSRF))
	for _, path := range []string{"/academy/staff", "/academy/policies", "/academy/settings"} {
		api.Use(path, unsafeMethodsOnly(authController.RequireRecentMFA))
	}
	academyController.Register(api)
	learningService, err := learning.NewService(pool)
	if err != nil {
		return nil, fmt.Errorf("creating learning service: %w", err)
	}
	fileStore, err := learningfiles.NewStore(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("creating learning file store: %w", err)
	}
	if err := learningService.ConfigureFiles(fileStore, learningfiles.Scanner{Address: cfg.ClamAVAddr}, cfg.UploadMaxBytes); err != nil {
		return nil, fmt.Errorf("configuring learning uploads: %w", err)
	}
	learningController, err := learningcontroller.New(learningService, principalActorResolver{})
	if err != nil {
		return nil, fmt.Errorf("creating learning controller: %w", err)
	}
	api.Use("/learning", authController.RequireAuth, unsafeMethodsOnly(authController.RequireCSRF))
	learningController.Register(api)

	httpApp.Use(func(c fiber.Ctx) error {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusNotFound, "not_found", "Not found", "No route matches this request"))
	})
	return &Application{HTTP: httpApp}, nil
}
