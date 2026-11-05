package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Service interface {
	ViewBalance(context.Context, string, string) (entitlement.AccountBalance, error)
	History(context.Context, entitlement.HistoryCommand) (entitlement.EntryPage, error)
	GrantForActor(context.Context, entitlement.GrantCommand) (entitlement.ApplyResult, error)
}

type ActorResolver interface {
	ActorID(fiber.Ctx) (string, bool)
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
		return nil, errors.New("entitlement controller: all dependencies are required")
	}
	return &Controller{
		service: dependencies.Service,
		actors:  dependencies.Actors,
		newID:   dependencies.NewID,
		now:     dependencies.Now,
	}, nil
}

// Register adds routes to a router whose /entitlements boundary already
// enforces authentication, CSRF on unsafe methods, and recent MFA on grants.
func (h *Controller) Register(router fiber.Router) {
	router.Get("/entitlements/accounts/:accountId", h.balance)
	router.Get("/entitlements/accounts/:accountId/entries", h.history)
	router.Post("/entitlements/accounts/:accountId/grants", h.grant)
}

func (h *Controller) balance(c fiber.Ctx) error {
	actorID, accountID, problem := h.requestIdentity(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	result, err := h.service.ViewBalance(c.Context(), actorID, accountID)
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toAccountBalanceResponse(result))
}

