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
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

const (
	testFamilyID       = "018f0f00-0000-7000-8000-000000000001"
	testGuardianID     = "018f0f00-0000-7000-8000-000000000002"
	testStudentID      = "018f0f00-0000-7000-8000-000000000003"
	testActorID        = "018f0f00-0000-7000-8000-000000000004"
	testRelationshipID = "018f0f00-0000-7000-8000-000000000005"
	testEventID        = "018f0f00-0000-7000-8000-000000000006"
)

func TestController_GrantUsesAuthenticatedActor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	service := &fakeService{}
	service.grant = func(
		_ context.Context,
		command family.GrantCommand,
	) (family.Relationship, error) {
		if command.ActorID != testActorID {
			t.Fatalf("Grant() actor = %q, want %q", command.ActorID, testActorID)
		}
		if command.FamilyID != testFamilyID || command.EventID != testEventID {
			t.Fatalf("Grant() command = %+v", command)
		}
		return family.NewRelationship(family.NewRelationshipParams{
			ID:          command.RelationshipID,
			FamilyID:    command.FamilyID,
			GuardianID:  command.GuardianID,
			StudentID:   command.StudentID,
			Permissions: command.Permissions,
			GrantedBy:   command.ActorID,
			GrantedAt:   command.OccurredAt,
		})
	}
	ids := []string{testRelationshipID, testEventID}
	controller, err := New(Dependencies{
		Service: service,
		Actors:  staticActor{actorID: testActorID, isAuthenticated: true},
		NewID: func() string {
			id := ids[0]
			ids = ids[1:]
			return id
		},
		Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app)

	body := `{"guardianId":"` + testGuardianID + `","studentId":"` +
		testStudentID + `","permissions":["view_schedule","view_progress"]}`
	request := httptest.NewRequest(
		http.MethodPost,
		"/families/"+testFamilyID+"/guardian-relationships",
		strings.NewReader(body),
	)
	request.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusCreated {
		data := readBody(t, response.Body)
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, fiber.StatusCreated, data)
	}
	var payload struct {
		ID      string `json:"id"`
		Version uint64 `json:"version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if payload.ID != testRelationshipID || payload.Version != 1 {
		t.Fatalf("response = %+v", payload)
	}
}

func TestController_GrantRejectsUnauthenticatedRequest(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	controller, err := New(Dependencies{
		Service: service,
		Actors:  staticActor{},
		NewID:   func() string { return testRelationshipID },
		Now:     time.Now,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(
		http.MethodPost,
		"/families/"+testFamilyID+"/guardian-relationships",
		strings.NewReader(`{}`),
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	assertProblem(t, response, fiber.StatusUnauthorized, "unauthenticated")
}

func TestController_ReplacePermissionsMapsConflict(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	service.replace = func(
		context.Context,
		family.ReplacePermissionsCommand,
	) (family.Relationship, error) {
		return family.Relationship{}, family.ErrConflict
	}
	controller, err := New(Dependencies{
		Service: service,
		Actors:  staticActor{actorID: testActorID, isAuthenticated: true},
		NewID:   func() string { return testEventID },
		Now:     time.Now,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app)
	body := `{"version":2,"permissions":["view_progress"],"reason":"scope updated"}`
	request := httptest.NewRequest(
		http.MethodPatch,
		"/families/"+testFamilyID+"/guardian-relationships/"+
			testRelationshipID+"/permissions",
		strings.NewReader(body),
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	assertProblem(t, response, fiber.StatusConflict, "relationship_version_conflict")
}

type staticActor struct {
	actorID         string
	isAuthenticated bool
}

func (a staticActor) ActorID(fiber.Ctx) (string, bool) {
	return a.actorID, a.isAuthenticated
}

type fakeService struct {
	grant   func(context.Context, family.GrantCommand) (family.Relationship, error)
	replace func(context.Context, family.ReplacePermissionsCommand) (family.Relationship, error)
	revoke  func(context.Context, family.RevokeCommand) (family.Relationship, error)
}

func (s *fakeService) Grant(
	ctx context.Context,
	command family.GrantCommand,
) (family.Relationship, error) {
	if s.grant == nil {
		return family.Relationship{}, errors.New("unexpected grant call")
	}
	return s.grant(ctx, command)
}

func (s *fakeService) ReplacePermissions(
	ctx context.Context,
	command family.ReplacePermissionsCommand,
) (family.Relationship, error) {
	if s.replace == nil {
		return family.Relationship{}, errors.New("unexpected replace permissions call")
	}
	return s.replace(ctx, command)
}

func (s *fakeService) Revoke(
	ctx context.Context,
	command family.RevokeCommand,
) (family.Relationship, error) {
	if s.revoke == nil {
		return family.Relationship{}, errors.New("unexpected revoke call")
	}
	return s.revoke(ctx, command)
}

func assertProblem(t *testing.T, response *http.Response, expectedStatus int, expectedCode string) {
	t.Helper()
	if response.StatusCode != expectedStatus {
		data := readBody(t, response.Body)
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, expectedStatus, data)
	}
	if contentType := response.Header.Get(fiber.HeaderContentType); contentType != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", contentType)
	}
	var problem httpx.Problem
	if err := json.NewDecoder(response.Body).Decode(&problem); err != nil {
		t.Fatalf("decoding problem: %v", err)
	}
	if problem.Code != expectedCode {
		t.Fatalf("problem code = %q, want %q", problem.Code, expectedCode)
	}
}

func readBody(t *testing.T, reader io.Reader) []byte {
	t.Helper()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	return data
}
