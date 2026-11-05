package controller

import (
	"bytes"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/health"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/platform/metrics"
)

type Operational struct {
	readiness *health.Checker
	metrics   *metrics.Registry
}

func NewOperational(readiness *health.Checker, registry *metrics.Registry) *Operational {
	return &Operational{readiness: readiness, metrics: registry}
}

func (h *Operational) Register(router fiber.Router) {
	router.Get("/health/live", h.live)
	router.Get("/health/ready", h.ready)
	router.Get("/metrics", h.prometheus)
}

func (h *Operational) live(c fiber.Ctx) error {
	return c.JSON(health.Result{Status: "ok"})
}

func (h *Operational) ready(c fiber.Ctx) error {
	result, err := h.readiness.Ready(c.Context())
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(result)
	}
	return c.JSON(result)
}

func (h *Operational) prometheus(c fiber.Ctx) error {
	var body bytes.Buffer
	if err := h.metrics.WritePrometheus(&body); err != nil {
		return httpx.WriteProblem(c, err)
	}
	c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4; charset=utf-8")
	return c.Send(body.Bytes())
}
