package controller

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/support"
)

const (
	testActorID    = "018f1f00-0000-7000-8000-000000000001"
	testFamilyID   = "018f1f00-0000-7000-8000-000000000002"
	testCaseID     = "018f1f00-0000-7000-8000-000000000003"
	testEventID    = "018f1f00-0000-7000-8000-000000000004"
	testAssigneeID = "018f1f00-0000-7000-8000-000000000005"
)

func TestController_CreateValidatesBoundary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		actor           staticActor
		body            string
		expectedStatus  int
		expectedCode    string
		expectedService bool
	}{
		{
			name:           "authentication required",
			actor:          staticActor{},
			body:           validCreateBody(),
			expectedStatus: fiber.StatusUnauthorized,
			expectedCode:   "unauthenticated",
		},
		{
			name:           "unknown field rejected",
			actor:          staticActor{actorID: testActorID, isAuthenticated: true},
			body:           strings.TrimSuffix(validCreateBody(), "}") + `,"actorId":"spoofed"}`,
			expectedStatus: fiber.StatusBadRequest,
			expectedCode:   "invalid_json",
		},
		{
			name:           "invalid family rejected",
			actor:          staticActor{actorID: testActorID, isAuthenticated: true},
			body:           strings.Replace(validCreateBody(), testFamilyID, "not-a-family", 1),
			expectedStatus: fiber.StatusUnprocessableEntity,
			expectedCode:   "invalid_support_case",
		},
		{
			name:            "valid create uses authenticated actor",
			actor:           staticActor{actorID: testActorID, isAuthenticated: true},
			body:            validCreateBody(),
			expectedStatus:  fiber.StatusCreated,
			expectedService: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			service := &fakeService{}
			service.create = func(
				_ context.Context,
				command support.CreateCommand,
			) (support.Case, error) {
				called = true
				if command.ActorID != testActorID || command.FamilyID != testFamilyID {
					t.Fatalf("Create() command = %+v", command)
				}
				return responseCase(t), nil
			}
			controller := newTestController(t, service, tt.actor)
			app := fiber.New()
			controller.Register(app)
			request := httptest.NewRequest(
				http.MethodPost,
				"/support/cases",
				strings.NewReader(tt.body),
			)
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != tt.expectedStatus {
				t.Fatalf("status = %d, want %d: %s", response.StatusCode, tt.expectedStatus, readBody(t, response.Body))
			}
			if called != tt.expectedService {
				t.Fatalf("service called = %t, want %t", called, tt.expectedService)
			}
			if tt.expectedCode != "" {
				assertProblem(t, response, tt.expectedCode)
			}
		})
	}
}

func TestController_AppendEventMapsDomainErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		serviceError   error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "internal note denied",
			serviceError:   support.ErrPermissionDenied,
			expectedStatus: fiber.StatusForbidden,
			expectedCode:   "support_case_forbidden",
		},
		{
			name:           "stale version conflicts",
			serviceError:   support.ErrConflict,
			expectedStatus: fiber.StatusConflict,
			expectedCode:   "support_case_version_conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			service := &fakeService{}
			service.appendEvent = func(
				context.Context,
				support.AppendEventCommand,
			) (support.Case, error) {
				return support.Case{}, tt.serviceError
			}
			controller := newTestController(t, service, staticActor{
				actorID:         testActorID,
				isAuthenticated: true,
			})
			app := fiber.New()
			controller.Register(app)
			request := httptest.NewRequest(
				http.MethodPost,
				"/support/cases/"+testCaseID+"/events",
				strings.NewReader(`{"version":1,"visibility":"internal","body":"staff note"}`),
			)
			response, err := app.Test(request)
			if err != nil {
				t.Fatalf("app.Test() error = %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode != tt.expectedStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, tt.expectedStatus)
			}
			assertProblem(t, response, tt.expectedCode)
		})
	}
}

func TestController_ListRejectsMalformedCursor(t *testing.T) {
	t.Parallel()

	controller := newTestController(t, &fakeService{}, staticActor{
		actorID:         testActorID,
		isAuthenticated: true,
	})
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(
		http.MethodGet,
		"/support/cases?familyId="+testFamilyID+"&cursor=not-base64!",
		nil,
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", response.StatusCode, fiber.StatusUnprocessableEntity)
	}
	assertProblem(t, response, "invalid_support_case")
}

