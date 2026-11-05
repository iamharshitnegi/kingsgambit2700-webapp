package controller

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

const principalKey = "auth_principal"

type Config struct {
	CookieName     string
	CookieSecure   bool
	CookieTTL      time.Duration
	AllowedOrigins []string
	PublicWebURL   string
}

type Controller struct {
	service *authservice.Service
	cfg     Config
}

func New(service *authservice.Service, cfg Config) *Controller {
	return &Controller{service: service, cfg: cfg}
}

func (h *Controller) Register(router fiber.Router) {
	router.Post("/auth/password/login", h.loginPassword)
	router.Post("/auth/password/register", h.registerPassword)
	router.Get("/auth/password/verify", h.verifyPassword)
	router.Get("/auth/google/config", h.googleConfiguration)
	router.Post("/auth/google", h.loginGoogle)
	auth := router.Group("/auth")
	auth.Post("/magic-links", h.requestMagicLink)
	auth.Get("/magic-links/consume", h.consumeMagicLink)
	auth.Post("/student-codes/exchange", h.exchangeStudentCode)
	auth.Get("/session", h.session)
	auth.Post("/session/restore", h.requireDeviceOrigin, h.restoreSession)
	auth.Get("/mfa/status", h.RequirePrimaryAuth, h.mfaStatus)
	auth.Post("/mfa/totp/enrollment", h.RequirePrimaryAuth, h.RequireCSRF, h.startTOTPEnrollment)
	auth.Post("/mfa/totp/enrollment/confirm", h.RequirePrimaryAuth, h.RequireCSRF, h.confirmTOTPEnrollment)
	auth.Post("/mfa/challenge", h.RequirePrimaryAuth, h.RequireCSRF, h.completeMFAChallenge)
	auth.Post("/mfa/users/:userId/reset", h.RequireAuth, h.RequireCSRF, h.RequireRecentMFA, h.resetMFA)
	auth.Post("/session/refresh", h.RequireAuth, h.RequireCSRF, h.refreshSession)
	auth.Get("/sessions", h.RequireAuth, h.listSessions)
	auth.Delete("/sessions/:sessionId", h.RequireAuth, h.RequireCSRF, h.revokeSession)
	auth.Post("/sessions/revoke-others", h.RequireAuth, h.RequireCSRF, h.revokeOtherSessions)
	auth.Delete("/sessions", h.RequireAuth, h.RequireCSRF, h.revokeAllSessions)
	auth.Post("/logout", h.requireDeviceOrigin, h.requireLogoutCSRF, h.logout)
	router.Get("/me", h.RequireAuth, h.me)
}

func (h *Controller) OptionalAuth(c fiber.Ctx) error {
	raw := c.Cookies(h.cfg.CookieName)
	if raw == "" {
		return c.Next()
	}
	principal, err := h.service.Authenticate(c.Context(), raw)
	if err == nil {
		c.Locals(principalKey, principal)
	}
	return c.Next()
}

func (h *Controller) RequireAuth(c fiber.Ctx) error {
	principal, err := h.service.Authenticate(c.Context(), c.Cookies(h.cfg.CookieName))
	if err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	c.Locals(principalKey, principal)
	return c.Next()
}

func (h *Controller) RequirePrimaryAuth(c fiber.Ctx) error {
	principal, err := h.service.AuthenticatePrimary(c.Context(), c.Cookies(h.cfg.CookieName))
	if err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	c.Locals(principalKey, principal)
	return c.Next()
}

// RequireRecentMFA is an enforcement hook for sensitive routes such as
// permission changes, recovery, impersonation, and staff MFA reset.
func (h *Controller) RequireRecentMFA(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnauthorized, "unauthenticated", "Authentication required", "Sign in to continue"))
	}
	if err := h.service.RequireRecentMFA(principal); err != nil {
		return httpx.WriteProblem(c, mfaProblem(err))
	}
	return c.Next()
}

