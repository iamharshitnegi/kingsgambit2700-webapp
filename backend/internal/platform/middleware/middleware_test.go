package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRequireSafeOrigin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		origin     string
		wantStatus int
	}{
		{
			name:       "approved browser mutation",
			path:       "/api/v1/families/example",
			origin:     "https://kingsgambit2700.in",
			wantStatus: fiber.StatusNoContent,
		},
		{
			name:       "browser mutation without origin",
			path:       "/api/v1/families/example",
			wantStatus: fiber.StatusForbidden,
		},
		{
			name:       "server webhook without origin",
			path:       "/api/v1/webhooks/razorpay",
			wantStatus: fiber.StatusNoContent,
		},
		{
			name:       "lookalike path is not exempt",
			path:       "/api/v1/webhooks-attacker/razorpay",
			wantStatus: fiber.StatusForbidden,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			app := fiber.New()
			app.Use(RequireSafeOrigin(
				[]string{"https://kingsgambit2700.in"},
				"/api/v1/webhooks/",
			))
			app.Post(test.path, func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
		})
	}
}
