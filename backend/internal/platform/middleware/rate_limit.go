package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/platform/ratelimit"
)

type RateRule struct {
	Name   string
	Limit  int64
	Window time.Duration
}

// RateLimitRoutes applies fixed-window Redis limits to exact method/path keys.
// Keys contain only a digest of the peer address, never the raw address.
func RateLimitRoutes(limiter ratelimit.Limiter, rules map[string]RateRule) fiber.Handler {
	return func(c fiber.Ctx) error {
		rule, ok := rules[c.Method()+" "+c.Path()]
		if !ok {
			return c.Next()
		}
		identity := sha256.Sum256([]byte(c.IP()))
		decision, err := limiter.Take(
			c.Context(),
			rule.Name+":"+hex.EncodeToString(identity[:]),
			rule.Limit,
			rule.Window,
		)
		if err != nil {
			return httpx.WriteProblem(c, httpx.NewProblem(
				fiber.StatusServiceUnavailable,
				"rate_limit_unavailable",
				"Request protection unavailable",
				"Please retry shortly",
			))
		}
		if decision.Allowed {
			return c.Next()
		}
		seconds := int64((decision.RetryAfter + time.Second - 1) / time.Second)
		c.Set("Retry-After", strconv.FormatInt(max(seconds, 1), 10))
		return httpx.WriteProblem(c, httpx.NewProblem(
			fiber.StatusTooManyRequests,
			"rate_limited",
			"Too many requests",
			"Wait before retrying this request",
		))
	}
}
