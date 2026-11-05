package controller

import (
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/publicsite/repository"
	publicservice "github.com/kingsgambit2700/platform/backend/internal/publicsite/service"
)

type Controller struct{ service *publicservice.Service }

func New(service *publicservice.Service) *Controller { return &Controller{service: service} }

func (h *Controller) Register(router fiber.Router) {
	public := router.Group("/public")
	public.Get("/organization", h.organization)
	public.Get("/policies", h.policies)
	public.Get("/policies/:slug", h.policy)
	public.Get("/policies/:slug/versions/:version", h.policyVersion)
	public.Get("/programs", h.programs)
	public.Get("/coaches", h.coaches)
	public.Get("/curriculum", h.curriculum)
	public.Get("/pricing", h.pricing)
}

func (h *Controller) policyVersion(c fiber.Ctx) error {
	value, err := h.service.PolicyVersion(c.Context(), c.Params("slug"), c.Params("version"))
	if errors.Is(err, repository.ErrNotFound) {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusNotFound, "policy_version_not_found", "Policy version not found", "No published policy has this slug and version"))
	}
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "public, max-age=31536000, immutable")
	return c.JSON(value)
}

func (h *Controller) organization(c fiber.Ctx) error {
	value, err := h.service.Organization(c.Context())
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(value)
}

func (h *Controller) policies(c fiber.Ctx) error {
	items, err := h.service.Policies(c.Context())
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *Controller) policy(c fiber.Ctx) error {
	value, err := h.service.Policy(c.Context(), c.Params("slug"))
	if errors.Is(err, repository.ErrNotFound) {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusNotFound, "policy_not_found", "Policy not found", "No active policy has this slug"))
	}
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(value)
}

func (h *Controller) programs(c fiber.Ctx) error {
	items, err := h.service.Programs(c.Context())
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *Controller) coaches(c fiber.Ctx) error {
	items, err := h.service.Coaches(c.Context())
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(fiber.Map{"items": items})
}

func (h *Controller) curriculum(c fiber.Ctx) error {
	value, err := h.service.Curriculum(c.Context())
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(value)
}

func (h *Controller) pricing(c fiber.Ctx) error {
	items, err := h.service.Prices(c.Context(), c.Query("currency", "INR"))
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(fiber.Map{"currency": c.Query("currency", "INR"), "items": items})
}
