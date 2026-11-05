package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestUnsafeMethodsOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		method     string
		wantCalled bool
	}{
		{method: http.MethodGet, wantCalled: false},
		{method: http.MethodHead, wantCalled: false},
		{method: http.MethodOptions, wantCalled: false},
		{method: http.MethodPost, wantCalled: true},
		{method: http.MethodPatch, wantCalled: true},
		{method: http.MethodDelete, wantCalled: true},
	}

	for _, test := range tests {
		t.Run(test.method, func(t *testing.T) {
			t.Parallel()
			called := false
			application := fiber.New()
			application.Use(unsafeMethodsOnly(func(c fiber.Ctx) error {
				called = true
				return c.Next()
			}))
			application.Add([]string{test.method}, "/test", func(c fiber.Ctx) error {
				return c.SendStatus(fiber.StatusNoContent)
			})

			response, err := application.Test(httptest.NewRequest(test.method, "/test", nil))
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			response.Body.Close()
			if called != test.wantCalled {
				t.Fatalf("middleware called = %t, want %t", called, test.wantCalled)
			}
		})
	}
}
