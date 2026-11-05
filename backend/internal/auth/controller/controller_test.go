package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
)

const testCookieName = "__Host-kg_session"

func TestController_SessionDeviceLifecycleRequiresAuthenticationAndCSRF(t *testing.T) {
	t.Parallel()

	app, service, email := newControllerHarness(t)
	first := controllerSignIn(t, service, email, "parent@example.com", "first device")
	current := controllerSignIn(t, service, email, "parent@example.com", "current device")

	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/v1/auth/sessions", nil)
	response, err := app.Test(unauthenticated)
	if err != nil {
		t.Fatalf("app.Test(unauthenticated list) error = %v", err)
	}
	defer response.Body.Close()
	assertControllerProblem(t, response, fiber.StatusUnauthorized, "unauthenticated")

	listRequest := authenticatedRequest(http.MethodGet, "/api/v1/auth/sessions", current, false)
	response, err = app.Test(listRequest)
	if err != nil {
		t.Fatalf("app.Test(list) error = %v", err)
	}
	listBody := readControllerBody(t, response.Body)
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("list status = %d, want %d: %s", response.StatusCode, fiber.StatusOK, listBody)
	}
	for _, secret := range []string{current.Token, current.CSRFToken, "tokenHash", "csrfHash", "ipAddressHash"} {
		if strings.Contains(listBody, secret) {
			t.Fatalf("session list exposed secret %q: %s", secret, listBody)
		}
	}
	var listPayload struct {
		Items []model.SessionSummary `json:"items"`
	}
	if err := json.Unmarshal([]byte(listBody), &listPayload); err != nil {
		t.Fatalf("decoding list response: %v", err)
	}
	if len(listPayload.Items) != 2 {
		t.Fatalf("session list count = %d, want 2", len(listPayload.Items))
	}

	withoutCSRF := authenticatedRequest(
		http.MethodDelete,
		"/api/v1/auth/sessions/"+first.Session.ID,
		current,
		false,
	)
	response, err = app.Test(withoutCSRF)
	if err != nil {
		t.Fatalf("app.Test(revoke without csrf) error = %v", err)
	}
	defer response.Body.Close()
	assertControllerProblem(t, response, fiber.StatusForbidden, "invalid_csrf")
	if _, err := service.Authenticate(context.Background(), first.Token); err != nil {
		t.Fatalf("missing-CSRF request revoked the target: %v", err)
	}

	withCSRF := authenticatedRequest(
		http.MethodDelete,
		"/api/v1/auth/sessions/"+first.Session.ID,
		current,
		true,
	)
	response, err = app.Test(withCSRF)
	if err != nil {
		t.Fatalf("app.Test(revoke selected) error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("revoke selected status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
	}
	if _, err := service.Authenticate(context.Background(), first.Token); err == nil {
		t.Fatal("selected session remains authenticated")
	}

	other := controllerSignIn(t, service, email, "parent@example.com", "other device")
	revokeOthers := authenticatedRequest(
		http.MethodPost,
		"/api/v1/auth/sessions/revoke-others",
		current,
		true,
	)
	response, err = app.Test(revokeOthers)
	if err != nil {
		t.Fatalf("app.Test(revoke others) error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("revoke others status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
	}
	if _, err := service.Authenticate(context.Background(), other.Token); err == nil {
		t.Fatal("other session remains authenticated")
	}
	if _, err := service.Authenticate(context.Background(), current.Token); err != nil {
		t.Fatalf("current session was revoked with other sessions: %v", err)
	}

	revokeAll := authenticatedRequest(http.MethodDelete, "/api/v1/auth/sessions", current, true)
	response, err = app.Test(revokeAll)
	if err != nil {
		t.Fatalf("app.Test(revoke all) error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusNoContent {
		t.Fatalf("revoke all status = %d, want %d", response.StatusCode, fiber.StatusNoContent)
	}
	if _, err := service.Authenticate(context.Background(), current.Token); err == nil {
		t.Fatal("current session remains authenticated after revoke all")
	}
	cookies := response.Cookies()
	if len(cookies) == 0 || cookies[0].Name != testCookieName || cookies[0].Value != "" {
		t.Fatalf("revoke all did not clear the session cookie: %+v", cookies)
	}
}

func TestController_StudentCodeExchangeRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	app, _, _ := newControllerHarness(t)
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/student-codes/exchange",
		strings.NewReader(`{"code":"high-entropy-code","unexpected":true}`),
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	assertControllerProblem(t, response, fiber.StatusBadRequest, "invalid_json")
}

type controllerEmail struct {
	link string
}

func (e *controllerEmail) SendPasswordVerification(ctx context.Context, to, link string, expires time.Time) error {
	return e.SendMagicLink(ctx, to, link, expires)
}

func (e *controllerEmail) SendMagicLink(_ context.Context, _, link string, _ time.Time) error {
	e.link = link
	return nil
}

func newControllerHarness(t *testing.T) (*fiber.App, *authservice.Service, *controllerEmail) {
	t.Helper()
	repo := repository.NewMemory()
	email := &controllerEmail{}
	service, err := authservice.New(repo, email, authservice.Config{
		PublicAPIURL:       "https://api.example.com",
		PublicWebURL:       "https://www.example.com",
		MagicLinkTTL:       15 * time.Minute,
		SessionIdleTTL:     6 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
	})
	if err != nil {
		t.Fatalf("authservice.New() error = %v", err)
	}
	controller := New(service, Config{
		CookieName:   testCookieName,
		CookieSecure: true,
		CookieTTL:    24 * time.Hour,
		PublicWebURL: "https://www.example.com",
	})
	app := fiber.New()
	controller.Register(app.Group("/api/v1"))
	return app, service, email
}

func controllerSignIn(
	t *testing.T,
	service *authservice.Service,
	email *controllerEmail,
	address string,
	userAgent string,
) authservice.SessionResult {
	t.Helper()
	if err := service.RequestMagicLink(context.Background(), address, "/portal"); err != nil {
		t.Fatalf("RequestMagicLink() error = %v", err)
	}
	parsed, err := url.Parse(email.link)
	if err != nil {
		t.Fatalf("parsing magic link: %v", err)
	}
	raw := parsed.Query().Get("token")
	result, _, err := service.ConsumeMagicLink(context.Background(), raw, userAgent, "192.0.2.1")
	if err != nil {
		t.Fatalf("ConsumeMagicLink() error = %v", err)
	}
	return result
}

func authenticatedRequest(method, target string, session authservice.SessionResult, csrf bool) *http.Request {
	request := httptest.NewRequest(method, target, nil)
	request.AddCookie(&http.Cookie{Name: testCookieName, Value: session.Token})
	if csrf {
		request.Header.Set("X-CSRF-Token", session.CSRFToken)
	}
	return request
}

func assertControllerProblem(t *testing.T, response *http.Response, status int, code string) {
	t.Helper()
	body := readControllerBody(t, response.Body)
	if response.StatusCode != status {
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, status, body)
	}
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(body), &problem); err != nil {
		t.Fatalf("decoding problem response: %v", err)
	}
	if problem.Code != code {
		t.Fatalf("problem code = %q, want %q", problem.Code, code)
	}
}

func readControllerBody(t *testing.T, body io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return string(data)
}
