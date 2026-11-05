package ticket

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/auth/model"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var (
	ErrForbidden = errors.New("realtime ticket: forbidden")
	ErrInvalid   = errors.New("realtime ticket: invalid or expired")
)

const lifetime = 60 * time.Second

type Result struct {
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Service struct{ pool *pgxpool.Pool }

func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// Authorize checks current room eligibility for an already connected user.
// A consumed ticket never substitutes for live assignment and class state.
func (s *Service) Authorize(ctx context.Context, userID, sessionID string) error {
	if !platformid.ValidReference(userID) || !platformid.ValidReference(sessionID) {
		return ErrForbidden
	}
	var allowed bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM class_sessions AS class_session
			JOIN users AS active_user ON active_user.id = $1::uuid AND active_user.status = 'active'
			WHERE class_session.id = $2::uuid
			  AND class_session.status IN ('scheduled', 'join_open', 'live')
			  AND class_session.join_opens_at <= now() AND class_session.ends_at > now()
			  AND (
				EXISTS (
					SELECT 1 FROM role_grants WHERE user_id = $1::uuid
					AND role = 'super_admin' AND revoked_at IS NULL
				) OR EXISTS (
					SELECT 1 FROM session_assignments
					WHERE class_session_id = class_session.id AND user_id = $1::uuid
					AND starts_at <= now() AND (ends_at IS NULL OR ends_at > now())
				)
			  )
		)`, userID, sessionID).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("realtime ticket: checking current eligibility: %w", err)
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func (s *Service) Issue(ctx context.Context, principal model.Principal, sessionID string) (Result, error) {
	if !platformid.ValidReference(principal.User.ID) || !platformid.ValidReference(sessionID) {
		return Result{}, ErrForbidden
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return Result{}, fmt.Errorf("realtime ticket: generating token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(random)
	hash := sha256.Sum256([]byte(token))
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `
			INSERT INTO realtime_join_tickets
				(id, token_hash, user_id, class_session_id, expires_at, created_at)
			SELECT $1::uuid, $2, $3::uuid, class_session.id,
			       now() + (INTERVAL '1 second' * $4::double precision), now()
			FROM class_sessions AS class_session
			WHERE class_session.id = $5::uuid
			  AND class_session.status IN ('scheduled', 'join_open', 'live')
			  AND class_session.join_opens_at <= now()
			  AND class_session.ends_at > now()
			  AND EXISTS (
				SELECT 1
				FROM users AS active_user
				WHERE active_user.id = $3::uuid
				  AND active_user.status = 'active'
			  )
			  AND (
				EXISTS (
					SELECT 1
					FROM role_grants AS role_grant
					WHERE role_grant.user_id = $3::uuid
					  AND role_grant.role = 'super_admin'
					  AND role_grant.revoked_at IS NULL
				) OR EXISTS (
					SELECT 1
					FROM session_assignments AS assignment
					WHERE assignment.class_session_id = class_session.id
					  AND assignment.user_id = $3::uuid
					  AND assignment.starts_at <= now()
					  AND (assignment.ends_at IS NULL OR assignment.ends_at > now())
				)
			  )
			RETURNING expires_at`,
		platformid.New(),
		hash[:],
		principal.User.ID,
		int64(lifetime/time.Second),
		sessionID,
	).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrForbidden
	}
	if err != nil {
		return Result{}, fmt.Errorf("realtime ticket: storing token: %w", err)
	}
	return Result{Ticket: token, ExpiresAt: expiresAt}, nil
}

func (s *Service) Consume(ctx context.Context, rawToken, userID string) (string, error) {
	if rawToken == "" || !platformid.ValidReference(userID) {
		return "", ErrInvalid
	}
	hash := sha256.Sum256([]byte(rawToken))
	var sessionID string
	err := s.pool.QueryRow(ctx, `
			UPDATE realtime_join_tickets AS ticket
			SET consumed_at = now()
			FROM class_sessions AS class_session
			WHERE ticket.token_hash = $1
			  AND ticket.user_id = $2::uuid
			  AND ticket.consumed_at IS NULL
			  AND ticket.expires_at > now()
			  AND class_session.id = ticket.class_session_id
			  AND class_session.status IN ('scheduled', 'join_open', 'live')
			  AND class_session.join_opens_at <= now()
			  AND class_session.ends_at > now()
			  AND EXISTS (
				SELECT 1
				FROM users AS active_user
				WHERE active_user.id = ticket.user_id
				  AND active_user.status = 'active'
			  )
			  AND (
				EXISTS (
					SELECT 1
					FROM role_grants AS role_grant
					WHERE role_grant.user_id = ticket.user_id
					  AND role_grant.role = 'super_admin'
					  AND role_grant.revoked_at IS NULL
				) OR EXISTS (
					SELECT 1
					FROM session_assignments AS assignment
					WHERE assignment.class_session_id = class_session.id
					  AND assignment.user_id = ticket.user_id
					  AND assignment.starts_at <= now()
					  AND (assignment.ends_at IS NULL OR assignment.ends_at > now())
				)
			  )
			RETURNING ticket.class_session_id::text`, hash[:], userID).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalid
	}
	if err != nil {
		return "", fmt.Errorf("realtime ticket: consuming token: %w", err)
	}
	return sessionID, nil
}
