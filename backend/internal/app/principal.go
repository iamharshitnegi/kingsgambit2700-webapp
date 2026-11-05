package app

import (
	"github.com/gofiber/fiber/v3"
	authcontroller "github.com/kingsgambit2700/platform/backend/internal/auth/controller"
)

type principalActorResolver struct{}

func (principalActorResolver) ActorID(c fiber.Ctx) (string, bool) {
	principal, ok := authcontroller.Principal(c)
	if !ok || principal.User.ID == "" {
		return "", false
	}
	return principal.User.ID, true
}
