package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type Service interface {
	Grant(context.Context, family.GrantCommand) (family.Relationship, error)
	ReplacePermissions(
		context.Context,
		family.ReplacePermissionsCommand,
	) (family.Relationship, error)
	Revoke(context.Context, family.RevokeCommand) (family.Relationship, error)
}

// ActorResolver reads the authenticated identity established by auth
// middleware. It deliberately exposes no Fiber value to the domain service.
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
		return nil, fmt.Errorf("family controller: all dependencies are required")
	}
	return &Controller{
		service: dependencies.Service,
		actors:  dependencies.Actors,
		newID:   dependencies.NewID,
		now:     dependencies.Now,
	}, nil
}

// Register adds relationship routes to a router that must already be protected
// by session, CSRF and safe-origin middleware.
func (h *Controller) Register(router fiber.Router) {
	router.Post("/families/:familyId/guardian-relationships", h.grant)
	router.Patch(
		"/families/:familyId/guardian-relationships/:relationshipId/permissions",
		h.replacePermissions,
	)
	router.Post(
		"/families/:familyId/guardian-relationships/:relationshipId/revoke",
		h.revoke,
	)
}

type grantRequest struct {
	GuardianID  string   `json:"guardianId"`
	StudentID   string   `json:"studentId"`
	Permissions []string `json:"permissions"`
}

func (h *Controller) grant(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	familyID := strings.TrimSpace(c.Params("familyId"))
	if !platformid.ValidReference(familyID) {
		return writeInvalidField(c, "familyId", "must be a UUIDv4 or UUIDv7")
	}

	var request grantRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	if !platformid.ValidReference(request.GuardianID) {
		return writeInvalidField(c, "guardianId", "must be a UUIDv4 or UUIDv7")
	}
	if !platformid.ValidReference(request.StudentID) {
		return writeInvalidField(c, "studentId", "must be a UUIDv4 or UUIDv7")
	}
	permissions := parsePermissions(request.Permissions)
	if len(permissions) == 0 {
		return writeInvalidField(c, "permissions", "at least one permission is required")
	}

	relationshipID := h.newID()
	eventID := h.newID()
	if !isUUIDv7(relationshipID) || !isUUIDv7(eventID) {
		return writeIDGenerationFailure(c)
	}
	relationship, err := h.service.Grant(c.Context(), family.GrantCommand{
		RelationshipID: relationshipID,
		EventID:        eventID,
		FamilyID:       familyID,
		GuardianID:     request.GuardianID,
		StudentID:      request.StudentID,
		Permissions:    permissions,
		ActorID:        actorID,
		OccurredAt:     h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(toResponse(relationship))
}

type replacePermissionsRequest struct {
	Version     uint64   `json:"version"`
	Permissions []string `json:"permissions"`
	Reason      string   `json:"reason"`
}

func (h *Controller) replacePermissions(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	familyID, relationshipID, problem := relationshipPath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}

	var request replacePermissionsRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	permissions := parsePermissions(request.Permissions)
	reason := strings.TrimSpace(request.Reason)
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	if len(permissions) == 0 {
		return writeInvalidField(c, "permissions", "at least one permission is required")
	}
	if reason == "" {
		return writeInvalidField(c, "reason", "is required")
	}
	eventID := h.newID()
	if !isUUIDv7(eventID) {
		return writeIDGenerationFailure(c)
	}

	relationship, err := h.service.ReplacePermissions(
		c.Context(),
		family.ReplacePermissionsCommand{
			FamilyID:         familyID,
			RelationshipID:   relationshipID,
			EventID:          eventID,
			ExpectedRevision: request.Version,
			Permissions:      permissions,
			ActorID:          actorID,
			Reason:           reason,
			OccurredAt:       h.now().UTC(),
		},
	)
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(relationship))
}

type revokeRequest struct {
	Version uint64 `json:"version"`
	Reason  string `json:"reason"`
}

