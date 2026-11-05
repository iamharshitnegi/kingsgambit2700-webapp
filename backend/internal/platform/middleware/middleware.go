package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/platform/metrics"
)

func Recover(logger *slog.Logger) fiber.Handler {
	return func(c fiber.Ctx) (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.ErrorContext(c.Context(), "request panic",
					"request_id", httpx.RequestID(c),
					"panic", recovered,
					"stack", string(debug.Stack()),
				)
				err = httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusInternalServerError, "internal_error", "Internal server error", "The request could not be completed"))
			}
		}()
		return c.Next()
	}
}

func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := strings.TrimSpace(c.Get("X-Request-ID"))
		if id == "" || len(id) > 128 {
			id = secureID()
		}
		httpx.SetRequestID(c, id)
		c.Set("X-Request-ID", id)
		return c.Next()
	}
}

func RequestLog(logger *slog.Logger, registry *metrics.Registry) fiber.Handler {
	return func(c fiber.Ctx) error {
		started := time.Now()
		registry.IncInFlight()
		defer registry.DecInFlight()
		err := c.Next()
		duration := time.Since(started)
		route := c.Route().Path
		if route == "" {
			route = "unmatched"
		}
		registry.Observe(c.Method(), route, c.Response().StatusCode(), duration)
		logger.InfoContext(c.Context(), "http request",
			"request_id", httpx.RequestID(c),
			"method", c.Method(),
			"route", route,
			"status", c.Response().StatusCode(),
			"duration_ms", duration.Milliseconds(),
		)
		return err
	}
}

func SecurityHeaders() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		c.Set("Cache-Control", "no-store")
		return c.Next()
	}
}

func CredentialedCORS(allowed []string) fiber.Handler {
	allowlist := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		allowlist[origin] = struct{}{}
	}
	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")
		if origin == "" {
			return c.Next()
		}
		if _, ok := allowlist[origin]; !ok {
			return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusForbidden, "origin_forbidden", "Origin forbidden", "This origin is not allowed"))
		}
		c.Set("Access-Control-Allow-Origin", origin)
		c.Set("Access-Control-Allow-Credentials", "true")
		c.Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Set("Access-Control-Allow-Headers", "Content-Type,Idempotency-Key,X-CSRF-Token,X-Request-ID")
		c.Set("Access-Control-Expose-Headers", "X-Request-ID,Location")
		c.Set("Vary", "Origin")
		if c.Method() == fiber.MethodOptions {
			return c.SendStatus(fiber.StatusNoContent)
		}
		return c.Next()
	}
}

// RequireSafeOrigin rejects browser-style unsafe requests without an exact
// approved Origin. The optional prefixes are reserved for authenticated
// server-to-server receivers (for example, signature-verified webhooks) whose
// providers do not send browser Origin headers.
func RequireSafeOrigin(allowed []string, serverToServerPrefixes ...string) fiber.Handler {
	allowlist := make(map[string]struct{}, len(allowed))
	for _, origin := range allowed {
		allowlist[origin] = struct{}{}
	}
	return func(c fiber.Ctx) error {
		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions:
			return c.Next()
		}
		for _, prefix := range serverToServerPrefixes {
			if prefix != "" && strings.HasPrefix(c.Path(), prefix) {
				return c.Next()
			}
		}
		origin := c.Get("Origin")
		if _, ok := allowlist[origin]; !ok {
			return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusForbidden, "origin_forbidden", "Origin forbidden", "Unsafe requests require an approved origin"))
		}
		return c.Next()
	}
}

func secureID() string {
	value := make([]byte, 18)
	if _, err := rand.Read(value); err != nil {
		return time.Now().UTC().Format("20060102T150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(value)
}
