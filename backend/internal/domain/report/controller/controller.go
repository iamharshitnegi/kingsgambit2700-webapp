package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Service interface {
	CreateMonthly(context.Context, report.CreateCommand) (report.Report, error)
	EditDraft(context.Context, report.EditCommand) (report.Report, error)
	Publish(context.Context, report.PublishCommand) (report.Report, error)
	StartCorrection(context.Context, report.CorrectCommand) (report.Report, error)
	RecordView(context.Context, report.ReceiptCommand) (report.Report, error)
	Acknowledge(context.Context, report.ReceiptCommand) (report.Report, error)
}

// ActorResolver reads the authenticated identity established by auth
// middleware. The report service remains the authorization boundary.
type ActorResolver interface {
	ActorID(c fiber.Ctx) (string, bool)
}

type Dependencies struct {
	Service Service
	Actors  ActorResolver
	NewID   func() string
	Now     func() time.Time
}

type Controller struct {
	service Service
	actors  ActorResolver
	newID   func() string
	now     func() time.Time
}

func New(dependencies Dependencies) (*Controller, error) {
	if dependencies.Service == nil || dependencies.Actors == nil ||
		dependencies.NewID == nil || dependencies.Now == nil {
		return nil, fmt.Errorf("report controller: all dependencies are required")
	}
	return &Controller{
		service: dependencies.Service,
		actors:  dependencies.Actors,
		newID:   dependencies.NewID,
		now:     dependencies.Now,
	}, nil
}

// Register adds monthly-report routes to a router that must already be
// protected by session, CSRF and safe-origin middleware.
func (h *Controller) Register(router fiber.Router) {
	router.Post("/reports/monthly", h.createMonthly)
	router.Patch("/reports/:reportId/versions/:versionId", h.editDraft)
	router.Post("/reports/:reportId/versions/:versionId/publish", h.publish)
	router.Post("/reports/:reportId/corrections", h.startCorrection)
	router.Post("/reports/:reportId/versions/:versionId/view", h.recordView)
	router.Post("/reports/:reportId/versions/:versionId/acknowledge", h.acknowledge)
}

type sectionRequest struct {
	Text          string `json:"text"`
	OmittedReason string `json:"omittedReason"`
}

type contentRequest struct {
	Learned      sectionRequest `json:"learned"`
	Strengths    sectionRequest `json:"strengths"`
	FocusNext    sectionRequest `json:"focusNext"`
	PracticePlan sectionRequest `json:"practicePlan"`
	CoachComment sectionRequest `json:"coachComment"`
}

type createMonthlyRequest struct {
	StudentID       string         `json:"studentId"`
	Year            int            `json:"year"`
	Month           int            `json:"month"`
	RubricVersionID string         `json:"rubricVersionId"`
	Content         contentRequest `json:"content"`
}

