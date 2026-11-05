package controller

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/lead/model"
	leadservice "github.com/kingsgambit2700/platform/backend/internal/lead/service"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

type Controller struct{ service *leadservice.Service }

func New(service *leadservice.Service) *Controller { return &Controller{service: service} }

func (h *Controller) Register(router fiber.Router) {
	router.Post("/public/leads", h.create)
}

func (h *Controller) create(c fiber.Ctx) error {
	var input model.CreateInput
	if err := httpx.DecodeStrictJSON(c.Body(), &input); err != nil {
		return httpx.WriteProblem(c, err)
	}
	lead, created, err := h.service.Create(c.Context(), input)
	if errors.Is(err, leadservice.ErrInvalidInput) {
		problem := httpx.NewProblem(fiber.StatusUnprocessableEntity, "invalid_lead", "Lead details are invalid", "Check contact, consent, country and timezone fields")
		return httpx.WriteProblem(c, problem)
	}
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	status := fiber.StatusOK
	if created {
		status = fiber.StatusCreated
	}
	return c.Status(status).JSON(fiber.Map{
		"id":        lead.ID,
		"status":    lead.Status,
		"createdAt": lead.CreatedAt,
	})
}
