package httpx

import "github.com/gofiber/fiber/v3"

const requestIDKey = "request_id"

func RequestID(c fiber.Ctx) string {
	if value, ok := c.Locals(requestIDKey).(string); ok {
		return value
	}
	return ""
}

func SetRequestID(c fiber.Ctx, value string) {
	c.Locals(requestIDKey, value)
}
