package learning

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// The same live predicate is used for discovery and individual actions. Explicit
// student/family/global denies precede role defaults and relationship grants.
const studentAccessSQL = `
 EXISTS(SELECT 1 FROM users u WHERE u.id=$1::uuid AND u.status='active')
 AND NOT EXISTS(SELECT 1 FROM permission_grants g WHERE g.user_id=$1::uuid AND g.permission_code=$2
   AND g.effect='deny' AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>now())
   AND (g.scope_type='global' OR (g.scope_type='student' AND g.scope_id=s.id)
     OR (g.scope_type='family' AND g.scope_id IN (SELECT family_id FROM guardian_student_relationships WHERE student_profile_id=s.id AND status='active'))))
 AND (
 EXISTS(SELECT 1 FROM role_grants r JOIN role_permission_defaults d ON d.role=r.role
   WHERE r.user_id=$1::uuid AND r.revoked_at IS NULL AND d.permission_code=$2)
 OR EXISTS(SELECT 1 FROM permission_grants g WHERE g.user_id=$1::uuid AND g.permission_code=$2
   AND g.effect='allow' AND g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>now())
   AND (g.scope_type='global' OR (g.scope_type='student' AND g.scope_id=s.id)
     OR (g.scope_type='family' AND g.scope_id IN (SELECT family_id FROM guardian_student_relationships WHERE student_profile_id=s.id AND status='active'))))
 OR ($4 AND s.user_id=$1::uuid)
 OR ($3<>'' AND EXISTS(SELECT 1 FROM guardian_student_relationships g WHERE g.student_profile_id=s.id
   AND g.guardian_user_id=$1::uuid AND g.status='active' AND $3=ANY(g.permissions)))
 OR ($2<>'learning.publish' AND EXISTS(SELECT 1 FROM role_grants r WHERE r.user_id=$1::uuid AND r.role='coach' AND r.revoked_at IS NULL)
   AND (EXISTS(SELECT 1 FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=s.id AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.status='active' AND b.coach_user_id=$1::uuid)
   OR EXISTS(SELECT 1 FROM session_assignments coach JOIN session_assignments learner ON learner.class_session_id=coach.class_session_id
     WHERE coach.user_id=$1::uuid AND coach.role='coach' AND coach.starts_at<=now() AND (coach.ends_at IS NULL OR coach.ends_at>now())
       AND learner.user_id=s.user_id AND learner.role='student' AND (learner.ends_at IS NULL OR learner.ends_at>now()))))
 )`

func authorizeStudent(ctx context.Context, q querier, actor, student, permission, guardian string, self bool) error {
	if !platformid.ValidReference(actor) || !platformid.ValidReference(student) {
		return ErrDenied
	}
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM student_profiles s WHERE s.id=$5::uuid AND `+studentAccessSQL+`)`, actor, permission, guardian, self, student).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("learning: checking student access: %w", err)
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}
func permission(ctx context.Context, q querier, actor, code string) (bool, error) {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid AND status='active')
 AND NOT EXISTS(SELECT 1 FROM permission_grants WHERE user_id=$1::uuid AND permission_code=$2 AND effect='deny'
 AND scope_type='global' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now()))
 AND (EXISTS(SELECT 1 FROM role_grants r JOIN role_permission_defaults d ON r.role=d.role WHERE r.user_id=$1::uuid AND r.revoked_at IS NULL AND d.permission_code=$2)
 OR EXISTS(SELECT 1 FROM permission_grants WHERE user_id=$1::uuid AND permission_code=$2 AND effect='allow' AND scope_type='global' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now())))`, actor, code).Scan(&allowed)
	if err != nil {
		return false, fmt.Errorf("learning: checking permission: %w", err)
	}
	return allowed, nil
}
func staffResourceAccess(ctx context.Context, q querier, actor string) (bool, error) {
	allowed, err := permission(ctx, q, actor, "learning.manage")
	if err != nil || allowed {
		return allowed, err
	}
	var coach bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_grants r JOIN users u ON u.id=r.user_id WHERE r.user_id=$1::uuid AND r.role='coach' AND r.revoked_at IS NULL AND u.status='active')
 AND NOT EXISTS(SELECT 1 FROM permission_grants WHERE user_id=$1::uuid AND permission_code='learning.manage' AND effect='deny' AND scope_type='global' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now()))`, actor).Scan(&coach)
	return coach, err
}
func ownStudent(ctx context.Context, q querier, actor, student string) error {
	var own bool
	if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM student_profiles s JOIN users u ON u.id=s.user_id WHERE s.id=$1::uuid AND s.user_id=$2::uuid AND u.status='active')`, student, actor).Scan(&own); err != nil {
		return err
	}
	if !own {
		return ErrDenied
	}
	return authorizeStudent(ctx, q, actor, student, "learning.view", "", true)
}
