package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/ratelimit"
)

func TestRateLimitRoutes(t *testing.T) {
	t.Parallel()

	limiter := &stubLimiter{decision: ratelimit.Decision{RetryAfter: 45 * time.Second}}
	app := fiber.New()
	app.Use(RateLimitRoutes(limiter, map[string]RateRule{
		"POST /limited": {Name: "limited", Limit: 2, Window: time.Minute},
	}))
	app.Post("/limited", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	app.Get("/limited", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/limited", nil))
	if err != nil {
		t.Fatalf("limited request: %v", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("limited status = %d", response.StatusCode)
	}
	if got := response.Header.Get("Retry-After"); got != "45" {
		t.Fatalf("Retry-After = %q", got)
	}
	if limiter.calls != 1 {
		t.Fatalf("limiter calls = %d", limiter.calls)
	}

	bypass, err := app.Test(httptest.NewRequest(http.MethodGet, "/limited", nil))
	if err != nil {
		t.Fatalf("bypass request: %v", err)
	}
	defer bypass.Body.Close()
	_, _ = io.Copy(io.Discard, bypass.Body)
	if bypass.StatusCode != fiber.StatusNoContent {
		t.Fatalf("bypass status = %d", bypass.StatusCode)
	}
	if limiter.calls != 1 {
		t.Fatalf("bypass invoked limiter; calls = %d", limiter.calls)
	}
}

type stubLimiter struct {
	decision ratelimit.Decision
	calls    int
}

func (s *stubLimiter) Take(context.Context, string, int64, time.Duration) (ratelimit.Decision, error) {
	s.calls++
	return s.decision, nil
}

func (s *stubLimiter) Ping(context.Context) error { return nil }
