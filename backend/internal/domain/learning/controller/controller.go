package controller

import (
	"context"
	"errors"
	"io"
	"mime"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/learning"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
)

type Service interface {
	Workspace(context.Context, string) (learning.Workspace, error)
	Action(context.Context, string, learning.Command) (learning.Workspace, error)
}
type ActorResolver interface {
	ActorID(fiber.Ctx) (string, bool)
}
type FileService interface {
	UploadFile(context.Context, string, learning.UploadCommand, io.Reader) (learning.UploadResult, error)
	DownloadFile(context.Context, string, string) (learning.File, io.ReadCloser, error)
}
type Controller struct {
	service Service
	actors  ActorResolver
}

func New(service Service, actors ActorResolver) (*Controller, error) {
	if service == nil || actors == nil {
		return nil, errors.New("learning controller: service and actor resolver are required")
	}
	return &Controller{service: service, actors: actors}, nil
}

// Register requires existing authentication, CSRF on unsafe methods and origin middleware.
func (h *Controller) Register(router fiber.Router) {
	router.Get("/learning/workspace", h.workspace)
	router.Post("/learning/actions", h.action)
	router.Post("/learning/files", h.upload)
	router.Get("/learning/files/:fileId/download", h.download)
}
func (h *Controller) upload(c fiber.Ctx) error {
	actor, ok := h.actors.ActorID(c)
	if !ok {
		return problem(c, learning.ErrDenied)
	}
	service, ok := h.service.(FileService)
	if !ok {
		return problem(c, learning.ValidationError{Detail: "file uploads are not configured"})
	}
	header, err := c.FormFile("file")
	if err != nil {
		return problem(c, learning.ValidationError{Detail: "choose a file to upload"})
	}
	if header.Size > 20<<20 {
		return problem(c, learning.ValidationError{Detail: "files must be smaller than 20 MB"})
	}
	reader, err := header.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	tags := []string{}
	for _, tag := range strings.Split(c.FormValue("tags"), ",") {
		if value := strings.TrimSpace(tag); value != "" {
			tags = append(tags, value)
		}
	}
	key := c.Get("Idempotency-Key")
	if len(key) > 200 {
		return problem(c, learning.ValidationError{Detail: "request key is too long"})
	}
	result, err := service.UploadFile(idempotency.WithKey(c.Context(), key), actor, learning.UploadCommand{Filename: header.Filename, Title: c.FormValue("title"), Visibility: c.FormValue("visibility"), LevelSlug: c.FormValue("levelSlug"), AssignmentID: c.FormValue("assignmentId"), CertificateID: c.FormValue("certificateId"), Tags: tags}, reader)
	if err != nil {
		return problem(c, err)
	}
	return c.Status(201).JSON(result)
}
func (h *Controller) download(c fiber.Ctx) error {
	actor, ok := h.actors.ActorID(c)
	if !ok {
		return problem(c, learning.ErrDenied)
	}
	service, ok := h.service.(FileService)
	if !ok {
		return problem(c, learning.ValidationError{Detail: "file downloads are not configured"})
	}
	file, body, err := service.DownloadFile(c.Context(), actor, c.Params("fileId"))
	if err != nil {
		return problem(c, err)
	}
	c.Set(fiber.HeaderContentType, file.MediaType)
	c.Set(fiber.HeaderContentDisposition, mime.FormatMediaType("attachment", map[string]string{"filename": file.Filename}))
	c.Set(fiber.HeaderCacheControl, "private, no-store")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.SendStream(body, int(file.SizeBytes))
}
func (h *Controller) workspace(c fiber.Ctx) error {
	actor, ok := h.actors.ActorID(c)
	if !ok {
		return problem(c, learning.ErrDenied)
	}
	limit := 500
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > int(^uint(0)>>1)-501 {
			return problem(c, learning.ValidationError{Detail: "workspace limit must be a positive integer"})
		}
		limit = parsed
	}
	result, err := h.service.Workspace(learning.WithWorkspaceLimit(c.Context(), limit), actor)
	if err != nil {
		return problem(c, err)
	}
	return c.JSON(result)
}
func (h *Controller) action(c fiber.Ctx) error {
	actor, ok := h.actors.ActorID(c)
	if !ok {
		return problem(c, learning.ErrDenied)
	}
	var command learning.Command
	if err := httpx.DecodeStrictJSON(c.Body(), &command); err != nil {
		return httpx.WriteProblem(c, err)
	}
	key := c.Get("Idempotency-Key")
	if len(key) > 200 {
		return problem(c, learning.ValidationError{Detail: "request key is too long"})
	}
	result, err := h.service.Action(idempotency.WithKey(c.Context(), key), actor, command)
	if err != nil {
		return problem(c, err)
	}
	return c.JSON(result)
}
func problem(c fiber.Ctx, err error) error {
	var validation learning.ValidationError
	switch {
	case errors.Is(err, learning.ErrDenied):
		return httpx.WriteProblem(c, httpx.NewProblem(403, "forbidden", "Access denied", "This action is not available for this account or learner"))
	case errors.Is(err, learning.ErrNotFound):
		return httpx.WriteProblem(c, httpx.NewProblem(404, "not_found", "Record not found", "This learning record is unavailable"))
	case errors.Is(err, learning.ErrConflict):
		return httpx.WriteProblem(c, httpx.NewProblem(409, "version_conflict", "Refresh and try again", "The record changed or its current status does not permit this action"))
	case errors.As(err, &validation):
		return httpx.WriteProblem(c, httpx.NewProblem(422, "invalid_input", "Check your input", validation.Detail))
	default:
		return err
	}
}
