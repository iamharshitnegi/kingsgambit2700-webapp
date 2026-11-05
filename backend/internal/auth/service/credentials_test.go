package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/auth/password"
	"github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	"google.golang.org/api/idtoken"
)

type credentialTestRepository struct {
	*repository.Memory
	repository.CredentialsRepository
	identity     model.PasswordIdentity
	lookupErr    error
	user         model.User
	exchanges    int
	registration model.PasswordRegistration
}

func (r *credentialTestRepository) PasswordIdentity(context.Context, string) (model.PasswordIdentity, error) {
	return r.identity, r.lookupErr
}
func (r *credentialTestRepository) CreatePasswordSession(_ context.Context, _ model.PasswordIdentity, session model.Session, _ time.Time) (model.User, model.Session, error) {
	r.exchanges++
	session.UserID = r.user.ID
	return r.user, session, nil
}
func (r *credentialTestRepository) CreatePasswordRegistration(_ context.Context, registration model.PasswordRegistration) error {
	r.registration = registration
	return nil
}

func TestPasswordLoginAndSignupValidation(t *testing.T) {
	hash, err := password.Hash("LocalChess!2700")
	if err != nil {
		t.Fatal(err)
	}
	repo := &credentialTestRepository{Memory: repository.NewMemory(), identity: model.PasswordIdentity{UserID: "coach", PasswordHash: hash},
		user: model.User{ID: "coach", Status: "active", Roles: []model.Role{model.RoleCoach}}}
	email := &captureEmail{}
	svc, err := New(repo, email, Config{MagicLinkTTL: time.Minute, SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour, CSRFTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	result, err := svc.LoginPassword(t.Context(), "coach", "LocalChess!2700", "ua", "ip")
	if err != nil || !result.User.RequiresMFA() || result.Session.MFAVerifiedAt != nil || result.Token == "" {
		t.Fatalf("staff primary login = %+v, %v", result.User, err)
	}
	for _, missing := range []bool{false, true} {
		if missing {
			repo.lookupErr = repository.ErrNotFound
		}
		if _, err := svc.LoginPassword(t.Context(), "coach", "wrong password", "ua", "ip"); !errors.Is(err, ErrUnauthenticated) {
			t.Fatalf("bad credential: %v", err)
		}
	}
	if repo.exchanges != 1 {
		t.Fatal("invalid credentials created a session")
	}
	input := PasswordSignup{Email: "Parent@example.com", Username: "Parent.One", DisplayName: "Parent", Password: "LocalChess!2700", RedirectPath: "//evil.test"}
	if err := svc.RegisterPassword(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if repo.registration.Username != "parent.one" || repo.registration.RedirectPath != "/portal" ||
		!password.Verify(repo.registration.PasswordHash, input.Password) || strings.Contains(email.link, input.Password) || !email.verification {
		t.Fatal("unsafe signup data")
	}
	input.Username = "parent@example.com"
	if err := svc.RegisterPassword(t.Context(), input); !errors.Is(err, ErrInvalidInput) {
		t.Fatal("accepted ambiguous username")
	}
}

func TestGoogleIdentityRejectsUntrustedClaims(t *testing.T) {
	now := time.Now()
	for _, invalid := range []string{"valid", "issuer", "audience", "expiry", "future", "nonce", "unverified", "subject", "email", "third_party"} {
		t.Run(invalid, func(t *testing.T) {
			payload := &idtoken.Payload{Issuer: "https://accounts.google.com", Audience: "client", Subject: "subject", Expires: now.Add(time.Hour).Unix(), IssuedAt: now.Unix(),
				Claims: map[string]any{"nonce": "nonce", "email_verified": true, "email": "parent@gmail.com"}}
			switch invalid {
			case "issuer":
				payload.Issuer = "https://evil.test"
			case "audience":
				payload.Audience = "other-client"
			case "expiry":
				payload.Expires = now.Unix()
			case "future":
				payload.IssuedAt = now.Add(time.Hour).Unix()
			case "nonce":
				payload.Claims["nonce"] = "another-browser"
			case "unverified":
				payload.Claims["email_verified"] = false
			case "subject":
				payload.Subject = ""
			case "email":
				payload.Claims["email"] = "Parent <parent@gmail.com>"
			case "third_party":
				payload.Claims["email"] = "parent@example.com"
			}
			identity, err := googleIdentity(payload, "nonce", "client", now)
			if invalid == "valid" || invalid == "third_party" {
				if err != nil || identity.EmailAuthoritative != (invalid == "valid") {
					t.Fatalf("identity = %+v, %v", identity, err)
				}
			} else if !errors.Is(err, ErrUnauthenticated) {
				t.Fatalf("accepted %s", invalid)
			}
		})
	}
}

func TestGoogleLoginRejectsInvalidSignatureBeforeRepository(t *testing.T) {
	svc := &Service{cfg: Config{GoogleClientID: "client"}, validateGoogleToken: func(context.Context, string, string) (*idtoken.Payload, error) {
		return nil, errors.New("invalid signature")
	}}
	if _, err := svc.LoginGoogle(t.Context(), "forged-token", strings.Repeat("a", 43), "ua", "ip"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("forged token: %v", err)
	}
}