func validCreateBody() string {
	return `{"familyId":"` + testFamilyID +
		`","category":"learning","severity":"medium",` +
		`"subject":"Assignment question","message":"Please help."}`
}

func responseCase(t *testing.T) support.Case {
	t.Helper()
	now := time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
	event := support.Event{
		ID:         testEventID,
		CaseID:     testCaseID,
		Type:       support.EventTypeOpened,
		Visibility: support.VisibilityCustomerVisible,
		ActorID:    testActorID,
		Body:       "Please help.",
		Metadata:   map[string]string{},
		OccurredAt: now,
	}
	created, err := support.New(support.NewParams{
		ID:           testCaseID,
		FamilyID:     testFamilyID,
		OpenedBy:     testActorID,
		Category:     support.CategoryLearning,
		Severity:     support.SeverityMedium,
		Subject:      "Assignment question",
		OpenedAt:     now,
		SLADueAt:     now.Add(24 * time.Hour),
		InitialEvent: event,
	})
	if err != nil {
		t.Fatalf("support.New() error = %v", err)
	}
	return created
}

func newTestController(t *testing.T, service Service, actor staticActor) *Controller {
	t.Helper()
	ids := []string{testCaseID, testEventID, testAssigneeID}
	index := 0
	controller, err := New(Dependencies{
		Service: service,
		Actors:  actor,
		NewID: func() string {
			value := ids[index%len(ids)]
			index++
			return value
		},
		Now: func() time.Time {
			return time.Date(2026, time.August, 31, 10, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return controller
}

type staticActor struct {
	actorID         string
	isAuthenticated bool
}

func (a staticActor) ActorID(fiber.Ctx) (string, bool) {
	return a.actorID, a.isAuthenticated
}

type fakeService struct {
	create       func(context.Context, support.CreateCommand) (support.Case, error)
	list         func(context.Context, support.ListCommand) (support.Page, error)
	view         func(context.Context, string, string) (support.Case, error)
	assign       func(context.Context, support.AssignCommand) (support.Case, error)
	changeStatus func(context.Context, support.ChangeStatusCommand) (support.Case, error)
	appendEvent  func(context.Context, support.AppendEventCommand) (support.Case, error)
}

func (s *fakeService) Create(
	ctx context.Context,
	command support.CreateCommand,
) (support.Case, error) {
	if s.create == nil {
		return support.Case{}, errors.New("unexpected create call")
	}
	return s.create(ctx, command)
}

func (s *fakeService) List(
	ctx context.Context,
	command support.ListCommand,
) (support.Page, error) {
	if s.list == nil {
		return support.Page{}, errors.New("unexpected list call")
	}
	return s.list(ctx, command)
}

func (s *fakeService) View(
	ctx context.Context,
	actorID string,
	caseID string,
) (support.Case, error) {
	if s.view == nil {
		return support.Case{}, errors.New("unexpected view call")
	}
	return s.view(ctx, actorID, caseID)
}

func (s *fakeService) Assign(
	ctx context.Context,
	command support.AssignCommand,
) (support.Case, error) {
	if s.assign == nil {
		return support.Case{}, errors.New("unexpected assign call")
	}
	return s.assign(ctx, command)
}

func (s *fakeService) ChangeStatus(
	ctx context.Context,
	command support.ChangeStatusCommand,
) (support.Case, error) {
	if s.changeStatus == nil {
		return support.Case{}, errors.New("unexpected change status call")
	}
	return s.changeStatus(ctx, command)
}

func (s *fakeService) AppendEvent(
	ctx context.Context,
	command support.AppendEventCommand,
) (support.Case, error) {
	if s.appendEvent == nil {
		return support.Case{}, errors.New("unexpected append event call")
	}
	return s.appendEvent(ctx, command)
}

func assertProblem(t *testing.T, response *http.Response, expectedCode string) {
	t.Helper()
	var problem struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatalf("decoding problem: %v", err)
	}
	if problem.Code != expectedCode {
		t.Fatalf("problem code = %q, want %q", problem.Code, expectedCode)
	}
}

func readBody(t *testing.T, body io.Reader) string {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	return string(data)
}