func (h *Controller) createMonthly(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	var request createMonthlyRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if !platformid.ValidReference(request.StudentID) {
		return writeInvalidField(c, "studentId", "must be a UUIDv4 or UUIDv7")
	}
	if request.Year < 1 {
		return writeInvalidField(c, "year", "must be greater than zero")
	}
	if request.Month < int(time.January) || request.Month > int(time.December) {
		return writeInvalidField(c, "month", "must be between 1 and 12")
	}
	if !platformid.ValidReference(request.RubricVersionID) {
		return writeInvalidField(c, "rubricVersionId", "must be a UUIDv4 or UUIDv7")
	}

	reportID, versionID, ok := h.newReportIDs()
	if !ok {
		return writeIDGenerationFailure(c)
	}
	created, err := h.service.CreateMonthly(c.Context(), report.CreateCommand{
		ReportID:        reportID,
		StudentID:       request.StudentID,
		Year:            request.Year,
		Month:           time.Month(request.Month),
		VersionID:       versionID,
		RubricVersionID: request.RubricVersionID,
		Content:         toContent(request.Content),
		ActorID:         actorID,
		CreatedAt:       h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(created, actorID))
}

type editDraftRequest struct {
	Version uint64         `json:"version"`
	Content contentRequest `json:"content"`
}

func (h *Controller) editDraft(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	reportID, versionID, problem := reportVersionPath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	var request editDraftRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}

	updated, err := h.service.EditDraft(c.Context(), report.EditCommand{
		ReportID:         reportID,
		VersionID:        versionID,
		ExpectedRevision: request.Version,
		Content:          toContent(request.Content),
		ActorID:          actorID,
		UpdatedAt:        h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(updated, actorID))
}

type versionMutationRequest struct {
	Version uint64 `json:"version"`
}

func (h *Controller) publish(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	reportID, versionID, problem := reportVersionPath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	var request versionMutationRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}

	updated, err := h.service.Publish(c.Context(), report.PublishCommand{
		ReportID:         reportID,
		VersionID:        versionID,
		ExpectedRevision: request.Version,
		ActorID:          actorID,
		PublishedAt:      h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(updated, actorID))
}

type correctionRequest struct {
	Version         uint64 `json:"version"`
	SourceVersionID string `json:"sourceVersionId"`
}

func (h *Controller) startCorrection(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	reportID := strings.TrimSpace(c.Params("reportId"))
	if !platformid.ValidReference(reportID) {
		return writeInvalidField(c, "reportId", "must be a UUIDv4 or UUIDv7")
	}
	var request correctionRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	if !platformid.ValidReference(request.SourceVersionID) {
		return writeInvalidField(c, "sourceVersionId", "must be a UUIDv4 or UUIDv7")
	}
	newVersionID := h.newID()
	if !isUUIDv7(newVersionID) {
		return writeIDGenerationFailure(c)
	}

	updated, err := h.service.StartCorrection(c.Context(), report.CorrectCommand{
		ReportID:         reportID,
		SourceVersionID:  request.SourceVersionID,
		NewVersionID:     newVersionID,
		ExpectedRevision: request.Version,
		ActorID:          actorID,
		CreatedAt:        h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(updated, actorID))
}

func (h *Controller) recordView(c fiber.Ctx) error {
	return h.recordReceipt(c, false)
}

func (h *Controller) acknowledge(c fiber.Ctx) error {
	return h.recordReceipt(c, true)
}

func (h *Controller) recordReceipt(c fiber.Ctx, isAcknowledgement bool) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	reportID, versionID, problem := reportVersionPath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	var request versionMutationRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	command := report.ReceiptCommand{
		ReportID:         reportID,
		VersionID:        versionID,
		ViewerID:         actorID,
		ExpectedRevision: request.Version,
		OccurredAt:       h.now().UTC(),
	}
	var updated report.Report
	var err error
	if isAcknowledgement {
		updated, err = h.service.Acknowledge(c.Context(), command)
	} else {
		updated, err = h.service.RecordView(c.Context(), command)
	}
	if err != nil {
		return writeDomainError(c, err)
	}
	response := toResponse(updated, actorID)
	// View and acknowledgement authorize published content only. A correction
	// draft in the same aggregate must remain private until it is published.
	response.Versions = slices.DeleteFunc(response.Versions, func(version versionResponse) bool {
		return version.Status != report.VersionStatusPublished
	})
	return c.JSON(response)
}

type sectionResponse struct {
	Text          string `json:"text,omitempty"`
	OmittedReason string `json:"omittedReason,omitempty"`
}

type contentResponse struct {
	Learned      sectionResponse `json:"learned"`
	Strengths    sectionResponse `json:"strengths"`
	FocusNext    sectionResponse `json:"focusNext"`
	PracticePlan sectionResponse `json:"practicePlan"`
	CoachComment sectionResponse `json:"coachComment"`
}

type versionResponse struct {
	ID              string               `json:"id"`
	Number          uint32               `json:"number"`
	Status          report.VersionStatus `json:"status"`
	Content         contentResponse      `json:"content"`
	RubricVersionID string               `json:"rubricVersionId"`
	SourceVersionID string               `json:"sourceVersionId,omitempty"`
	CreatedAt       time.Time            `json:"createdAt"`
	UpdatedAt       time.Time            `json:"updatedAt"`
	PublishedAt     *time.Time           `json:"publishedAt,omitempty"`
}

type receiptResponse struct {
	VersionID      string     `json:"versionId"`
	ViewerID       string     `json:"viewerId"`
	FirstViewedAt  time.Time  `json:"firstViewedAt"`
	AcknowledgedAt *time.Time `json:"acknowledgedAt,omitempty"`
}

type reportResponse struct {
	ID          string            `json:"id"`
	StudentID   string            `json:"studentId"`
	PeriodStart time.Time         `json:"periodStart"`
	PeriodEnd   time.Time         `json:"periodEnd"`
	Version     uint64            `json:"version"`
	Versions    []versionResponse `json:"versions"`
	Receipts    []receiptResponse `json:"receipts"`
}

func toResponse(value report.Report, actorID string) reportResponse {
	versions := make([]versionResponse, 0, len(value.Versions()))
	for _, version := range value.Versions() {
		versions = append(versions, versionResponse{
			ID:              version.ID,
			Number:          version.Number,
			Status:          version.Status,
			Content:         toContentResponse(version.Content),
			RubricVersionID: version.RubricVersionID,
			SourceVersionID: version.SourceVersionID,
			CreatedAt:       version.CreatedAt,
			UpdatedAt:       version.UpdatedAt,
			PublishedAt:     version.PublishedAt,
		})
	}
	receipts := make([]receiptResponse, 0, len(value.Receipts()))
	for _, receipt := range value.Receipts() {
		if receipt.ViewerID != actorID {
			continue
		}
		receipts = append(receipts, receiptResponse{
			VersionID:      receipt.VersionID,
			ViewerID:       receipt.ViewerID,
			FirstViewedAt:  receipt.FirstViewedAt,
			AcknowledgedAt: receipt.AcknowledgedAt,
		})
	}
	period := value.Period()
	return reportResponse{
		ID:          value.ID(),
		StudentID:   value.StudentID(),
		PeriodStart: period.Start,
		PeriodEnd:   period.End,
		Version:     value.Revision(),
		Versions:    versions,
		Receipts:    receipts,
	}
}

func toContent(request contentRequest) report.Content {
	return report.Content{
		Learned:      toSection(request.Learned),
		Strengths:    toSection(request.Strengths),
		FocusNext:    toSection(request.FocusNext),
		PracticePlan: toSection(request.PracticePlan),
		CoachComment: toSection(request.CoachComment),
	}
}

func toSection(request sectionRequest) report.Section {
	return report.Section{
		Text:          strings.TrimSpace(request.Text),
		OmittedReason: strings.TrimSpace(request.OmittedReason),
	}
}

func toContentResponse(content report.Content) contentResponse {
	return contentResponse{
		Learned:      toSectionResponse(content.Learned),
		Strengths:    toSectionResponse(content.Strengths),
		FocusNext:    toSectionResponse(content.FocusNext),
		PracticePlan: toSectionResponse(content.PracticePlan),
		CoachComment: toSectionResponse(content.CoachComment),
	}
}

func toSectionResponse(section report.Section) sectionResponse {
	return sectionResponse{Text: section.Text, OmittedReason: section.OmittedReason}
}

func (h *Controller) authenticatedActor(c fiber.Ctx) (string, bool) {
	actorID, ok := h.actors.ActorID(c)
	actorID = strings.TrimSpace(actorID)
	return actorID, ok && platformid.ValidReference(actorID)
}

func (h *Controller) newReportIDs() (string, string, bool) {
	reportID := h.newID()
	versionID := h.newID()
	return reportID, versionID, platformid.ValidReference(reportID) && platformid.ValidReference(versionID)
}

func reportVersionPath(c fiber.Ctx) (string, string, *httpx.Problem) {
	reportID := strings.TrimSpace(c.Params("reportId"))
	if !platformid.ValidReference(reportID) {
		problem := invalidFieldProblem("reportId", "must be a UUIDv4 or UUIDv7")
		return "", "", &problem
	}
	versionID := strings.TrimSpace(c.Params("versionId"))
	if !platformid.ValidReference(versionID) {
		problem := invalidFieldProblem("versionId", "must be a UUIDv4 or UUIDv7")
		return "", "", &problem
	}
	return reportID, versionID, nil
}

func decodeStrict(body []byte, target any) error {
	if len(body) == 0 {
		return httpx.NewProblem(
			fiber.StatusBadRequest,
			"invalid_json",
			"Invalid JSON",
			"A JSON request body is required",
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return httpx.NewProblem(
			fiber.StatusBadRequest,
			"invalid_json",
			"Invalid JSON",
			"The request body is not valid JSON",
		)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return httpx.NewProblem(
			fiber.StatusBadRequest,
			"invalid_json",
			"Invalid JSON",
			"The request body must contain exactly one JSON object",
		)
	}
	return nil
}

func isUUIDv7(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.Version() == 7
}

func writeUnauthenticated(c fiber.Ctx) error {
	return writeProblem(c, httpx.NewProblem(
		fiber.StatusUnauthorized,
		"unauthenticated",
		"Authentication required",
		"Sign in to continue",
	))
}

func writeIDGenerationFailure(c fiber.Ctx) error {
	return writeProblem(c, httpx.NewProblem(
		fiber.StatusInternalServerError,
		"id_generation_failed",
		"Identifier generation failed",
		"The request could not be completed",
	))
}

func writeInvalidField(c fiber.Ctx, field, message string) error {
	return writeProblem(c, invalidFieldProblem(field, message))
}

func invalidFieldProblem(field, message string) httpx.Problem {
	problem := httpx.NewProblem(
		fiber.StatusUnprocessableEntity,
		"invalid_report",
		"Report details are invalid",
		"Correct the highlighted fields and retry",
	)
	problem.Errors = append(problem.Errors, httpx.FieldError{Field: field, Message: message})
	return problem
}

func writeDomainError(c fiber.Ctx, err error) error {
	var problem httpx.Problem
	switch {
	case errors.Is(err, report.ErrPermissionDenied):
		problem = httpx.NewProblem(
			fiber.StatusForbidden,
			"report_forbidden",
			"Report access denied",
			"You cannot perform this report action for the student",
		)
	case errors.Is(err, report.ErrNotFound),
		errors.Is(err, report.ErrDraftNotFound),
		errors.Is(err, report.ErrVersionNotFound):
		problem = httpx.NewProblem(
			fiber.StatusNotFound,
			"report_not_found",
			"Report not found",
			"The requested report or version does not exist",
		)
	case errors.Is(err, report.ErrConflict),
		errors.Is(err, report.ErrDraftExists),
		errors.Is(err, report.ErrPublishedImmutable):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"report_version_conflict",
			"Report changed",
			"Refresh the report and retry with its current version",
		)
	case errors.Is(err, report.ErrInvalidCommand), errors.Is(err, report.ErrInvalidReport):
		problem = invalidFieldProblem("request", "contains an invalid report change")
	case errors.Is(err, context.DeadlineExceeded):
		problem = httpx.NewProblem(
			fiber.StatusGatewayTimeout,
			"request_timeout",
			"Request timed out",
			"Retry the request",
		)
	default:
		return writeProblem(c, err)
	}
	return writeProblem(c, problem)
}

func writeProblem(c fiber.Ctx, err error) error {
	writeErr := httpx.WriteProblem(c, err)
	// Fiber's JSON helper overwrites a custom media type. Restore the contract
	// after httpx has serialized the RFC 9457 response.
	c.Set(fiber.HeaderContentType, "application/problem+json")
	return writeErr
}
