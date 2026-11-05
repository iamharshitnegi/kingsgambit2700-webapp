package controller

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v3"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

func (h *Controller) mfaStatus(c fiber.Ctx) error {
	status, err := h.service.MFAStatus(c.Context(), c.Cookies(h.cfg.CookieName))
	if err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	return c.JSON(status)
}

func (h *Controller) startTOTPEnrollment(c fiber.Ctx) error {
	enrollment, err := h.service.StartTOTPEnrollment(c.Context(), c.Cookies(h.cfg.CookieName))
	if err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	return c.Status(fiber.StatusCreated).JSON(enrollment)
}

func (h *Controller) confirmTOTPEnrollment(c fiber.Ctx) error {
	var request struct {
		Code string `json:"code"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	result, err := h.service.ConfirmTOTPEnrollment(
		c.Context(),
		c.Cookies(h.cfg.CookieName),
		strings.TrimSpace(request.Code),
		c.Get(fiber.HeaderUserAgent),
		c.IP(),
		httpx.RequestID(c),
	)
	if err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	if err := h.installSession(c, &result.SessionResult); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	response := sessionResponse(result.User, result.CSRFToken)
	response["recoveryCodes"] = result.RecoveryCodes
	return c.JSON(response)
}

func (h *Controller) completeMFAChallenge(c fiber.Ctx) error {
	var request struct {
		Code         string `json:"code"`
		RecoveryCode string `json:"recoveryCode"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	result, err := h.service.CompleteMFAChallenge(
		c.Context(),
		c.Cookies(h.cfg.CookieName),
		request.Code,
		request.RecoveryCode,
		c.Get(fiber.HeaderUserAgent),
		c.IP(),
		httpx.RequestID(c),
	)
	if err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	if err := h.installSession(c, &result); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.JSON(sessionResponse(result.User, result.CSRFToken))
}

func (h *Controller) resetMFA(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, mfaProblem(authservice.ErrUnauthenticated))
	}
	var request struct {
		Reason string `json:"reason"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	if err := h.service.ResetMFA(
		c.Context(),
		principal,
		c.Params("userId"),
		request.Reason,
		httpx.RequestID(c),
	); err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func mfaProblem(err error) httpx.Problem {
	switch {
	case errors.Is(err, authservice.ErrUnauthenticated):
		return httpx.NewProblem(fiber.StatusUnauthorized, "unauthenticated", "Authentication required", "Sign in to continue")
	case errors.Is(err, authservice.ErrMFARequired):
		return httpx.NewProblem(fiber.StatusUnauthorized, "mfa_required", "Multi-factor authentication required", "Complete the MFA challenge to continue")
	case errors.Is(err, authservice.ErrMFAChallengeExpired):
		return httpx.NewProblem(fiber.StatusUnauthorized, "mfa_challenge_expired", "MFA challenge expired", "Request a new magic link to continue")
	case errors.Is(err, authservice.ErrMFAEnrollmentRequired):
		return httpx.NewProblem(fiber.StatusConflict, "mfa_enrollment_required", "MFA enrollment required", "Enroll an authenticator before continuing")
	case errors.Is(err, authservice.ErrMFAAlreadyEnrolled):
		return httpx.NewProblem(fiber.StatusConflict, "mfa_already_enrolled", "MFA already enrolled", "Use the existing authenticator or request an audited reset")
	case errors.Is(err, authservice.ErrInvalidMFA):
		return httpx.NewProblem(fiber.StatusUnauthorized, "invalid_mfa_credential", "Invalid MFA credential", "Check the code and try again")
	case errors.Is(err, authservice.ErrRecentMFARequired):
		return httpx.NewProblem(fiber.StatusForbidden, "recent_mfa_required", "Recent MFA required", "Complete a fresh MFA challenge before this operation")
	case errors.Is(err, authservice.ErrMFAResetForbidden):
		return httpx.NewProblem(fiber.StatusForbidden, "mfa_reset_forbidden", "MFA reset forbidden", "You are not allowed to reset this MFA method")
	case errors.Is(err, authservice.ErrInvalidInput):
		return httpx.NewProblem(fiber.StatusBadRequest, "invalid_mfa_request", "Invalid MFA request", "Check the request fields and try again")
	case errors.Is(err, authservice.ErrMFAUnavailable):
		return httpx.NewProblem(fiber.StatusServiceUnavailable, "mfa_unavailable", "MFA unavailable", "MFA is not configured on this service")
	default:
		return httpx.NewProblem(fiber.StatusInternalServerError, "mfa_operation_failed", "MFA operation failed", "The operation could not be completed")
	}
}
