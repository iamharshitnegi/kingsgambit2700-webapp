package controller

import (
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

func (h *Controller) deviceCookieName() string { return h.cfg.CookieName + "_device" }
func (h *Controller) setDeviceCookie(c fiber.Ctx, raw string) {
	c.Cookie(&fiber.Cookie{Name: h.deviceCookieName(), Value: raw, Path: "/", Secure: h.cfg.CookieSecure, HTTPOnly: true, SameSite: "Lax", Expires: time.Now().UTC().Add(400 * 24 * time.Hour), MaxAge: 400 * 24 * 60 * 60})
}
func (h *Controller) clearDeviceCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{Name: h.deviceCookieName(), Value: "", Path: "/", Secure: h.cfg.CookieSecure, HTTPOnly: true, SameSite: "Lax", Expires: time.Unix(1, 0).UTC(), MaxAge: -1})
}
func (h *Controller) renewDeviceCookie(c fiber.Ctx) {
	raw := c.Cookies(h.deviceCookieName())
	if _, err := h.service.DeviceCSRF(c.Context(), raw); err == nil {
		h.setDeviceCookie(c, raw)
	}
}

func (h *Controller) installSession(c fiber.Ctx, result *authservice.SessionResult) error {
	h.setSessionCookie(c, result.Token, result.Session.AbsoluteExpiresAt)
	if result.User.RequiresMFA() && result.Session.MFAVerifiedAt == nil {
		// A new primary sign-in must never fall back to an earlier user's device.
		h.clearDeviceCookie(c)
	} else {
		raw, csrf, err := h.service.RememberSession(c.Context(), result.Token)
		if err != nil {
			return err
		}
		result.CSRFToken = csrf
		if raw != "" {
			h.setDeviceCookie(c, raw)
		} else {
			h.renewDeviceCookie(c)
		}
	}
	return nil
}

func (h *Controller) requireDeviceOrigin(c fiber.Ctx) error {
	origin := c.Get(fiber.HeaderOrigin)
	allowed := append([]string{}, h.cfg.AllowedOrigins...)
	if parsed, err := url.Parse(h.cfg.PublicWebURL); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		allowed = append(allowed, parsed.Scheme+"://"+parsed.Host)
	}
	for _, candidate := range allowed {
		if origin != "" && origin == strings.TrimRight(candidate, "/") {
			return c.Next()
		}
	}
	return httpx.WriteProblem(c, httpx.NewProblem(403, "origin_forbidden", "Origin not allowed", "Open the academy website to continue"))
}

func (h *Controller) restoreSession(c fiber.Ctx) error {
	c.Set(fiber.HeaderCacheControl, "no-store")
	raw := c.Cookies(h.deviceCookieName())
	result, err := h.service.RestoreSession(c.Context(), raw, c.Get("X-CSRF-Token"), c.Get(fiber.HeaderUserAgent), c.IP())
	if err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	h.setSessionCookie(c, result.Token, result.Session.AbsoluteExpiresAt)
	h.setDeviceCookie(c, raw)
	if result.User.RequiresMFA() && result.Session.MFAVerifiedAt == nil {
		return c.JSON(fiber.Map{"authenticated": false, "mfaRequired": true, "csrfToken": result.CSRFToken})
	}
	return c.JSON(sessionResponse(result.User, result.CSRFToken))
}

func (h *Controller) requireLogoutCSRF(c fiber.Ctx) error {
	csrf := c.Get("X-CSRF-Token")
	err := h.service.ValidateCSRF(c.Context(), c.Cookies(h.cfg.CookieName), csrf)
	if err == nil {
		return c.Next()
	}
	if !errors.Is(err, authservice.ErrInvalidCSRF) && !errors.Is(err, authservice.ErrUnauthenticated) {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	if err := h.service.ValidateDeviceCSRF(c.Context(), c.Cookies(h.deviceCookieName()), csrf); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.Next()
}

func sessionProblem(err error) httpx.Problem {
	switch {
	case errors.Is(err, authservice.ErrInvalidCSRF):
		return httpx.NewProblem(403, "invalid_csrf", "Invalid CSRF token", "Refresh the session and retry")
	case errors.Is(err, authservice.ErrUnauthenticated), errors.Is(err, authservice.ErrMFARequired):
		return mfaProblem(err)
	default:
		return httpx.NewProblem(503, "session_unavailable", "Session service unavailable", "Your sign-in could not be verified. Please try again.")
	}
}
