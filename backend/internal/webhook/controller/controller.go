package controller

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
	"github.com/kingsgambit2700/platform/backend/internal/webhook/model"
	webhookservice "github.com/kingsgambit2700/platform/backend/internal/webhook/service"
)

const defaultMaxBodyBytes = 256 << 10

type Receiver interface {
	Ingest(context.Context, model.Provider, []byte, model.Headers) (model.IngestResult, error)
}

type Controller struct {
	receiver     Receiver
	maxBodyBytes int
}

func New(receiver Receiver, maxBodyBytes int) (*Controller, error) {
	if receiver == nil {
		return nil, errors.New("webhook controller: receiver is required")
	}
	if maxBodyBytes == 0 {
		maxBodyBytes = defaultMaxBodyBytes
	}
	if maxBodyBytes < 0 {
		return nil, errors.New("webhook controller: body limit must not be negative")
	}
	return &Controller{receiver: receiver, maxBodyBytes: maxBodyBytes}, nil
}

func (h *Controller) Register(router fiber.Router) {
	router.Post("/webhooks/razorpay", h.receive(model.ProviderRazorpay))
	router.Post("/webhooks/resend", h.receive(model.ProviderResend))
	router.Post("/webhooks/daily", h.receive(model.ProviderDaily))
}

func (h *Controller) receive(provider model.Provider) fiber.Handler {
	return func(c fiber.Ctx) error {
		body := c.Body()
		if len(body) > h.maxBodyBytes {
			return writeProblem(c, httpx.NewProblem(
				fiber.StatusRequestEntityTooLarge,
				"webhook_body_too_large",
				"Webhook body too large",
				"The webhook request exceeds the accepted size",
			))
		}
		_, err := h.receiver.Ingest(c.Context(), provider, body, headers(c, provider))
		if err != nil {
			return writeError(c, err)
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func headers(c fiber.Ctx, provider model.Provider) model.Headers {
	switch provider {
	case model.ProviderRazorpay:
		return model.Headers{
			EventID:   c.Get("X-Razorpay-Event-Id"),
			Signature: c.Get("X-Razorpay-Signature"),
		}
	case model.ProviderResend:
		return model.Headers{
			EventID:   c.Get("svix-id"),
			Signature: c.Get("svix-signature"),
			Timestamp: c.Get("svix-timestamp"),
		}
	case model.ProviderDaily:
		return model.Headers{
			Signature: c.Get("X-Webhook-Signature"),
			Timestamp: c.Get("X-Webhook-Timestamp"),
		}
	default:
		return model.Headers{}
	}
}

func writeError(c fiber.Ctx, err error) error {
	var problem httpx.Problem
	switch {
	case errors.Is(err, webhookservice.ErrInvalidSignature):
		problem = httpx.NewProblem(
			fiber.StatusUnauthorized,
			"webhook_signature_invalid",
			"Webhook signature invalid",
			"The webhook could not be authenticated",
		)
	case errors.Is(err, webhookservice.ErrInvalidPayload):
		problem = httpx.NewProblem(
			fiber.StatusBadRequest,
			"webhook_payload_invalid",
			"Webhook payload invalid",
			"The verified webhook does not contain a valid event envelope",
		)
	case errors.Is(err, webhookservice.ErrEventConflict):
		problem = httpx.NewProblem(
			fiber.StatusConflict,
			"webhook_event_conflict",
			"Webhook event conflict",
			"This provider event ID was already received with a different payload",
		)
	default:
		problem = httpx.NewProblem(
			fiber.StatusServiceUnavailable,
			"webhook_unavailable",
			"Webhook receipt unavailable",
			"The verified webhook could not be stored; retry the delivery",
		)
	}
	return writeProblem(c, problem)
}

func writeProblem(c fiber.Ctx, problem httpx.Problem) error {
	problem.RequestID = httpx.RequestID(c)
	err := c.Status(problem.Status).JSON(problem)
	c.Set(fiber.HeaderContentType, "application/problem+json")
	return err
}
