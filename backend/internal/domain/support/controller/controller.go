package controller

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/support"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Service interface {
	Create(context.Context, support.CreateCommand) (support.Case, error)
	List(context.Context, support.ListCommand) (support.Page, error)
	View(context.Context, string, string) (support.Case, error)
	Assign(context.Context, support.AssignCommand) (support.Case, error)
	ChangeStatus(context.Context, support.ChangeStatusCommand) (support.Case, error)
	AppendEvent(context.Context, support.AppendEventCommand) (support.Case, error)
}

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
		return nil, errors.New("support controller: all dependencies are required")
	}
	return &Controller{
		service: dependencies.Service,
		actors:  dependencies.Actors,
		newID:   dependencies.NewID,
		now:     dependencies.Now,
	}, nil
}

// Register adds routes to a router that must already enforce authentication,
// CSRF and safe-origin middleware. Authorization remains in support.Service.
func (h *Controller) Register(router fiber.Router) {
	router.Post("/support/cases", h.create)
	router.Get("/support/cases", h.list)
	router.Get("/support/cases/:caseId", h.view)
	router.Post("/support/cases/:caseId/events", h.appendEvent)
	router.Patch("/support/cases/:caseId/assignment", h.assign)
	router.Patch("/support/cases/:caseId/status", h.changeStatus)
}

type createRequest struct {
	FamilyID string `json:"familyId"`
	Category string `json:"category"`
	Severity string `json:"severity"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
}

func (h *Controller) create(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	var request createRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if problem := validateCreateRequest(request); problem != nil {
		return writeProblem(c, *problem)
	}
	caseID, eventID := h.newID(), h.newID()
	if !platformid.Valid(caseID) || !platformid.Valid(eventID) {
		return writeIDGenerationFailure(c)
	}
	created, err := h.service.Create(c.Context(), support.CreateCommand{
		CaseID:     caseID,
		EventID:    eventID,
		FamilyID:   strings.TrimSpace(request.FamilyID),
		Category:   support.Category(strings.TrimSpace(request.Category)),
		Severity:   support.Severity(strings.TrimSpace(request.Severity)),
		Subject:    strings.TrimSpace(request.Subject),
		Message:    strings.TrimSpace(request.Message),
		ActorID:    actorID,
		OccurredAt: h.now().UTC(),
		RequestID:  httpx.RequestID(c),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(created))
}

func (h *Controller) list(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	familyID := strings.TrimSpace(c.Query("familyId"))
	if !platformid.ValidReference(familyID) {
		return writeInvalidField(c, "familyId", "must be a UUIDv4 or UUIDv7")
	}
	limit, problem := parseLimit(c.Query("limit"))
	if problem != nil {
		return writeProblem(c, *problem)
	}
	cursor, problem := decodeCursor(strings.TrimSpace(c.Query("cursor")))
	if problem != nil {
		return writeProblem(c, *problem)
	}
	page, err := h.service.List(c.Context(), support.ListCommand{
		FamilyID: familyID,
		ActorID:  actorID,
		Options: support.ListOptions{
			Limit:  limit,
			Cursor: cursor,
		},
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	items := make([]caseResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toResponse(item))
	}
	nextCursor := ""
	if page.NextCursor != nil {
		nextCursor = encodeCursor(*page.NextCursor)
	}
	return c.JSON(pageResponse{Items: items, NextCursor: nextCursor})
}

func (h *Controller) view(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	caseID, problem := casePath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	loaded, err := h.service.View(c.Context(), actorID, caseID)
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(loaded))
}

type appendEventRequest struct {
	Version    uint64 `json:"version"`
	Visibility string `json:"visibility"`
	Body       string `json:"body"`
}

func (h *Controller) appendEvent(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	caseID, problem := casePath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	var request appendEventRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	visibility := support.Visibility(strings.TrimSpace(request.Visibility))
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	if !visibility.Valid() {
		return writeInvalidField(c, "visibility", "must be customer_visible or internal")
	}
	body := strings.TrimSpace(request.Body)
	if body == "" || utf8.RuneCountInString(body) > 10000 {
		return writeInvalidField(c, "body", "must contain 1 to 10000 characters")
	}
	eventID := h.newID()
	if !platformid.Valid(eventID) {
		return writeIDGenerationFailure(c)
	}
	updated, err := h.service.AppendEvent(c.Context(), support.AppendEventCommand{
		CaseID:          caseID,
		EventID:         eventID,
		ExpectedVersion: request.Version,
		Visibility:      visibility,
		Body:            body,
		ActorID:         actorID,
		OccurredAt:      h.now().UTC(),
		RequestID:       httpx.RequestID(c),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(updated))
}

type assignRequest struct {
	Version    uint64 `json:"version"`
	AssigneeID string `json:"assigneeId"`
	Reason     string `json:"reason"`
}

func (h *Controller) assign(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	caseID, problem := casePath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	var request assignRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	assigneeID := strings.TrimSpace(request.AssigneeID)
	if !platformid.ValidReference(assigneeID) {
		return writeInvalidField(c, "assigneeId", "must be a UUIDv4 or UUIDv7")
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return writeInvalidField(c, "reason", "must contain 1 to 500 characters")
	}
	eventID := h.newID()
	if !platformid.Valid(eventID) {
		return writeIDGenerationFailure(c)
	}
	updated, err := h.service.Assign(c.Context(), support.AssignCommand{
		CaseID:          caseID,
		EventID:         eventID,
		ExpectedVersion: request.Version,
		AssigneeID:      assigneeID,
		ActorID:         actorID,
		Reason:          reason,
		OccurredAt:      h.now().UTC(),
		RequestID:       httpx.RequestID(c),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(updated))
}

type statusRequest struct {
	Version uint64 `json:"version"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
}

