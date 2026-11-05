package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
	webhookservice "github.com/kingsgambit2700/platform/backend/internal/webhook/service"
)

func TestController_ReceivesRawProviderRequests(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		path              string
		headers           map[string]string
		expectedProvider  model.Provider
		expectedEventID   string
		expectedSignature string
		expectedTimestamp string
	}{
		{
			name: "razorpay headers",
			path: "/api/v1/webhooks/razorpay",
			headers: map[string]string{
				"X-Razorpay-Event-Id":  "evt_1",
				"X-Razorpay-Signature": "signature",
			},
			expectedProvider:  model.ProviderRazorpay,
			expectedEventID:   "evt_1",
			expectedSignature: "signature",
		},
		{
			name: "resend headers",
			path: "/api/v1/webhooks/resend",
			headers: map[string]string{
				"svix-id":        "msg_1",
				"svix-signature": "v1,signature",
				"svix-timestamp": "1800000000",
			},
			expectedProvider:  model.ProviderResend,
			expectedEventID:   "msg_1",
			expectedSignature: "v1,signature",
			expectedTimestamp: "1800000000",
		},
		{
			name: "daily headers",
			path: "/api/v1/webhooks/daily",
			headers: map[string]string{
				"X-Webhook-Signature": "signature",
				"X-Webhook-Timestamp": "1800000000",
			},
			expectedProvider:  model.ProviderDaily,
			expectedSignature: "signature",
			expectedTimestamp: "1800000000",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			receiver := &fakeReceiver{}
			app := newTestApp(t, receiver, 0)
			request := httptest.NewRequest(http.MethodPost, test.path, strings.NewReader("{ \"raw\": true }\n"))
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != fiber.StatusNoContent {
				t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
			}
			if string(receiver.body) != "{ \"raw\": true }\n" {
				t.Fatalf("body = %q", receiver.body)
			}
			if receiver.provider != test.expectedProvider ||
				receiver.headers.EventID != test.expectedEventID ||
				receiver.headers.Signature != test.expectedSignature ||
				receiver.headers.Timestamp != test.expectedTimestamp {
				t.Fatalf("received provider=%q headers=%+v", receiver.provider, receiver.headers)
			}
		})
	}
}

func TestController_Problems(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		receiverError  error
		maxBodyBytes   int
		body           string
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "body too large",
			maxBodyBytes:   2,
			body:           "{}\n",
			expectedStatus: fiber.StatusRequestEntityTooLarge,
			expectedCode:   "webhook_body_too_large",
		},
		{
			name:           "invalid signature",
			receiverError:  webhookservice.ErrInvalidSignature,
			body:           "{}",
			expectedStatus: fiber.StatusUnauthorized,
			expectedCode:   "webhook_signature_invalid",
		},
		{
			name:           "invalid payload",
			receiverError:  webhookservice.ErrInvalidPayload,
			body:           "{}",
			expectedStatus: fiber.StatusBadRequest,
			expectedCode:   "webhook_payload_invalid",
		},
		{
			name:           "storage unavailable",
			receiverError:  errors.New("database unavailable"),
			body:           "{}",
			expectedStatus: fiber.StatusServiceUnavailable,
			expectedCode:   "webhook_unavailable",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			receiver := &fakeReceiver{err: test.receiverError}
			app := newTestApp(t, receiver, test.maxBodyBytes)
			request := httptest.NewRequest(
				http.MethodPost,
				"/api/v1/webhooks/razorpay",
				strings.NewReader(test.body),
			)
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != test.expectedStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.expectedStatus)
			}
			var problem httpx.Problem
			if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
				t.Fatalf("decoding problem: %v", err)
			}
			if problem.Code != test.expectedCode || response.Header.Get("Content-Type") != "application/problem+json" {
				t.Fatalf("problem = %+v, content type = %q", problem, response.Header.Get("Content-Type"))
			}
		})
	}
}

func newTestApp(t *testing.T, receiver Receiver, maxBodyBytes int) *fiber.App {
	t.Helper()
	controller, err := New(receiver, maxBodyBytes)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app.Group("/api/v1"))
	return app
}

type fakeReceiver struct {
	provider model.Provider
	body     []byte
	headers  model.Headers
	err      error
}

func (r *fakeReceiver) Ingest(
	_ context.Context,
	provider model.Provider,
	body []byte,
	headers model.Headers,
) (model.IngestResult, error) {
	r.provider = provider
	r.body = append([]byte(nil), body...)
	r.headers = headers
	return model.IngestResult{}, r.err
}
