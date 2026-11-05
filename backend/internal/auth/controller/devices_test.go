package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
)

func TestRememberedBootstrapRestoreAndExplicitLogout(t *testing.T) {
	app, service, email := newControllerHarness(t)
	original := controllerSignIn(t, service, email, "remembered@example.com", "browser")
	response, err := app.Test(authenticatedRequest(http.MethodGet, "/api/v1/auth/session", original, false))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("bootstrap status %d", response.StatusCode)
	}
	var payload struct {
		Authenticated    bool   `json:"authenticated"`
		RestoreAvailable bool   `json:"restoreAvailable"`
		CSRFToken        string `json:"csrfToken"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	var device *http.Cookie
	for _, cookie := range response.Cookies() {
		if cookie.Name == testCookieName+"_device" {
			device = cookie
		}
	}
	if device == nil || !device.HttpOnly || !device.Secure || device.Path != "/" || device.MaxAge != 400*24*60*60 {
		t.Fatalf("persistent cookie attributes: %+v", device)
	}
	originalCSRF := payload.CSRFToken
	// A background read may finish after another tab signs into another account.
	// Its JSON can be ignored by the client; browser Set-Cookie cannot.
	request := authenticatedRequest(http.MethodGet, "/api/v1/auth/session", original, false)
	request.AddCookie(device)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || len(response.Header.Values("Set-Cookie")) != 0 {
		t.Fatalf("ordinary bootstrap changed browser credentials: status=%d cookies=%v", response.StatusCode, response.Header.Values("Set-Cookie"))
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if !payload.Authenticated || payload.CSRFToken != originalCSRF {
		t.Fatalf("ordinary bootstrap changed identity or CSRF: %+v", payload)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.AddCookie(device)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	payload.Authenticated = false
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if payload.Authenticated || !payload.RestoreAvailable || payload.CSRFToken != originalCSRF {
		t.Fatalf("restore bootstrap: %+v", payload)
	}
	for _, origin := range []string{"", "https://evil.example"} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/session/restore", nil)
		request.AddCookie(device)
		request.Header.Set("Origin", origin)
		request.Header.Set("X-CSRF-Token", originalCSRF)
		response, err := app.Test(request)
		if err != nil {
			t.Fatal(err)
		}
		assertControllerProblem(t, response, 403, "origin_forbidden")
		response.Body.Close()
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/session/restore", nil)
	request.AddCookie(device)
	request.Header.Set("Origin", "https://www.example.com")
	request.Header.Set("X-CSRF-Token", originalCSRF)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("restore status %d: %s", response.StatusCode, readControllerBody(t, response.Body))
	}
	var renewed bool
	for _, cookie := range response.Cookies() {
		if cookie.Name == device.Name && cookie.Value == device.Value && cookie.MaxAge == 400*24*60*60 {
			renewed = true
		}
	}
	if !renewed {
		t.Fatal("access restoration did not renew the remembered browser cookie")
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if !payload.Authenticated || payload.CSRFToken != originalCSRF {
		t.Fatalf("restored identity: %+v", payload)
	}
	// Logout is still possible when the browser has no usable access cookie.
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	request.AddCookie(device)
	request.Header.Set("Origin", "https://www.example.com")
	request.Header.Set("X-CSRF-Token", originalCSRF)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 204 {
		t.Fatalf("logout status %d", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Value != "" {
			t.Fatal("logout retained credential")
		}
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/session/restore", nil)
	request.AddCookie(device)
	request.Header.Set("Origin", "https://www.example.com")
	request.Header.Set("X-CSRF-Token", originalCSRF)
	response, err = app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	assertControllerProblem(t, response, 401, "unauthenticated")
	response.Body.Close()
}

type failingSessionRepository struct{ repository.Repository }

func (failingSessionRepository) SessionByTokenHash(context.Context, []byte) (model.Session, model.User, error) {
	return model.Session{}, model.User{}, errors.New("database unavailable")
}
func TestSessionBootstrapPreservesOutageAsRetryableFailure(t *testing.T) {
	service, err := authservice.New(failingSessionRepository{repository.NewMemory()}, &controllerEmail{}, authservice.Config{MagicLinkTTL: time.Minute, SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour, CSRFTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	New(service, Config{CookieName: testCookieName, PublicWebURL: "https://www.example.com"}).Register(app.Group("/api/v1"))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	request.AddCookie(&http.Cookie{Name: testCookieName, Value: "existing-browser-session"})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	assertControllerProblem(t, response, 503, "session_unavailable")
	if len(response.Cookies()) != 0 {
		t.Fatal("service outage changed browser credentials")
	}
}
