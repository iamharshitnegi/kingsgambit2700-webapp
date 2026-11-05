package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
)

const (
	actorID   = "01990000-0000-7000-8000-000000000001"
	accountID = "01990000-0000-7000-8000-000000000002"
	studentID = "01990000-0000-7000-8000-000000000003"
	entryID   = "01990000-0000-7000-8000-000000000004"
)

func TestControllerBalanceAndHistory(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 31, 12, 30, 0, 0, time.UTC)
	next := entitlement.EntryCursor{OccurredAt: now, ID: entryID}
	service := &stubService{
		balance: entitlement.AccountBalance{
			Account: entitlement.Account{
				ID:          accountID,
				EnrolmentID: "01990000-0000-7000-8000-000000000005",
				StudentID:   studentID,
				Unit:        "session_credit",
				CreatedAt:   now,
			},
			Balance: entitlement.Balance{Available: 4, Reserved: 1, Consumed: 2},
		},
		history: entitlement.EntryPage{
			Items:      []entitlement.Entry{testEntry(now)},
			NextCursor: &next,
		},
	}
	application := testApplication(t, service, actorResolver{id: actorID}, now)

	response, err := application.Test(httptest.NewRequest(
		http.MethodGet,
		"/entitlements/accounts/"+accountID,
		nil,
	))
	if err != nil {
		t.Fatalf("balance Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("balance status = %d", response.StatusCode)
	}
	var balance accountBalanceResponse
	if err := json.NewDecoder(response.Body).Decode(&balance); err != nil {
		t.Fatalf("decoding balance: %v", err)
	}
	if balance.Account.ID != accountID || balance.Balance.Available != 4 {
		t.Fatalf("balance response = %+v", balance)
	}

	historyResponse, err := application.Test(httptest.NewRequest(
		http.MethodGet,
		"/entitlements/accounts/"+accountID+"/entries?limit=1",
		nil,
	))
	if err != nil {
		t.Fatalf("history Test() error = %v", err)
	}
	defer historyResponse.Body.Close()
	if historyResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("history status = %d", historyResponse.StatusCode)
	}
	var page entryPageResponse
	if err := json.NewDecoder(historyResponse.Body).Decode(&page); err != nil {
		t.Fatalf("decoding history: %v", err)
	}
	if len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("history response = %+v", page)
	}
	decoded, problem := decodeCursor(page.NextCursor)
	if problem != nil || decoded == nil || *decoded != next {
		t.Fatalf("decoded cursor = %+v, problem = %+v", decoded, problem)
	}
	if service.historyCommand.Limit != 1 || service.historyCommand.ActorID != actorID {
		t.Fatalf("history command = %+v", service.historyCommand)
	}
}

func TestControllerGrantUsesActorIdempotencyAndReplayStatus(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 31, 12, 30, 0, 0, time.UTC)
	service := &stubService{
		grant: entitlement.ApplyResult{
			Entry:   testEntry(now),
			Balance: entitlement.Balance{Available: 3},
			Created: false,
		},
	}
	application := testApplication(t, service, actorResolver{id: actorID}, now)
	request := httptest.NewRequest(
		http.MethodPost,
		"/entitlements/accounts/"+accountID+"/grants",
		strings.NewReader(`{"units":3,"referenceId":"approved-adjustment-42","reason":"Approved opening balance"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "manual-grant-2026-0001")
	response, err := application.Test(request)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusOK)
	}
	command := service.grantCommand
	if command.ActorID != actorID || command.AccountID != accountID ||
		command.EntryID != entryID || command.IdempotencyKey != "manual-grant-2026-0001" ||
		command.Units != 3 || !command.OccurredAt.Equal(now) {
		t.Fatalf("grant command = %+v", command)
	}
	var result applyResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decoding grant: %v", err)
	}
	if result.Created || result.Balance.Available != 3 {
		t.Fatalf("grant response = %+v", result)
	}
}

func TestControllerRejectsInvalidBoundaryInput(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 31, 12, 30, 0, 0, time.UTC)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		header string
	}{
		{name: "account", method: http.MethodGet, path: "/entitlements/accounts/not-a-uuid"},
		{name: "cursor", method: http.MethodGet, path: "/entitlements/accounts/" + accountID + "/entries?cursor=bad"},
		{name: "idempotency", method: http.MethodPost, path: "/entitlements/accounts/" + accountID + "/grants", body: `{"units":1,"referenceId":"x","reason":"why"}`, header: "short"},
		{name: "unknown field", method: http.MethodPost, path: "/entitlements/accounts/" + accountID + "/grants", body: `{"units":1,"referenceId":"x","reason":"why","extra":true}`, header: "manual-grant-2026-0002"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			service := &stubService{}
			application := testApplication(t, service, actorResolver{id: actorID}, now)
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			if test.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			if test.header != "" {
				request.Header.Set("Idempotency-Key", test.header)
			}
			response, err := application.Test(request)
			if err != nil {
				t.Fatalf("Test() error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != fiber.StatusUnprocessableEntity &&
				response.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("status = %d", response.StatusCode)
			}
			if service.calls != 0 {
				t.Fatalf("service calls = %d, want 0", service.calls)
			}
		})
	}
}

func TestControllerMapsAuthorizationFailure(t *testing.T) {
	t.Parallel()
	service := &stubService{balanceError: entitlement.ErrPermissionDenied}
	application := testApplication(t, service, actorResolver{id: actorID}, time.Now())
	response, err := application.Test(httptest.NewRequest(
		http.MethodGet,
		"/entitlements/accounts/"+accountID,
		nil,
	))
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusForbidden {
		t.Fatalf("status = %d", response.StatusCode)
	}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/problem+json") {
		t.Fatalf("content type = %q", contentType)
	}
}

type stubService struct {
	balance        entitlement.AccountBalance
	balanceError   error
	history        entitlement.EntryPage
	historyError   error
	grant          entitlement.ApplyResult
	grantError     error
	historyCommand entitlement.HistoryCommand
	grantCommand   entitlement.GrantCommand
	calls          int
}

func (s *stubService) ViewBalance(
	_ context.Context,
	_, _ string,
) (entitlement.AccountBalance, error) {
	s.calls++
	return s.balance, s.balanceError
}

func (s *stubService) History(
	_ context.Context,
	command entitlement.HistoryCommand,
) (entitlement.EntryPage, error) {
	s.calls++
	s.historyCommand = command
	return s.history, s.historyError
}

func (s *stubService) GrantForActor(
	_ context.Context,
	command entitlement.GrantCommand,
) (entitlement.ApplyResult, error) {
	s.calls++
	s.grantCommand = command
	return s.grant, s.grantError
}

type actorResolver struct {
	id string
}

func (r actorResolver) ActorID(fiber.Ctx) (string, bool) {
	return r.id, r.id != ""
}

func testApplication(
	t *testing.T,
	service Service,
	actors ActorResolver,
	now time.Time,
) *fiber.App {
	t.Helper()
	controller, err := New(Dependencies{
		Service: service,
		Actors:  actors,
		NewID:   func() string { return entryID },
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	application := fiber.New()
	controller.Register(application)
	return application
}

func testEntry(now time.Time) entitlement.Entry {
	return entitlement.Entry{
		ID:            entryID,
		AccountID:     accountID,
		StudentID:     studentID,
		Kind:          entitlement.EntryKindGranted,
		Delta:         entitlement.Delta{Available: 3},
		ReferenceType: "manual_grant",
		ReferenceID:   "approved-adjustment-42",
		Reason:        "Approved opening balance",
		ActorID:       actorID,
		OccurredAt:    now,
	}
}
