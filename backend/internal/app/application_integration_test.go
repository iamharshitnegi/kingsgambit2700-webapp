//go:build integration

package app

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/config"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
	"github.com/kingsgambit2700/platform/backend/internal/platform/ratelimit"
	"github.com/kingsgambit2700/platform/backend/internal/provider/fake"
)

func TestApplicationRouteBoundaries(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	application, err := New(config.Config{
		Environment:           "test",
		S3Region:              "us-east-1",
		S3BucketQuarantine:    "test-quarantine",
		S3BucketClean:         "test-clean",
		S3AccessKeyID:         "test",
		S3SecretAccessKey:     "test",
		PublicAPIURL:          "https://api.example.test",
		PublicWebURL:          "https://www.example.test",
		AllowedOrigins:        []string{"https://www.example.test"},
		CookieName:            "kg_session",
		MagicLinkTTL:          15 * time.Minute,
		SessionIdleTTL:        30 * time.Minute,
		SessionAbsoluteTTL:    7 * 24 * time.Hour,
		CSRFTTL:               30 * time.Minute,
		TOTPEncryptionKey:     "bG9jYWwta2cyNzAwLXRvdHAta2V5LW1hdGVyaWFsLTE=",
		MFAChallengeTTL:       5 * time.Minute,
		MFARecentTTL:          5 * time.Minute,
		RazorpayWebhookSecret: "test-razorpay-secret",
		ResendWebhookSecret:   "whsec_bG9jYWwtcmVzZW5kLXdlYmhvb2stc2VjcmV0LTAwMDE=",
		DailyWebhookSecret:    "bG9jYWwtZGFpbHktd2ViaG9vay1zZWNyZXQ=",
	}, pool, logger, &fake.Email{}, allowLimiter{})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		origin     string
		body       string
		headers    map[string]string
		wantStatus int
	}{
		{
			name:       "public organization bypasses auth",
			method:     http.MethodGet,
			path:       "/api/v1/public/organization",
			wantStatus: fiber.StatusOK,
		},
		{
			name:       "unknown route remains a not found problem",
			method:     http.MethodGet,
			path:       "/api/v1/not-a-route",
			wantStatus: fiber.StatusNotFound,
		},
		{
			name:       "family mutation is authenticated",
			method:     http.MethodPost,
			path:       "/api/v1/families/0198f8c4-0c80-7000-8000-000000000001/guardian-relationships",
			origin:     "https://www.example.test",
			body:       `{}`,
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:       "entitlement read is authenticated",
			method:     http.MethodGet,
			path:       "/api/v1/entitlements/accounts/0198f8c4-0c80-7000-8000-000000000001",
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:       "entitlement grant is authenticated before validation",
			method:     http.MethodPost,
			path:       "/api/v1/entitlements/accounts/0198f8c4-0c80-7000-8000-000000000001/grants",
			origin:     "https://www.example.test",
			body:       `{}`,
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:       "support read is authenticated",
			method:     http.MethodGet,
			path:       "/api/v1/support/cases?familyId=0198f8c4-0c80-7000-8000-000000000001",
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:       "support mutation is authenticated before validation",
			method:     http.MethodPost,
			path:       "/api/v1/support/cases",
			origin:     "https://www.example.test",
			body:       `{}`,
			wantStatus: fiber.StatusUnauthorized,
		},
		{
			name:   "signed webhook boundary bypasses browser origin only",
			method: http.MethodPost,
			path:   "/api/v1/webhooks/razorpay",
			body:   `{}`,
			headers: map[string]string{
				"X-Razorpay-Event-Id":  "evt_route_test",
				"X-Razorpay-Signature": "invalid",
			},
			wantStatus: fiber.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}
			response, err := application.HTTP.Test(request)
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

type allowLimiter struct{}

func (allowLimiter) Take(context.Context, string, int64, time.Duration) (ratelimit.Decision, error) {
	return ratelimit.Decision{Allowed: true}, nil
}

func (allowLimiter) Ping(context.Context) error { return nil }
