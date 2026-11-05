package controller

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/platform/httpx"
)

func (h *Controller) loginPassword(c fiber.Ctx) error {
	var request struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	result, err := h.service.LoginPassword(c.Context(), request.Login, request.Password, c.Get(fiber.HeaderUserAgent), c.IP())
	return h.completePrimaryLogin(c, result, err)
}

func (h *Controller) registerPassword(c fiber.Ctx) error {
	var request authservice.PasswordSignup
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	if err := h.service.RegisterPassword(c.Context(), request); err != nil {
		if errors.Is(err, authservice.ErrInvalidInput) {
			return httpx.WriteProblem(c, httpx.NewProblem(400, "invalid_registration", "Check your details", "Use a valid email, a 3–32 character username (letters, numbers, dots, hyphens or underscores), and a 12–128 character password."))
		}
		return httpx.WriteProblem(c, httpx.NewProblem(503, "registration_unavailable", "Registration unavailable", "We could not send your verification email. Please try again."))
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"status": "accepted", "message": "Check your email to verify your account."})
}

func (h *Controller) verifyPassword(c fiber.Ctx) error {
	result, redirect, err := h.service.VerifyPasswordRegistration(c.Context(), c.Query("token"), c.Get(fiber.HeaderUserAgent), c.IP())
	if err != nil {
		reason := "invalid_or_expired_link"
		if errors.Is(err, repository.ErrConflict) {
			reason = "registration_conflict"
		}
		return c.Redirect().Status(303).To(strings.TrimRight(h.cfg.PublicWebURL, "/") + "/sign-in?error=" + reason)
	}
	if err := h.installSession(c, &result); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	return c.Redirect().Status(303).To(redirect)
}

func (h *Controller) googleConfiguration(c fiber.Ctx) error {
	configuration, err := h.service.GoogleConfiguration(c.Context())
	if err != nil {
		return credentialProblem(c, err)
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	if configuration.Enabled {
		c.Cookie(&fiber.Cookie{Name: h.cfg.CookieName + "_google", Value: configuration.Nonce,
			Path: "/", HTTPOnly: true, Secure: h.cfg.CookieSecure, SameSite: "Lax", Expires: time.Now().Add(10 * time.Minute)})
	}
	return c.JSON(configuration)
}

func (h *Controller) loginGoogle(c fiber.Ctx) error {
	var request struct {
		Credential string `json:"credential"`
	}
	if err := decode(c, &request); err != nil {
		return httpx.WriteProblem(c, err)
	}
	result, err := h.service.LoginGoogle(c.Context(), request.Credential, c.Cookies(h.cfg.CookieName+"_google"), c.Get(fiber.HeaderUserAgent), c.IP())
	if err == nil {
		c.Cookie(&fiber.Cookie{Name: h.cfg.CookieName + "_google", Value: "", Path: "/", HTTPOnly: true, Secure: h.cfg.CookieSecure, SameSite: "Lax", Expires: time.Unix(1, 0)})
	}
	return h.completePrimaryLogin(c, result, err)
}

func (h *Controller) completePrimaryLogin(c fiber.Ctx, result authservice.SessionResult, err error) error {
	if err != nil {
		return credentialProblem(c, err)
	}
	if err := h.installSession(c, &result); err != nil {
		return httpx.WriteProblem(c, sessionProblem(err))
	}
	if result.User.RequiresMFA() {
		return c.JSON(fiber.Map{"authenticated": false, "mfaRequired": true, "csrfToken": result.CSRFToken})
	}
	return c.JSON(sessionResponse(result.User, result.CSRFToken))
}

func credentialProblem(c fiber.Ctx, err error) error {
	if errors.Is(err, authservice.ErrUnauthenticated) {
		return httpx.WriteProblem(c, httpx.NewProblem(401, "invalid_credentials", "Sign-in failed", "Check your sign-in details and try again."))
	}
	if errors.Is(err, repository.ErrConflict) {
		return httpx.WriteProblem(c, httpx.NewProblem(409, "account_link_required", "Use your existing sign-in method", "Sign in with your password or an email link for this account."))
	}
	return httpx.WriteProblem(c, httpx.NewProblem(503, "sign_in_unavailable", "Sign-in unavailable", "This sign-in method is currently unavailable. Please try another method."))
}
