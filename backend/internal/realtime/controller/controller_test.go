package controller

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	authrepository "github.com/kingsgambit2700/platform/backend/internal/auth/repository"
	authservice "github.com/kingsgambit2700/platform/backend/internal/auth/service"
	"github.com/kingsgambit2700/platform/backend/internal/domain/classroom"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/realtime/ticket"
)

type discardEmail struct{}

func (discardEmail) SendMagicLink(context.Context, string, string, time.Time) error { return nil }
func (discardEmail) SendPasswordVerification(context.Context, string, string, time.Time) error {
	return nil
}

type liveRoomAccess struct{ denied bool }

func (*liveRoomAccess) Consume(context.Context, string, string) (string, error) {
	return "", errors.New("unexpected ticket consumption")
}

func (r *liveRoomAccess) Authorize(context.Context, string, string) error {
	if r.denied {
		return ticket.ErrForbidden
	}
	return nil
}

func TestConnectedClassroomRechecksSessionAndRole(t *testing.T) {
	for _, change := range []string{"session revoked", "user suspended", "role removed", "assignment revoked"} {
		t.Run(change, func(t *testing.T) {
			repo := authrepository.NewMemory()
			user := model.User{ID: platformid.New(), DisplayName: "Student", Roles: []model.Role{model.RoleStudent}, Status: "active", CreatedAt: time.Now()}
			repo.PutUser(user)
			code := "student-access-code-for-test"
			hash := sha256.Sum256([]byte(code))
			if err := repo.CreateStudentCode(t.Context(), model.StudentCode{
				ID: platformid.New(), StudentUserID: user.ID, CodeHash: hash[:], ExpiresAt: time.Now().Add(time.Hour),
			}); err != nil {
				t.Fatal(err)
			}
			auth, err := authservice.New(repo, discardEmail{}, authservice.Config{
				MagicLinkTTL: time.Minute, SessionIdleTTL: time.Hour, SessionAbsoluteTTL: 24 * time.Hour, CSRFTTL: time.Hour,
			})
			if err != nil {
				t.Fatal(err)
			}
			session, err := auth.ExchangeStudentCode(t.Context(), code, "test", "127.0.0.1")
			if err != nil {
				t.Fatal(err)
			}
			room := &liveRoomAccess{}
			handler := &Controller{auth: auth, tickets: room}
			classID := platformid.New()
			actor, err := handler.authorizeCommand(t.Context(), session.Token, user.ID, classID)
			if err != nil || actor.Role != classroom.ActorRoleStudent {
				t.Fatalf("initial authorization = %+v, %v", actor, err)
			}
			switch change {
			case "session revoked":
				if err := repo.RevokeUserSession(t.Context(), user.ID, session.Session.ID, time.Now()); err != nil {
					t.Fatal(err)
				}
			case "user suspended":
				user.Status = "suspended"
				repo.PutUser(user)
			case "role removed":
				user.Roles = []model.Role{model.RoleGuardian}
				repo.PutUser(user)
			case "assignment revoked":
				room.denied = true
			}
			if _, err := handler.authorizeCommand(t.Context(), session.Token, user.ID, classID); err == nil {
				t.Fatal("connected classroom retained access after revocation")
			}
		})
	}
}
