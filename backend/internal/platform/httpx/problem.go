package httpx

import (
	"errors"
	"fmt"

	"github.com/gofiber/fiber/v3"
)

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Code      string       `json:"code"`
	RequestID string       `json:"requestId,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

func (p Problem) Error() string {
	return fmt.Sprintf("%s: %s", p.Code, p.Title)
}

func NewProblem(status int, code, title, detail string) Problem {
	return Problem{
		Type:   "https://api.kingsgambit2700.in/problems/" + code,
		Title:  title,
		Status: status,
		Detail: detail,
		Code:   code,
		Errors: []FieldError{},
	}
}

func WriteProblem(c fiber.Ctx, err error) error {
	var problem Problem
	if !errors.As(err, &problem) {
		problem = NewProblem(fiber.StatusInternalServerError, "internal_error", "Internal server error", "The request could not be completed")
	}
	problem.RequestID = RequestID(c)
	c.Set(fiber.HeaderContentType, "application/problem+json")
	return c.Status(problem.Status).JSON(problem)
}
