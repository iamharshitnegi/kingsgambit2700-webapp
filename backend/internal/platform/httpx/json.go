package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gofiber/fiber/v3"
)

// DecodeStrictJSON decodes exactly one JSON value and rejects unknown fields.
// Keeping this at the HTTP boundary prevents wire typos from being silently
// interpreted as missing consent, authentication, or payment fields.
func DecodeStrictJSON(body []byte, target any) error {
	if len(body) == 0 {
		return NewProblem(
			fiber.StatusBadRequest,
			"invalid_json",
			"Invalid JSON",
			"A JSON request body is required",
		)
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return NewProblem(
			fiber.StatusBadRequest,
			"invalid_json",
			"Invalid JSON",
			"The request body is not valid JSON or contains an unknown field",
		)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return NewProblem(
			fiber.StatusBadRequest,
			"invalid_json",
			"Invalid JSON",
			"The request body must contain exactly one JSON value",
		)
	}
	return nil
}
