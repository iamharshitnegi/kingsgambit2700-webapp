package repository

import (
	"context"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
)

// CredentialsRepository keeps primary credential exchange atomic with session creation.
type CredentialsRepository interface {
	CreatePasswordRegistration(context.Context, model.PasswordRegistration) error
	CompletePasswordRegistration(context.Context, []byte, model.Session, time.Time) (model.User, model.Session, string, error)
	PasswordIdentity(context.Context, string) (model.PasswordIdentity, error)
	CreatePasswordSession(context.Context, model.PasswordIdentity, model.Session, time.Time) (model.User, model.Session, error)
	CreateGoogleChallenge(context.Context, []byte, time.Time) error
	ExchangeGoogle(context.Context, model.GoogleIdentity, []byte, model.Session, time.Time) (model.User, model.Session, error)
}
