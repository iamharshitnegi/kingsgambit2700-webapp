package controller

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/trial/model"
	"github.com/kingsgambit2700/platform/backend/internal/trial/repository"
	trialservice "github.com/kingsgambit2700/platform/backend/internal/trial/service"
)

type Controller struct{ service *trialservice.Service }

func New(service *trialservice.Service) *Controller { return &Controller{service: service} }

func (h *Controller) Register(router fiber.Router) {
	router.Get("/public/trial-slots", h.listSlots)
	router.Post("/public/trial-bookings", h.book)
}

func (h *Controller) listSlots(c fiber.Ctx) error {
	filter := model.ListFilter{
		Format:   c.Query("format"),
		AgeBand:  c.Query("ageBand"),
		TimeZone: c.Query("timezone", "UTC"),
	}
	var err error
	if raw := c.Query("from"); raw != "" {
		filter.From, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return invalid(c)
		}
	}
	if raw := c.Query("to"); raw != "" {
		filter.To, err = time.Parse(time.RFC3339, raw)
		if err != nil {
			return invalid(c)
		}
	}
	items, err := h.service.List(c.Context(), filter)
	if errors.Is(err, trialservice.ErrInvalidInput) {
		return invalid(c)
	}
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *Controller) book(c fiber.Ctx) error {
	var input model.BookingInput
	if err := httpx.DecodeStrictJSON(c.Body(), &input); err != nil {
		return httpx.WriteProblem(c, err)
	}
	input.IdempotencyKey = c.Get("Idempotency-Key")
	booking, err := h.service.Book(c.Context(), input)
	switch {
	case errors.Is(err, trialservice.ErrInvalidInput):
		return invalid(c)
	case errors.Is(err, repository.ErrNotFound):
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusNotFound, "trial_slot_not_found", "Trial slot not found", "Choose another available slot"))
	case errors.Is(err, repository.ErrFull):
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusConflict, "trial_slot_full", "Trial slot is full", "Choose another available slot"))
	case errors.Is(err, repository.ErrConflict):
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusConflict, "idempotency_conflict", "Booking request changed", "Use a new idempotency key when changing the booking details"))
	case err != nil:
		return httpx.WriteProblem(c, err)
	default:
		return c.Status(fiber.StatusCreated).JSON(booking)
	}
}

func invalid(c fiber.Ctx) error {
	return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnprocessableEntity, "invalid_trial_request", "Trial request is invalid", "Check the slot, contact, timezone and idempotency key"))
}