func (h *Controller) revoke(c fiber.Ctx) error {
	actorID, ok := h.authenticatedActor(c)
	if !ok {
		return writeUnauthenticated(c)
	}
	familyID, relationshipID, problem := relationshipPath(c)
	if problem != nil {
		return writeProblem(c, *problem)
	}

	var request revokeRequest
	if err := decodeStrict(c.Body(), &request); err != nil {
		return writeProblem(c, err)
	}
	reason := strings.TrimSpace(request.Reason)
	if request.Version == 0 {
		return writeInvalidField(c, "version", "must be greater than zero")
	}
	if reason == "" {
		return writeInvalidField(c, "reason", "is required")
	}
	eventID := h.newID()
	if !isUUIDv7(eventID) {
		return writeIDGenerationFailure(c)
	}

	relationship, err := h.service.Revoke(c.Context(), family.RevokeCommand{
		FamilyID:         familyID,
		RelationshipID:   relationshipID,
		EventID:          eventID,
		ExpectedRevision: request.Version,
		ActorID:          actorID,
		Reason:           reason,
		OccurredAt:       h.now().UTC(),
	})
	if err != nil {
		return writeDomainError(c, err)
	}
	return c.JSON(toResponse(relationship))
}

type relationshipResponse struct {
	ID          string                    `json:"id"`
	FamilyID    string                    `json:"familyId"`
	GuardianID  string                    `json:"guardianId"`
	StudentID   string                    `json:"studentId"`
	Permissions []family.Permission       `json:"permissions"`
	Status      family.RelationshipStatus `json:"status"`
	Version     uint64                    `json:"version"`
	GrantedAt   time.Time                 `json:"grantedAt"`
	RevokedAt   *time.Time                `json:"revokedAt,omitempty"`
}

func toResponse(relationship family.Relationship) relationshipResponse {
	record := relationship.Record()
	return relationshipResponse{
		ID:          record.ID,
		FamilyID:    record.FamilyID,
		GuardianID:  record.GuardianID,
		StudentID:   record.StudentID,
		Permissions: record.Permissions,
		Status:      record.Status,
		Version:     record.Revision,
		GrantedAt:   record.GrantedAt,
		RevokedAt:   record.RevokedAt,
	}
}

func (h *Controller) authenticatedActor(c fiber.Ctx) (string, bool) {
	actorID, ok := h.actors.ActorID(c)
	actorID = strings.TrimSpace(actorID)
	return actorID, ok && platformid.ValidReference(actorID)
}

func relationshipPath(c fiber.Ctx) (string, string, *httpx.Problem) {
	familyID := strings.TrimSpace(c.Params("familyId"))
	if !platformid.ValidReference(familyID) {
		problem := invalidFieldProblem("familyId", "must be a UUIDv4 or UUIDv7")
		return "", "", &problem
	}
	relationshipID := strings.TrimSpace(c.Params("relationshipId"))
	if !platformid.ValidReference(relationshipID) {
		problem := invalidFieldProblem("relationshipId", "must be a UUIDv4 or UUIDv7")
		return "", "", &problem
	}
	return familyID, relationshipID, nil
}

func parsePermissions(values []string) []family.Permission {
	permissions := make([]family.Permission, 0, len(values))
	for _, value := range values {
		permissions = append(permissions, family.Permission(strings.TrimSpace(value)))
	}
	return permissions
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
		"invalid_relationship",
		"Relationship details are invalid",
		"Correct the highlighted fields and retry",
	)
	problem.Errors = append(problem.Errors, httpx.FieldError{Field: field, Message: message})
	return problem
}

func writeDomainError(c fiber.Ctx, err error) error {
	var problem httpx.Problem
	switch {
	case errors.Is(err, family.ErrPermissionDenied):
		problem = httpx.NewProblem(
			fiber.StatusForbidden,
			"relationship_forbidden",
			"Relationship access denied",
			"You cannot manage guardian access for this family",
		)
	case errors.Is(err, family.ErrNotFound):
		problem = httpx.NewProblem(
			fiber.StatusNotFound,
			"relationship_not_found",
			"Relationship not found",
			"The guardian relationship does not exist in this family",
		)
	case errors.Is(err, family.ErrConflict):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"relationship_version_conflict",
			"Relationship changed",
			"Refresh the relationship and retry with its current version",
		)
	case errors.Is(err, family.ErrInvalidCommand),
		errors.Is(err, family.ErrInvalidRelationship),
		errors.Is(err, family.ErrInactiveRelationship):
		problem = invalidFieldProblem("request", "contains an invalid relationship change")
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
