package app

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
)

// unsafeMethodsOnly keeps cookie-authenticated reads usable while applying a
// second middleware to state-changing requests on a path-scoped route group.
func unsafeMethodsOnly(middleware fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		switch c.Method() {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return c.Next()
		default:
			return middleware(c)
		}
	}
}
