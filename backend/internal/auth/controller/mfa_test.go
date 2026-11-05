package controller

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/auth/totp"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestController_MFAEnrollmentUpgradesRestrictedStaffSession(t *testing.T) {
	t.Parallel()
	app, service, email, staff := newMFAControllerHarness(t)
	primary := controllerSignIn(t, service, email, staff.Email, "coach device")

	sessionRequest := authenticatedRequest(http.MethodGet, "/api/v1/auth/session", primary, false)
	response, err := app.Test(sessionRequest)
	if err != nil {
		t.Fatalf("app.Test(session) error = %v", err)
	}
	var sessionBody struct {
		Authenticated bool `json:"authenticated"`
		MFARequired   bool `json:"mfaRequired"`
	}
	decodeControllerJSON(t, response, &sessionBody)
	if sessionBody.Authenticated || !sessionBody.MFARequired {
		t.Fatalf("restricted session response = %+v", sessionBody)
	}

	statusRequest := authenticatedRequest(http.MethodGet, "/api/v1/auth/mfa/status", primary, false)
	response, err = app.Test(statusRequest)
	if err != nil {
		t.Fatalf("app.Test(mfa status) error = %v", err)
	}
	var status authservice.MFAStatus
	decodeControllerJSON(t, response, &status)
	if !status.Required || status.Enrolled || status.CSRFToken == "" {
		t.Fatalf("mfa status = %+v", status)
	}

	withoutCSRF := authenticatedJSONRequest(
		http.MethodPost,
		"/api/v1/auth/mfa/totp/enrollment",
		primary.Token,
		"",
		"",
	)
	response, err = app.Test(withoutCSRF)
	if err != nil {
		t.Fatalf("app.Test(enrollment without csrf) error = %v", err)
	}
	assertControllerProblem(t, response, fiber.StatusForbidden, "invalid_csrf")

	startRequest := authenticatedJSONRequest(
		http.MethodPost,
		"/api/v1/auth/mfa/totp/enrollment",
		primary.Token,
		status.CSRFToken,
		"",
	)
	response, err = app.Test(startRequest)
	if err != nil {
		t.Fatalf("app.Test(start enrollment) error = %v", err)
	}
	var enrollment authservice.TOTPEnrollment
	decodeControllerJSON(t, response, &enrollment)
	if enrollment.Secret == "" || !strings.HasPrefix(enrollment.OTPAuthURI, "otpauth://totp/") {
		t.Fatalf("enrollment response = %+v", enrollment)
	}
	code, err := totp.Generate(enrollment.Secret, time.Now())
	if err != nil {
		t.Fatalf("totp.Generate() error = %v", err)
	}
	confirmRequest := authenticatedJSONRequest(
		http.MethodPost,
		"/api/v1/auth/mfa/totp/enrollment/confirm",
		primary.Token,
		status.CSRFToken,
		`{"code":"`+code+`"}`,
	)
	response, err = app.Test(confirmRequest)
	if err != nil {
		t.Fatalf("app.Test(confirm enrollment) error = %v", err)
	}
	var completed struct {
		Authenticated bool     `json:"authenticated"`
		RecoveryCodes []string `json:"recoveryCodes"`
	}
	decodeControllerJSON(t, response, &completed)
	if !completed.Authenticated || len(completed.RecoveryCodes) != 10 {
		t.Fatalf("completion response = %+v", completed)
	}
	cookies := response.Cookies()
	if len(cookies) == 0 || cookies[0].Value == "" || cookies[0].Value == primary.Token {
		t.Fatalf("completion did not rotate the session cookie: %+v", cookies)
	}
	if _, err := service.Authenticate(context.Background(), cookies[0].Value); err != nil {
		t.Fatalf("Authenticate(rotated cookie) error = %v", err)
	}
}

func newMFAControllerHarness(
	t *testing.T,
) (*fiber.App, *authservice.Service, *controllerEmail, model.User) {
	t.Helper()
	repo := repository.NewMemory()
	staff := model.User{
		ID:          platformid.New(),
		Email:       "coach-controller@example.com",
		DisplayName: "Controller Coach",
		Roles:       []model.Role{model.RoleCoach},
		Status:      "active",
		CreatedAt:   time.Now().UTC(),
	}
	repo.PutUser(staff)
	email := &controllerEmail{}
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("c", 32)))
	service, err := authservice.New(repo, email, authservice.Config{
		PublicAPIURL:       "https://api.example.com",
		PublicWebURL:       "https://www.example.com",
		MagicLinkTTL:       15 * time.Minute,
		SessionIdleTTL:     6 * time.Hour,
		SessionAbsoluteTTL: 24 * time.Hour,
		CSRFTTL:            time.Hour,
		TOTPEncryptionKey:  key,
		MFAChallengeTTL:    5 * time.Minute,
		MFARecentTTL:       5 * time.Minute,
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
	return app, service, email, staff
}

func authenticatedJSONRequest(method, target, session, csrf, body string) *http.Request {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.AddCookie(&http.Cookie{Name: testCookieName, Value: session})
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}

func decodeControllerJSON(t *testing.T, response *http.Response, target any) {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("response status = %d, want success: %s", response.StatusCode, readControllerBody(t, response.Body))
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
}
