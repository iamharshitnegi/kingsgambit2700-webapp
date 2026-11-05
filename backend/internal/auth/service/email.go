package service

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/provider"
)

// ProviderEmailSender adapts the infrastructure-neutral email provider to the
// authentication service. The raw link is sent only to the provider; it is
// never returned from an API response or written to logs.
type ProviderEmailSender struct {
	sender provider.EmailSender
}

func NewProviderEmailSender(sender provider.EmailSender) *ProviderEmailSender {
	return &ProviderEmailSender{sender: sender}
}

func (s *ProviderEmailSender) SendPasswordVerification(ctx context.Context, to, link string, expiresAt time.Time) error {
	_, err := s.sender.Send(ctx, provider.EmailMessage{
		To:       to,
		Subject:  "Verify your new KingsGambit2700 password account",
		TextBody: fmt.Sprintf("You requested a new academy account with a username and password. This link verifies your email and activates the password you chose during registration. Only continue if you made this request. If you did not, ignore this email; no account will be created.\n\nVerify your account: %s\n\nThis one-time link expires at %s.", link, expiresAt.Format(time.RFC3339)),
		HTMLBody: fmt.Sprintf("<p>You requested a new academy account with a username and password. This link verifies your email and activates the password you chose during registration.</p><p>Only continue if you made this request. If you did not, ignore this email; no account will be created.</p><p><a href=\"%s\">Verify my new account</a></p><p>This one-time link expires at %s.</p>", html.EscapeString(link), expiresAt.Format(time.RFC3339)),
		Tags:     map[string]string{"kind": "password_registration"},
	})
	if err != nil {
		return fmt.Errorf("auth email: sending account verification: %w", err)
	}
	return nil
}

func (s *ProviderEmailSender) SendMagicLink(ctx context.Context, to, link string, expiresAt time.Time) error {
	_, err := s.sender.Send(ctx, provider.EmailMessage{
		To:       to,
		Subject:  "Your KingsGambit2700 sign-in link",
		TextBody: fmt.Sprintf("Sign in: %s\n\nThis one-time link expires at %s.", link, expiresAt.Format(time.RFC3339)),
		HTMLBody: fmt.Sprintf("<p><a href=\"%s\">Sign in to KingsGambit2700</a></p><p>This one-time link expires at %s.</p>", html.EscapeString(link), expiresAt.Format(time.RFC3339)),
		Tags:     map[string]string{"kind": "magic_link"},
	})
	if err != nil {
		return fmt.Errorf("auth email: sending magic link: %w", err)
	}
	return nil
}