func (h *Controller) changeStatus(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	caseID, problem := casePath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	var request statusRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	status := support.Status(strings.TrimSpace(request.Status))
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	if !status.Valid() {
		return writeInvalidField(c, "status", "is not a supported case status")
	}
	reason := strings.TrimSpace(request.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return writeInvalidField(c, "reason", "must contain 1 to 500 characters")
	}
	eventID := h.newID()
	if !platformid.Valid(eventID) {
		return writeIDGenerationFailure(c)
	}
	updated, err := h.service.ChangeStatus(c.Context(), support.ChangeStatusCommand{
		CaseID:          caseID,
		EventID:         eventID,
		ExpectedVersion: request.Version,
		Status:          status,
		ActorID:         actorID,
		Reason:          reason,
		OccurredAt:      h.now().UTC(),
		RequestID:       httpx.RequestID(c),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(updated))
}

type eventResponse struct {
	ID         string             `json:"id"`
	Type       support.EventType  `json:"type"`
	Visibility support.Visibility `json:"visibility"`
	ActorID    string             `json:"actorId"`
	Body       string             `json:"body,omitempty"`
	Metadata   map[string]string  `json:"metadata"`
	OccurredAt time.Time          `json:"occurredAt"`
}

type caseResponse struct {
	ID         string           `json:"id"`
	FamilyID   string           `json:"familyId"`
	OpenedBy   string           `json:"openedBy"`
	Category   support.Category `json:"category"`
	Severity   support.Severity `json:"severity"`
	Status     support.Status   `json:"status"`
	Subject    string           `json:"subject"`
	AssignedTo string           `json:"assignedTo,omitempty"`
	OpenedAt   time.Time        `json:"openedAt"`
	SLADueAt   time.Time        `json:"slaDueAt"`
	ResolvedAt *time.Time       `json:"resolvedAt,omitempty"`
	UpdatedAt  time.Time        `json:"updatedAt"`
	Version    uint64           `json:"version"`
	Events     []eventResponse  `json:"events"`
}

type pageResponse struct {
	Items      []caseResponse `json:"items"`
	NextCursor string         `json:"nextCursor,omitempty"`
}

func toResponse(supportCase support.Case) caseResponse {
	events := make([]eventResponse, 0, len(supportCase.Events))
	for _, event := range supportCase.Events {
		events = append(events, eventResponse{
			ID:         event.ID,
			Type:       event.Type,
			Visibility: event.Visibility,
			ActorID:    event.ActorID,
			Body:       event.Body,
			Metadata:   event.Metadata,
			OccurredAt: event.OccurredAt,
		})
	}
	return caseResponse{
		ID:         supportCase.ID,
		FamilyID:   supportCase.FamilyID,
		OpenedBy:   supportCase.OpenedBy,
		Category:   supportCase.Category,
		Severity:   supportCase.Severity,
		Status:     supportCase.Status,
		Subject:    supportCase.Subject,
		AssignedTo: supportCase.AssignedTo,
		OpenedAt:   supportCase.OpenedAt,
		SLADueAt:   supportCase.SLADueAt,
		ResolvedAt: supportCase.ResolvedAt,
		UpdatedAt:  supportCase.UpdatedAt,
		Version:    supportCase.Version,
		Events:     events,
	}
}

func validateCreateRequest(request createRequest) *httpx.Problem {
	if !platformid.ValidReference(strings.TrimSpace(request.FamilyID)) {
		problem := invalidFieldProblem("familyId", "must be a UUIDv4 or UUIDv7")
		return &problem
	}
	if !support.Category(strings.TrimSpace(request.Category)).Valid() {
		problem := invalidFieldProblem("category", "is not a supported category")
		return &problem
	}
	if !support.Severity(strings.TrimSpace(request.Severity)).Valid() {
		problem := invalidFieldProblem("severity", "is not a supported severity")
		return &problem
	}
	subjectLength := utf8.RuneCountInString(strings.TrimSpace(request.Subject))
	if subjectLength < 1 || subjectLength > 200 {
		problem := invalidFieldProblem("subject", "must contain 1 to 200 characters")
		return &problem
	}
	messageLength := utf8.RuneCountInString(strings.TrimSpace(request.Message))
	if messageLength < 1 || messageLength > 10000 {
		problem := invalidFieldProblem("message", "must contain 1 to 10000 characters")
		return &problem
	}
	return nil
}

func (h *Controller) authenticatedActor(c fiber.Ctx) (string, bool) {
	actorID, ok := h.actors.ActorID(c)
	actorID = strings.TrimSpace(actorID)
	return actorID, ok && platformid.ValidReference(actorID)
}

func casePath(c fiber.Ctx) (string, *httpx.Problem) {
	caseID := strings.TrimSpace(c.Params("caseId"))
	if !platformid.ValidReference(caseID) {
		problem := invalidFieldProblem("caseId", "must be a UUIDv4 or UUIDv7")
		return "", &problem
	}
	return caseID, nil
}

func parseLimit(value string) (int, *httpx.Problem) {
	if strings.TrimSpace(value) == "" {
		return 20, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		problem := invalidFieldProblem("limit", "must be an integer between 1 and 100")
		return 0, &problem
	}
	return limit, nil
}

func encodeCursor(cursor support.Cursor) string {
	value := cursor.OpenedAt.UTC().Format(time.RFC3339Nano) + "|" + cursor.ID
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCursor(value string) (*support.Cursor, *httpx.Problem) {
	if value == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		problem := invalidFieldProblem("cursor", "is invalid")
		return nil, &problem
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || !platformid.ValidReference(parts[1]) {
		problem := invalidFieldProblem("cursor", "is invalid")
		return nil, &problem
	}
	openedAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		problem := invalidFieldProblem("cursor", "is invalid")
		return nil, &problem
	}
	return &support.Cursor{OpenedAt: openedAt.UTC(), ID: parts[1]}, nil
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
		"invalid_support_case",
		"Support case details are invalid",
		"Correct the highlighted fields and retry",
	)
	problem.Errors = append(problem.Errors, httpx.FieldError{Field: field, Message: message})
	return problem
}

func writeDomainError(c fiber.Ctx, err error) error {
	var problem httpx.Problem
	switch {
	case errors.Is(err, support.ErrPermissionDenied):
		problem = httpx.NewProblem(
			fiber.StatusForbidden,
			"support_case_forbidden",
			"Support case access denied",
			"You cannot access or change this support case",
		)
	case errors.Is(err, support.ErrNotFound):
		problem = httpx.NewProblem(
			fiber.StatusNotFound,
			"support_case_not_found",
			"Support case not found",
			"The support case does not exist",
		)
	case errors.Is(err, support.ErrConflict):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"support_case_version_conflict",
			"Support case changed",
			"Refresh the support case and retry with its current version",
		)
	case errors.Is(err, support.ErrInvalidCommand), errors.Is(err, support.ErrInvalidCase):
		problem = invalidFieldProblem("request", "contains an invalid support case change")
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
	c.Set(fiber.HeaderContentType, "application/problem+json")
	return writeErr
}