func (h *Controller) RequireCSRF(c fiber.Ctx) error {
	if err := h.service.ValidateCSRF(
		c.Context(),
		c.Cookies(h.cfg.CookieName),
		c.Get("X-CSRF-Token"),
	); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.Next()
}

func Principal(c fiber.Ctx) (model.Principal, bool) {
	principal, ok := c.Locals(principalKey).(model.Principal)
	return principal, ok
}

func (h *Controller) requestMagicLink(c fiber.Ctx) error {
	var request struct {
		Email        string `json:"email"`
		RedirectPath string `json:"redirectPath"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	err := h.service.RequestMagicLink(c.Context(), request.Email, request.RedirectPath)
	if err != nil && !errors.Is(err, authservice.ErrInvalidInput) {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusServiceUnavailable, "email_unavailable", "Email unavailable", "Please try again shortly"))
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"status":  "accepted",
		"message": "If the address is eligible, a sign-in link will be sent",
	})
}

func (h *Controller) consumeMagicLink(c fiber.Ctx) error {
	result, redirect, err := h.service.ConsumeMagicLink(
		c.Context(),
		c.Query("token"),
		c.Get(fiber.HeaderUserAgent),
		c.IP(),
	)
	if err != nil {
		return c.Redirect().Status(fiber.StatusSeeOther).To(strings.TrimRight(h.cfg.PublicWebURL, "/") + "/sign-in?error=invalid_or_expired_link")
	}
	if err := h.installSession(c, &result); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(redirect)
}

func (h *Controller) me(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnauthorized, "unauthenticated", "Authentication required", "Sign in to continue"))
	}
	response := sessionResponse(principal.User, "")
	delete(response, "csrfToken")
	return c.JSON(response)
}

func (h *Controller) exchangeStudentCode(c fiber.Ctx) error {
	var request struct {
		Code string `json:"code"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	result, err := h.service.ExchangeStudentCode(c.Context(), strings.TrimSpace(request.Code), c.Get(fiber.HeaderUserAgent), c.IP())
	if err != nil {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnauthorized, "invalid_student_code", "Invalid student code", "Ask your guardian for a new code"))
	}
	if err := h.installSession(c, &result); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.JSON(sessionResponse(result.User, result.CSRFToken))
}

func (h *Controller) session(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	raw := c.Cookies(h.cfg.CookieName)
	principal, err := h.service.Authenticate(c.Context(), raw)
	if errors.Is(err, authservice.ErrMFARequired) {
		csrf, csrfErr := h.service.IssueCSRF(c.Context(), raw)
		if csrfErr != nil {
			return httpx.WriteProblem(c, sessionProblem(csrfErr))
		}
		return c.JSON(fiber.Map{"authenticated": false, "mfaRequired": true, "csrfToken": csrf})
	}
	if err != nil {
		if !errors.Is(err, authservice.ErrUnauthenticated) {
			return httpx.WriteProblem(c, sessionProblem(err))
		}
		csrf, deviceErr := h.service.DeviceCSRF(c.Context(), c.Cookies(h.deviceCookieName()))
		if deviceErr == nil {
			return c.JSON(fiber.Map{"authenticated": false, "restoreAvailable": true, "csrfToken": csrf})
		}
		if !errors.Is(deviceErr, authservice.ErrUnauthenticated) {
			return httpx.WriteProblem(c, sessionProblem(deviceErr))
		}
		return c.JSON(fiber.Map{"authenticated": false})
	}
	remembered, csrf, err := h.service.RememberSession(c.Context(), raw)
	if err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	if remembered != "" {
		h.setDeviceCookie(c, remembered)
	}
	// Normal reads must not overwrite a newer sign-in's cookie when an older
	// bootstrap response arrives late. Restore and sign-in renew the credential;
	// only the one-time migration of a legacy session sets it here.
	return c.JSON(sessionResponse(principal.User, csrf))
}