func (h *Controller) history(c fiber.Ctx) error {
	actorID, accountID, problem := h.requestIdentity(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	limit, problem := parseLimit(c.Query("limit"))
	if problem != nil {
		return writeProblem(c, *problem)
	}
	cursor, problem := decodeCursor(strings.TrimSpace(c.Query("cursor")))
	if problem != nil {
		return writeProblem(c, *problem)
	}
	page, err := h.service.History(c.Context(), entitlement.HistoryCommand{
		ActorID:   actorID,
		AccountID: accountID,
		Cursor:    cursor,
		Limit:     limit,
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	items := make([]entryResponse, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toEntryResponse(item))
	}
	response := entryPageResponse{Items: items}
	if page.NextCursor != nil {
		response.NextCursor = encodeCursor(*page.NextCursor)
	}
	return c.JSON(response)
}

type grantRequest struct {
	Units       int64  `json:"units"`
	ReferenceID string `json:"referenceId"`
	Reason      string `json:"reason"`
}

func (h *Controller) grant(c fiber.Ctx) error {
	actorID, accountID, problem := h.requestIdentity(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}
	idempotencyKey := strings.TrimSpace(c.Get("Idempotency-Key"))
	if len(idempotencyKey) < 16 || len(idempotencyKey) > 255 {
		return writeInvalidField(c, "Idempotency-Key", "must contain 16 to 255 characters")
	}
	var request grantRequest
	if err := httpx.DecodeStrictJSON(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	referenceID := strings.TrimSpace(request.ReferenceID)
	reason := strings.TrimSpace(request.Reason)
	switch {
	case request.Units < 1:
		return writeInvalidField(c, "units", "must be a positive integer")
	case referenceID == "" || len(referenceID) > 255:
		return writeInvalidField(c, "referenceId", "must contain 1 to 255 characters")
	case reason == "" || utf8.RuneCountInString(reason) > 500:
		return writeInvalidField(c, "reason", "must contain 1 to 500 characters")
	}
	entryID := h.newID()
	if !platformid.Valid(entryID) {
		return writeProblem(c, httpx.NewProblem(
			fiber.StatusInternalServerError,
			"id_generation_failed",
			"Identifier generation failed",
			"The request could not be completed",
		))
	}
	result, err := h.service.GrantForActor(c.Context(), entitlement.GrantCommand{
		EntryID:        entryID,
		AccountID:      accountID,
		IdempotencyKey: idempotencyKey,
		ReferenceID:    referenceID,
		Units:          request.Units,
		Reason:         reason,
		ActorID:        actorID,
		OccurredAt:     h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	status := fiber.StatusOK
	if result.Created {
		status = fiber.StatusCreated
	}
	return c.Status(status).JSON(applyResponse{
		Entry:   toEntryResponse(result.Entry),
		Balance: toBalanceResponse(result.Balance),
		Created: result.Created,
	})
}

func (h *Controller) requestIdentity(c fiber.Ctx) (string, string, *httpx.Problem) {
	actorID, ok := h.actors.ActorID(c)
	actorID = strings.TrimSpace(actorID)
	if !ok || !platformid.ValidReference(actorID) {
		problem := httpx.NewProblem(
			fiber.StatusUnauthorized,
			"unauthenticated",
			"Authentication required",
			"Sign in to continue",
		)
		return "", "", &problem
	}
	accountID := strings.TrimSpace(c.Params("accountId"))
	if !platformid.ValidReference(accountID) {
		problem := invalidFieldProblem("accountId", "must be a UUIDv4 or UUIDv7")
		return "", "", &problem
	}
	return actorID, accountID, nil
}

type accountResponse struct {
	ID          string    `json:"id"`
	EnrolmentID string    `json:"enrolmentId"`
	StudentID   string    `json:"studentId"`
	Unit        string    `json:"unit"`
	CreatedAt   time.Time `json:"createdAt"`
}

type balanceResponse struct {
	Available int64 `json:"available"`
	Reserved  int64 `json:"reserved"`
	Consumed  int64 `json:"consumed"`
}

type deltaResponse struct {
	Available int64 `json:"available"`
	Reserved  int64 `json:"reserved"`
	Consumed  int64 `json:"consumed"`
}

type accountBalanceResponse struct {
	Account accountResponse `json:"account"`
	Balance balanceResponse `json:"balance"`
}

type entryResponse struct {
	ID            string                `json:"id"`
	AccountID     string                `json:"accountId"`
	StudentID     string                `json:"studentId"`
	Kind          entitlement.EntryKind `json:"kind"`
	Delta         deltaResponse         `json:"delta"`
	ReferenceType string                `json:"referenceType"`
	ReferenceID   string                `json:"referenceId"`
	Reason        string                `json:"reason"`
	ActorID       string                `json:"actorId"`
	OccurredAt    time.Time             `json:"occurredAt"`
}

type entryPageResponse struct {
	Items      []entryResponse `json:"items"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

type applyResponse struct {
	Entry   entryResponse   `json:"entry"`
	Balance balanceResponse `json:"balance"`
	Created bool            `json:"created"`
}

func toAccountBalanceResponse(value entitlement.AccountBalance) accountBalanceResponse {
	return accountBalanceResponse{
		Account: accountResponse{
			ID:          value.Account.ID,
			EnrolmentID: value.Account.EnrolmentID,
			StudentID:   value.Account.StudentID,
			Unit:        value.Account.Unit,
			CreatedAt:   value.Account.CreatedAt,
		},
		Balance: toBalanceResponse(value.Balance),
	}
}

func toBalanceResponse(value entitlement.Balance) balanceResponse {
	return balanceResponse{
		Available: value.Available,
		Reserved:  value.Reserved,
		Consumed:  value.Consumed,
	}
}

func toEntryResponse(value entitlement.Entry) entryResponse {
	return entryResponse{
		ID:        value.ID,
		AccountID: value.AccountID,
		StudentID: value.StudentID,
		Kind:      value.Kind,
		Delta: deltaResponse{
			Available: value.Delta.Available,
			Reserved:  value.Delta.Reserved,
			Consumed:  value.Delta.Consumed,
		},
		ReferenceType: value.ReferenceType,
		ReferenceID:   value.ReferenceID,
		Reason:        value.Reason,
		ActorID:       value.ActorID,
		OccurredAt:    value.OccurredAt,
	}
}

func parseLimit(value string) (int, *httpx.Problem) {
	if strings.TrimSpace(value) == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(value)
	if err != nil || limit < 1 || limit > 100 {
		problem := invalidFieldProblem("limit", "must be an integer between 1 and 100")
		return 0, &problem
	}
	return limit, nil
}

func encodeCursor(cursor entitlement.EntryCursor) string {
	value := cursor.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + cursor.ID
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func decodeCursor(value string) (*entitlement.EntryCursor, *httpx.Problem) {
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
	occurredAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		problem := invalidFieldProblem("cursor", "is invalid")
		return nil, &problem
	}
	return &entitlement.EntryCursor{OccurredAt: occurredAt.UTC(), ID: parts[1]}, nil
}

func writeInvalidField(c fiber.Ctx, field, message string) error {
	return writeProblem(c, invalidFieldProblem(field, message))
}

func invalidFieldProblem(field, message string) httpx.Problem {
	problem := httpx.NewProblem(
		fiber.StatusUnprocessableEntity,
		"invalid_entitlement_request",
		"Entitlement request is invalid",
		"Correct the highlighted fields and retry",
	)
	problem.Errors = append(problem.Errors, httpx.FieldError{Field: field, Message: message})
	return problem
}

func writeDomainError(c fiber.Ctx, err error) error {
	var problem httpx.Problem
	switch {
	case errors.Is(err, entitlement.ErrPermissionDenied):
		problem = httpx.NewProblem(
			fiber.StatusForbidden,
			"entitlement_forbidden",
			"Entitlement access denied",
			"You cannot access or change this entitlement account",
		)
	case errors.Is(err, entitlement.ErrAccountNotFound):
		problem = httpx.NewProblem(
			fiber.StatusNotFound,
			"entitlement_account_not_found",
			"Entitlement account not found",
			"The entitlement account does not exist",
		)
	case errors.Is(err, entitlement.ErrIdempotencyConflict):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"idempotency_conflict",
			"Idempotency key conflict",
			"Use a new idempotency key for a different grant",
		)
	case errors.Is(err, entitlement.ErrEntryConflict):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"entitlement_entry_conflict",
			"Entitlement entry conflict",
			"Refresh the account and retry",
		)
	case errors.Is(err, entitlement.ErrInsufficientBalance):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"insufficient_entitlement_balance",
			"Insufficient entitlement balance",
			"The requested transfer would make a balance negative",
		)
	case errors.Is(err, entitlement.ErrBalanceOverflow):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"entitlement_balance_overflow",
			"Entitlement balance is out of range",
			"The requested transfer cannot be represented safely",
		)
	case errors.Is(err, entitlement.ErrInvalidCommand),
		errors.Is(err, entitlement.ErrInvalidEntry):
		problem = invalidFieldProblem("request", "contains an invalid entitlement change")
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
