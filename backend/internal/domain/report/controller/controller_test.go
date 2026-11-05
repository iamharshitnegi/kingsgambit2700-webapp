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
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

const (
	testReportID        = "018f1f00-0000-7000-8000-000000000001"
	testVersionID       = "018f1f00-0000-7000-8000-000000000002"
	testStudentID       = "018f1f00-0000-7000-8000-000000000003"
	testActorID         = "018f1f00-0000-7000-8000-000000000004"
	testRubricVersionID = "018f1f00-0000-7000-8000-000000000005"
)

func TestController_PublishUsesAuthenticatedActor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 27, 10, 0, 0, 0, time.UTC)
	period, err := report.NewMonthlyPeriod(2026, time.August)
	if err != nil {
		t.Fatalf("NewMonthlyPeriod() error = %v", err)
	}
	draft, err := report.New(report.NewParams{
		ID:              testReportID,
		StudentID:       testStudentID,
		Period:          period,
		VersionID:       testVersionID,
		RubricVersionID: testRubricVersionID,
		Content:         validContent(),
		ActorID:         testActorID,
		CreatedAt:       now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	service := &fakeService{}
	service.publish = func(
		_ context.Context,
		command report.PublishCommand,
	) (report.Report, error) {
		if command.ActorID != testActorID {
			t.Fatalf("Publish() actor = %q, want %q", command.ActorID, testActorID)
		}
		if command.ReportID != testReportID || command.VersionID != testVersionID {
			t.Fatalf("Publish() command = %+v", command)
		}
		return draft.Publish(command.VersionID, command.ActorID, command.PublishedAt)
	}
	controller, err := New(Dependencies{
		Service: service,
		Actors:  staticActor{actorID: testActorID, isAuthenticated: true},
		NewID:   func() string { return testVersionID },
		Now:     func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(
		http.MethodPost,
		"/reports/"+testReportID+"/versions/"+testVersionID+"/publish",
		strings.NewReader(`{"version":1}`),
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != fiber.StatusOK {
		data := readBody(t, response.Body)
		t.Fatalf("status = %d, want %d: %s", response.StatusCode, fiber.StatusOK, data)
	}
	var payload struct {
		Version  uint64 `json:"version"`
		Versions []struct {
			Status report.VersionStatus `json:"status"`
		} `json:"versions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if payload.Version != 2 || len(payload.Versions) != 1 ||
		payload.Versions[0].Status != report.VersionStatusPublished {
		t.Fatalf("response = %+v", payload)
	}
}

func TestController_RecordViewRejectsViewerSpoofing(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	controller, err := New(Dependencies{
		Service: service,
		Actors:  staticActor{actorID: testActorID, isAuthenticated: true},
		NewID:   func() string { return testVersionID },
		Now:     time.Now,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(
		http.MethodPost,
		"/reports/"+testReportID+"/versions/"+testVersionID+"/view",
		strings.NewReader(`{"version":2,"viewerId":"018f1f00-0000-7000-8000-000000000099"}`),
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	assertProblem(t, response, fiber.StatusBadRequest, "invalid_json")
}

func TestController_ReceiptResponsesExcludeUnpublishedCorrections(t *testing.T) {
	now := time.Now().UTC()
	period, err := report.NewMonthlyPeriod(2026, time.August)
	if err != nil {
		t.Fatal(err)
	}
	value, err := report.New(report.NewParams{
		ID: testReportID, StudentID: testStudentID, Period: period, VersionID: testVersionID,
		RubricVersionID: testRubricVersionID, Content: validContent(), ActorID: testActorID, CreatedAt: now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	value, err = value.Publish(testVersionID, testActorID, now.Add(-30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	correctionID := "018f1f00-0000-7000-8000-000000000099"
	value, err = value.StartCorrection(correctionID, testVersionID, testActorID, now.Add(-20*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	content := validContent()
	content.CoachComment.Text = "UNPUBLISHED_PRIVATE_CORRECTION"
	value, err = value.EditDraft(correctionID, content, testActorID, now.Add(-10*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"view", "acknowledge"} {
		t.Run(operation, func(t *testing.T) {
			receipt := func(context.Context, report.ReceiptCommand) (report.Report, error) { return value, nil }
			controller, err := New(Dependencies{
				Service: &fakeService{view: receipt, acknowledge: receipt},
				Actors:  staticActor{actorID: testActorID, isAuthenticated: true},
				NewID:   func() string { return testVersionID }, Now: func() time.Time { return now },
			})
			if err != nil {
				t.Fatal(err)
			}
			app := fiber.New()
			controller.Register(app)
			response, err := app.Test(httptest.NewRequest(http.MethodPost,
				"/reports/"+testReportID+"/versions/"+testVersionID+"/"+operation,
				strings.NewReader(`{"version":4}`)))
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body := string(readBody(t, response.Body))
			if response.StatusCode != fiber.StatusOK {
				t.Fatalf("status = %d: %s", response.StatusCode, body)
			}
			if strings.Contains(body, correctionID) || strings.Contains(body, "UNPUBLISHED_PRIVATE_CORRECTION") {
				t.Fatalf("receipt disclosed unpublished correction: %s", body)
			}
			if !strings.Contains(body, testVersionID) {
				t.Fatal("receipt omitted the published report")
			}
		})
	}
}

func TestController_PublishMapsPermissionDenied(t *testing.T) {
	t.Parallel()

	service := &fakeService{}
	service.publish = func(
		context.Context,
		report.PublishCommand,
	) (report.Report, error) {
		return report.Report{}, report.ErrPermissionDenied
	}
	controller, err := New(Dependencies{
		Service: service,
		Actors:  staticActor{actorID: testActorID, isAuthenticated: true},
		NewID:   func() string { return testVersionID },
		Now:     time.Now,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	app := fiber.New()
	controller.Register(app)
	request := httptest.NewRequest(
		http.MethodPost,
		"/reports/"+testReportID+"/versions/"+testVersionID+"/publish",
		strings.NewReader(`{"version":1}`),
	)
	response, err := app.Test(request)
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	defer response.Body.Close()
	assertProblem(t, response, fiber.StatusForbidden, "report_forbidden")
}

type staticActor struct {
	actorID         string
	isAuthenticated bool
}

func (a staticActor) ActorID(fiber.Ctx) (string, bool) {
	return a.actorID, a.isAuthenticated
}

type fakeService struct {
	create      func(context.Context, report.CreateCommand) (report.Report, error)
	edit        func(context.Context, report.EditCommand) (report.Report, error)
	publish     func(context.Context, report.PublishCommand) (report.Report, error)
	correction  func(context.Context, report.CorrectCommand) (report.Report, error)
	view        func(context.Context, report.ReceiptCommand) (report.Report, error)
	acknowledge func(context.Context, report.ReceiptCommand) (report.Report, error)
}

func (s *fakeService) CreateMonthly(
	ctx context.Context,
	command report.CreateCommand,
) (report.Report, error) {
	if s.create == nil {
		return report.Report{}, errors.New("unexpected create monthly call")
	}
	return s.create(ctx, command)
}

func (s *fakeService) EditDraft(
	ctx context.Context,
	command report.EditCommand,
) (report.Report, error) {
	if s.edit == nil {
		return report.Report{}, errors.New("unexpected edit draft call")
	}
	return s.edit(ctx, command)
}

func (s *fakeService) Publish(
	ctx context.Context,
	command report.PublishCommand,
) (report.Report, error) {
	if s.publish == nil {
		return report.Report{}, errors.New("unexpected publish call")
	}
	return s.publish(ctx, command)
}

func (s *fakeService) StartCorrection(
	ctx context.Context,
	command report.CorrectCommand,
) (report.Report, error) {
	if s.correction == nil {
		return report.Report{}, errors.New("unexpected start correction call")
	}
	return s.correction(ctx, command)
}

func (s *fakeService) RecordView(
	ctx context.Context,
	command report.ReceiptCommand,
) (report.Report, error) {
	if s.view == nil {
		return report.Report{}, errors.New("unexpected record view call")
	}
	return s.view(ctx, command)
}

func (s *fakeService) Acknowledge(
	ctx context.Context,
	command report.ReceiptCommand,
) (report.Report, error) {
	if s.acknowledge == nil {
		return report.Report{}, errors.New("unexpected acknowledge call")
	}
	return s.acknowledge(ctx, command)
}

func validContent() report.Content {
	return report.Content{
		Learned:      report.Written("learned"),
		Strengths:    report.Written("strengths"),
		FocusNext:    report.Written("focus next"),
		PracticePlan: report.Written("practice plan"),
		CoachComment: report.Written("coach comment"),
	}
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
