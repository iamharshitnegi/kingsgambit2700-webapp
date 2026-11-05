package controller

import (
	"errors"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/academy"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
)

type ActorResolver interface {
	ActorID(fiber.Ctx) (string, bool)
}
type Controller struct {
	service *academy.Service
	actors  ActorResolver
}

func New(service *academy.Service, actors ActorResolver) (*Controller, error) {
	if service == nil || actors == nil {
		return nil, errors.New("academy controller: dependencies are required")
	}
	return &Controller{service: service, actors: actors}, nil
}

// Register requires the caller to mount current-session, staff MFA and CSRF
// middleware on /academy. Permission checks remain inside the domain layer.
func (h *Controller) Register(router fiber.Router) {
	h.registerStaff(router)
	router.Get("/academy/workspace", h.workspace)
	router.Post("/academy/make-ups/:id/fulfill", mutate(h, func(c fiber.Ctx, a string, in academy.MakeUpFulfillment) (any, error) {
		return h.service.FulfillMakeUp(c.Context(), a, c.Params("id"), in)
	}))
	router.Patch("/academy/profile", mutate(h, func(c fiber.Ctx, a string, in academy.ProfileInput) (any, error) {
		return h.service.UpdateProfile(c.Context(), a, in)
	}))
	router.Put("/academy/preferences", mutate(h, func(c fiber.Ctx, a string, in academy.Preferences) (any, error) {
		return h.service.SavePreferences(c.Context(), a, in)
	}))
	router.Get("/academy/operations", func(c fiber.Ctx) error {
		actor, ok := h.actors.ActorID(c)
		if !ok {
			return unauthenticated(c)
		}
		limit := 500
		if raw := c.Query("limit"); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < 1 || parsed > int(^uint(0)>>1)-501 {
				return problem(c, academy.ErrInvalid)
			}
			limit = parsed
		}
		out, err := h.service.Operations(academy.WithWorkspaceLimit(c.Context(), limit), actor)
		if err != nil {
			return problem(c, err)
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(out)
	})
	router.Get("/academy/availability", func(c fiber.Ctx) error {
		actor, ok := h.actors.ActorID(c)
		if !ok {
			return unauthenticated(c)
		}
		out, err := h.service.Availability(c.Context(), actor)
		if err != nil {
			return problem(c, err)
		}
		return c.JSON(out)
	})
	router.Post("/academy/availability", mutate(h, func(c fiber.Ctx, a string, in academy.Availability) (any, error) {
		return h.service.SaveAvailability(c.Context(), a, in)
	}))
	router.Delete("/academy/availability/:id", func(c fiber.Ctx) error {
		actor, ok := h.actors.ActorID(c)
		if !ok {
			return unauthenticated(c)
		}
		if err := h.service.DeleteAvailability(c.Context(), actor, c.Params("id")); err != nil {
			return problem(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	})
	router.Patch("/academy/students/:id", mutate(h, func(c fiber.Ctx, a string, in academy.StudentUpdate) (any, error) {
		return h.service.UpdateStudent(c.Context(), a, c.Params("id"), in)
	}))
	router.Patch("/academy/batches/:id", mutate(h, func(c fiber.Ctx, a string, in academy.BatchInput) (any, error) {
		return h.service.UpdateBatch(c.Context(), a, c.Params("id"), in)
	}))
	router.Patch("/academy/enrolments/:id", mutate(h, func(c fiber.Ctx, a string, in academy.EnrolmentUpdate) (any, error) {
		return h.service.UpdateEnrolment(c.Context(), a, c.Params("id"), in)
	}))
	router.Patch("/academy/leads/:id", mutate(h, func(c fiber.Ctx, a string, in academy.LeadUpdate) (any, error) {
		return h.service.UpdateLead(c.Context(), a, c.Params("id"), in)
	}))
	router.Post("/academy/leads/:id/convert", mutate(h, func(c fiber.Ctx, a string, in academy.ConversionInput) (any, error) {
		return h.service.ConvertLead(c.Context(), a, c.Params("id"), in)
	}))
	router.Post("/academy/trial-slots", mutate(h, func(c fiber.Ctx, a string, in academy.TrialSlot) (any, error) {
		return h.service.CreateTrialSlot(c.Context(), a, in)
	}))
	router.Patch("/academy/trial-bookings/:id", mutate(h, func(c fiber.Ctx, a string, in academy.TrialUpdate) (any, error) {
		return h.service.UpdateTrial(c.Context(), a, c.Params("id"), in)
	}))
	router.Post("/academy/trial-bookings/:id/reschedule", mutate(h, func(c fiber.Ctx, a string, in academy.TrialReschedule) (any, error) {
		return h.service.RescheduleTrial(c.Context(), a, c.Params("id"), in)
	}))
	router.Post("/academy/students", mutate(h, func(c fiber.Ctx, a string, in academy.StudentInput) (any, error) {
		return h.service.CreateStudent(c.Context(), a, in)
	}))
	router.Post("/academy/staff", mutate(h, func(c fiber.Ctx, a string, in academy.StaffInput) (any, error) {
		return h.service.CreateStaff(c.Context(), a, in)
	}))
	router.Post("/academy/batches", mutate(h, func(c fiber.Ctx, a string, in academy.BatchInput) (any, error) {
		return h.service.CreateBatch(c.Context(), a, in)
	}))
	router.Post("/academy/enrolments", mutate(h, func(c fiber.Ctx, a string, in academy.EnrolmentInput) (any, error) {
		return h.service.CreateEnrolment(c.Context(), a, in)
	}))
	router.Post("/academy/policies", mutate(h, func(c fiber.Ctx, a string, in academy.PolicyInput) (any, error) {
		return h.service.PublishPolicy(c.Context(), a, in)
	}))
	router.Put("/academy/settings", mutate(h, func(c fiber.Ctx, a string, in academy.Settings) (any, error) {
		return h.service.SaveSettings(c.Context(), a, in)
	}))
	router.Post("/academy/students/:studentId/consent", mutate(h, func(c fiber.Ctx, a string, in academy.ConsentInput) (any, error) {
		err := h.service.AcceptConsent(c.Context(), a, c.Params("studentId"), in)
		return fiber.Map{"accepted": err == nil}, err
	}))
	router.Post("/academy/students/:studentId/access-code", h.code)
	router.Post("/academy/classes/preview", mutate(h, func(c fiber.Ctx, a string, in academy.ClassInput) (any, error) {
		return h.service.Schedule(c.Context(), a, in, true)
	}))
	router.Post("/academy/classes", mutate(h, func(c fiber.Ctx, a string, in academy.ClassInput) (any, error) {
		return h.service.Schedule(c.Context(), a, in, false)
	}))
	router.Patch("/academy/classes/:classId", mutate(h, func(c fiber.Ctx, a string, in academy.ClassInput) (any, error) {
		return h.service.UpdateClass(c.Context(), a, c.Params("classId"), in)
	}))
	router.Post("/academy/classes/:classId/complete", mutate(h, func(c fiber.Ctx, a string, in academy.CompletionInput) (any, error) {
		return h.service.Complete(c.Context(), a, c.Params("classId"), in)
	}))
	router.Post("/academy/classes/:classId/cancel-preview", mutate(h, func(c fiber.Ctx, a string, in academy.CancellationInput) (any, error) {
		return h.service.Cancel(c.Context(), a, c.Params("classId"), in, true)
	}))
	router.Post("/academy/classes/:classId/cancel", mutate(h, func(c fiber.Ctx, a string, in academy.CancellationInput) (any, error) {
		return h.service.Cancel(c.Context(), a, c.Params("classId"), in, false)
	}))
}
func (h *Controller) workspace(c fiber.Ctx) error {
	actor, ok := h.actors.ActorID(c)
	if !ok {
		return unauthenticated(c)
	}
	limit := 500
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > int(^uint(0)>>1)-501 {
			return problem(c, academy.ErrInvalid)
		}
		limit = parsed
	}
	out, err := h.service.Workspace(academy.WithWorkspaceLimit(c.Context(), limit), actor)
	if err != nil {
		return problem(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func (h *Controller) code(c fiber.Ctx) error {
	actor, ok := h.actors.ActorID(c)
	if !ok {
		return unauthenticated(c)
	}
	out, err := h.service.IssueCode(c.Context(), actor, c.Params("studentId"))
	if err != nil {
		return problem(c, err)
	}
	c.Set("Cache-Control", "no-store")
	return c.JSON(out)
}
func mutate[T any](h *Controller, fn func(fiber.Ctx, string, T) (any, error)) fiber.Handler {
	return func(c fiber.Ctx) error {
		actor, ok := h.actors.ActorID(c)
		if !ok {
			return unauthenticated(c)
		}
		var in T
		if err := httpx.DecodeStrictJSON(c.Body(), &in); err != nil {
			return err
		}
		key := c.Get("Idempotency-Key")
		if len(key) > 200 {
			return problem(c, academy.ErrInvalid)
		}
		c.SetContext(idempotency.WithKey(c.Context(), key))
		out, err := fn(c, actor, in)
		if err != nil {
			return problem(c, err)
		}
		c.Set("Cache-Control", "no-store")
		return c.JSON(out)
	}
}
func unauthenticated(c fiber.Ctx) error {
	return httpx.WriteProblem(c, httpx.NewProblem(401, "unauthenticated", "Sign in required", "Sign in to use your academy workspace"))
}
func problem(c fiber.Ctx, err error) error {
	status, code, title, detail := 500, "internal_error", "Unable to save", "The academy could not complete this request. Please try again."
	switch {
	case errors.Is(err, academy.ErrDenied):
		status, code, title, detail = 403, "permission_denied", "Action unavailable", "Your account does not have permission for this record."
	case errors.Is(err, academy.ErrNotFound):
		status, code, title, detail = 404, "not_found", "Record not found", "This academy record is no longer available."
	case errors.Is(err, academy.ErrCredits):
		status, code, title, detail = 409, "insufficient_credits", "Class credits needed", "Every student needs available credits for all occurrences. Ask finance to review the enrolment."
	case errors.Is(err, academy.ErrConsent):
		status, code, title, detail = 409, "consent_required", "Guardian consent needed", "Publish both policies and academy details, then have the guardian accept the current policy versions."
	case errors.Is(err, academy.ErrConflict):
		status, code, title, detail = 409, "conflict", "Schedule changed", err.Error()
	case errors.Is(err, academy.ErrInvalid):
		status, code, title, detail = 422, "invalid_input", "Check the details", err.Error()
	}
	return httpx.WriteProblem(c, httpx.NewProblem(status, code, title, detail))
}
