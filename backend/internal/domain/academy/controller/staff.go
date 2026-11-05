package controller

import (
	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/academy"
)

func (h *Controller) registerStaff(router fiber.Router) {
	router.Get("/academy/staff", func(c fiber.Ctx) error {
		actor, ok := h.actors.ActorID(c)
		if !ok {
			return unauthenticated(c)
		}
		out, err := h.service.Staff(c.Context(), actor)
		if err != nil {
			return problem(c, err)
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(out)
	})
	router.Patch("/academy/staff/:id", mutate(h, func(c fiber.Ctx, actor string, input academy.StaffUpdate) (any, error) {
		return h.service.UpdateStaff(c.Context(), actor, c.Params("id"), input)
	}))
}