func (h *Controller) logout(c fiber.Ctx) error {
	if err := h.service.LogoutDevice(c.Context(), c.Cookies(h.deviceCookieName())); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	if err := h.service.Logout(c.Context(), c.Cookies(h.cfg.CookieName)); err != nil {
		return httpx.WriteProblem(c, err)
	}
	h.clearSessionCookie(c)
	h.clearDeviceCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Controller) listSessions(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(
			fiber.StatusUnauthorized,
			"unauthenticated",
			"Authentication required",
			"Sign in to continue",
		))
	}
	sessions, err := h.service.ListSessions(c.Context(), principal)
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.JSON(fiber.Map{"items": sessions})
}

func (h *Controller) revokeSession(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(
			fiber.StatusUnauthorized,
			"unauthenticated",
			"Authentication required",
			"Sign in to continue",
		))
	}
	revokedCurrent, err := h.service.RevokeSession(c.Context(), principal, c.Params("sessionId"))
	if errors.Is(err, authservice.ErrInvalidInput) {
		return httpx.WriteProblem(c, httpx.NewProblem(
			fiber.StatusBadRequest,
			"invalid_session_id",
			"Invalid session identifier",
			"The session identifier must be a UUIDv7",
		))
	}
	if err != nil {
		return httpx.WriteProblem(c, err)
	}
	if revokedCurrent {
		h.clearSessionCookie(c)
		h.clearDeviceCookie(c)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Controller) revokeOtherSessions(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(
			fiber.StatusUnauthorized,
			"unauthenticated",
			"Authentication required",
			"Sign in to continue",
		))
	}
	if err := h.service.RevokeOtherSessions(c.Context(), principal); err != nil {
		return httpx.WriteProblem(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Controller) revokeAllSessions(c fiber.Ctx) error {
	principal, ok := Principal(c)
	if !ok {
		return httpx.WriteProblem(c, httpx.NewProblem(
			fiber.StatusUnauthorized,
			"unauthenticated",
			"Authentication required",
			"Sign in to continue",
		))
	}
	if err := h.service.RevokeAllSessions(c.Context(), principal); err != nil {
		return httpx.WriteProblem(c, err)
	}
	h.clearSessionCookie(c)
	h.clearDeviceCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

func (h *Controller) refreshSession(c fiber.Ctx) error {
	result, err := h.service.RotateSession(
		c.Context(),
		c.Cookies(h.cfg.CookieName),
		c.Get(fiber.HeaderUserAgent),
		c.IP(),
	)
	if err != nil {
		return httpx.WriteProblem(c, httpx.NewProblem(fiber.StatusUnauthorized, "unauthenticated", "Authentication required", "Sign in to continue"))
	}
	if err := h.installSession(c, &result); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.JSON(sessionResponse(result.User, result.CSRFToken))
}

func (h *Controller) setSessionCookie(c fiber.Ctx, value string, expires time.Time) {
	c.Cookie(&fiber.Cookie{
		Name:     h.cfg.CookieName,
		Value:    value,
		Path:     "/",
		Secure:   h.cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: "Lax",
		Expires:  expires,
	})
}

func (h *Controller) clearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     h.cfg.CookieName,
		Value:    "",
		Path:     "/",
		Secure:   h.cfg.CookieSecure,
		HTTPOnly: true,
		SameSite: "Lax",
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func sessionResponse(user model.User, csrf string) fiber.Map {
	roles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, string(role))
	}
	return fiber.Map{
		"authenticated": true,
		"csrfToken":     csrf,
		"user": fiber.Map{
			"id":          user.ID,
			"email":       user.Email,
			"displayName": user.DisplayName,
			"roles":       roles,
		},
	}
}

func decode(c fiber.Ctx, target any) error {
	return httpx.DecodeStrictJSON(c.Body(), target)
}

func ValidateRedirect(raw string) bool {
	parsed, err := url.Parse(raw)
	return err == nil && parsed.IsAbs()
}
