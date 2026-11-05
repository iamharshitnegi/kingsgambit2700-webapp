package ticket

import (
	"encoding/json"
	"errors"

	"github.com/gofiber/fiber/v3"
	authcontroller "github.com/kingsgambit2700/platform/backend/internal/auth/controller"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

type Controller struct{ service *Service }

func NewController(service *Service) *Controller { return &Controller{service: service} }

func (h *Controller) Register(router fiber.Router, requireAuth, requireCSRF fiber.Handler) {
	router.Post("/realtime/join-tickets", requireAuth, requireCSRF, h.issue)
}

func (h *Controller) issue(c fiber.Ctx) error {
	var request struct {
		ClassSessionID string `json:"classSessionId"`
	}
	if err := json.Unmarshal(c.Body(), &request); err != nil {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusBadRequest, "invalid_json", "Invalid JSON", "The request body is not valid JSON"))
	}
	principal, ok := authcontroller.Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnauthorized, "unauthenticated", "Authentication required", "Sign in to continue"))
	}
	result, err := h.service.Issue(c.Context(), principal, request.ClassSessionID)
	if errors.Is(err, ErrForbidden) {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusForbidden, "classroom_forbidden", "Classroom access denied", "You are not assigned to this class session"))
	}
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(result)
}
