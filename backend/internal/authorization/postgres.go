package authorization

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	authmodel "github.com/kingsgambit2700/platform/backend/internal/auth/model"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	"github.com/kingsgambit2700/platform/backend/internal/domain/support"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

var ErrMFAResetDenied = errors.New("authorization: mfa reset denied")

const (
	permissionManageGuardianRelationships = "family.guardian_relationship.manage"
	permissionManageReportDrafts          = "report.draft.manage"
	permissionPublishReports              = "report.publish"
	permissionViewReports                 = "report.view"
	permissionViewEntitlements            = "billing.entitlement.view"
	permissionGrantEntitlements           = "billing.entitlement.grant"
	permissionManageSupportCases          = "support.case.manage"

	scopeFamily  = "family"
	scopeStudent = "student"
)

type rowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Postgres evaluates permissions from current database state. It deliberately
// does not trust role claims cached in a request principal, so revocations take
// effect on the next authorization query.
type Postgres struct {
	database rowQuerier
}

func NewPostgres(pool *pgxpool.Pool) (*Postgres, error) {
	if pool == nil {
		return nil, errors.New("authorization: postgres pool is required")
	}
	return newPostgres(pool), nil
}

func newPostgres(database rowQuerier) *Postgres {
	return &Postgres{database: database}
}

var (
	_ family.Authorizer      = (*Postgres)(nil)
	_ report.Authorizer      = (*Postgres)(nil)
	_ entitlement.Authorizer = (*Postgres)(nil)
	_ support.Authorizer     = (*Postgres)(nil)
)

// AuthorizeMFAReset permits only an active super-admin to reset MFA for a
// different active staff user. Recent-MFA enforcement remains in auth.Service.
func (a *Postgres) AuthorizeMFAReset(
	ctx context.Context,
	actor authmodel.Principal,
	targetUserID string,
) error {
	actorID := actor.User.ID
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(targetUserID) || actorID == targetUserID {
		return ErrMFAResetDenied
	}

	var allowed bool
	err := a.database.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM users AS actor_user
				JOIN role_grants AS actor_role
				  ON actor_role.user_id = actor_user.id
				WHERE actor_user.id = $1::uuid
				  AND actor_user.status = 'active'
				  AND actor_role.role = 'super_admin'
				  AND actor_role.revoked_at IS NULL
			) AND EXISTS (
				SELECT 1
				FROM users AS target_user
				JOIN role_grants AS target_role
				  ON target_role.user_id = target_user.id
				WHERE target_user.id = $2::uuid
				  AND target_user.status = 'active'
				  AND target_role.role IN (
					'coach', 'academic_manager', 'operations_admin',
					'finance_admin', 'super_admin'
				  )
				  AND target_role.revoked_at IS NULL
			)`, actorID, targetUserID).Scan(&allowed)
	if err != nil {
		return fmt.Errorf("authorization: checking mfa reset scope: %w", err)
	}
	if !allowed {
		return ErrMFAResetDenied
	}
	return nil
}

func (a *Postgres) AuthorizeRelationshipManagement(
	ctx context.Context,
	actorID string,
	familyID string,
) error {
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(familyID) {
		return family.ErrPermissionDenied
	}
	active, err := a.activeUser(ctx, actorID)
	if err != nil {
		return fmt.Errorf("authorization: checking active actor: %w", err)
	}
	if !active {
		return family.ErrPermissionDenied
	}

	familyIsActive, err := a.activeFamily(ctx, familyID)
	if err != nil {
		return fmt.Errorf("authorization: checking family: %w", err)
	}
	if !familyIsActive {
		return family.ErrPermissionDenied
	}

	allowed, denied, err := a.permissionDecision(
		ctx,
		actorID,
		permissionManageGuardianRelationships,
		scopeFamily,
		familyID,
	)
	if err != nil {
		return fmt.Errorf("authorization: checking guardian-management permission: %w", err)
	}
	if denied {
		return family.ErrPermissionDenied
	}
	if allowed {
		return nil
	}

	allowed, err = a.guardianHasFamilyPermission(
		ctx,
		actorID,
		familyID,
		family.PermissionManageGuardians,
	)
	if err != nil {
		return fmt.Errorf("authorization: checking guardian family scope: %w", err)
	}
	if !allowed {
		return family.ErrPermissionDenied
	}
	return nil
}

func (a *Postgres) AuthorizeReport(
	ctx context.Context,
	actorID string,
	studentID string,
	action report.Action,
) error {
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(studentID) {
		return report.ErrPermissionDenied
	}
	active, err := a.activeUser(ctx, actorID)
	if err != nil {
		return fmt.Errorf("authorization: checking active actor: %w", err)
	}
	if !active {
		return report.ErrPermissionDenied
	}

	var allowed, denied bool
	switch action {
	case report.ActionCreateDraft, report.ActionEditDraft, report.ActionStartCorrection:
		allowed, denied, err = a.permissionDecision(
			ctx,
			actorID,
			permissionManageReportDrafts,
			scopeStudent,
			studentID,
		)
		if err == nil && !allowed && !denied {
			allowed, err = a.isAssignedCoach(ctx, actorID, studentID)
		}
	case report.ActionPublish:
		allowed, denied, err = a.permissionDecision(
			ctx,
			actorID,
			permissionPublishReports,
			scopeStudent,
			studentID,
		)
	case report.ActionView:
		allowed, denied, err = a.permissionDecision(
			ctx,
			actorID,
			permissionViewReports,
			scopeStudent,
			studentID,
		)
		if err == nil && !allowed && !denied {
			allowed, err = a.isStudent(ctx, actorID, studentID)
		}
		if err == nil && !allowed && !denied {
			allowed, err = a.isAssignedCoach(ctx, actorID, studentID)
		}
		if err == nil && !allowed && !denied {
			allowed, err = a.guardianHasStudentPermission(
				ctx,
				actorID,
				studentID,
				family.PermissionViewProgress,
			)
		}
	case report.ActionAcknowledge:
		// Acknowledgement is guardian evidence. Staff and students may view a
		// report through their scopes, but cannot acknowledge on a guardian's behalf.
		_, denied, err = a.permissionDecision(
			ctx,
			actorID,
			permissionViewReports,
			scopeStudent,
			studentID,
		)
		if err == nil && !denied {
			allowed, err = a.guardianHasStudentPermission(
				ctx,
				actorID,
				studentID,
				family.PermissionViewProgress,
			)
		}
	default:
		return report.ErrPermissionDenied
	}
	if err != nil {
		return fmt.Errorf("authorization: checking report action %q: %w", action, err)
	}
	if denied || !allowed {
		return report.ErrPermissionDenied
	}
	return nil
}

func (a *Postgres) AuthorizeEntitlement(
	ctx context.Context,
	actorID string,
	studentID string,
	action entitlement.Action,
) error {
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(studentID) {
		return entitlement.ErrPermissionDenied
	}
	active, err := a.activeUser(ctx, actorID)
	if err != nil {
		return fmt.Errorf("authorization: checking active entitlement actor: %w", err)
	}
	if !active {
		return entitlement.ErrPermissionDenied
	}

	permission := permissionViewEntitlements
	if action == entitlement.ActionGrant {
		permission = permissionGrantEntitlements
	} else if action != entitlement.ActionView {
		return entitlement.ErrPermissionDenied
	}
	allowed, denied, err := a.permissionDecision(
		ctx,
		actorID,
		permission,
		scopeStudent,
		studentID,
	)
	if err != nil {
		return fmt.Errorf("authorization: checking entitlement permission: %w", err)
	}
	if denied {
		return entitlement.ErrPermissionDenied
	}
	if allowed {
		return nil
	}
	if action != entitlement.ActionView {
		return entitlement.ErrPermissionDenied
	}
	allowed, err = a.guardianHasStudentPermission(
		ctx,
		actorID,
		studentID,
		family.PermissionManageBilling,
	)
	if err != nil {
		return fmt.Errorf("authorization: checking guardian billing scope: %w", err)
	}
	if !allowed {
		return entitlement.ErrPermissionDenied
	}
	return nil
}

func (a *Postgres) SupportAccess(
	ctx context.Context,
	actorID string,
	familyID string,
) (support.Access, error) {
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(familyID) {
		return support.Access{}, support.ErrPermissionDenied
	}
	active, err := a.activeUser(ctx, actorID)
	if err != nil {
		return support.Access{}, fmt.Errorf("authorization: checking active support actor: %w", err)
	}
	if !active {
		return support.Access{}, support.ErrPermissionDenied
	}
	familyIsActive, err := a.activeFamily(ctx, familyID)
	if err != nil {
		return support.Access{}, fmt.Errorf("authorization: checking support family: %w", err)
	}
	if !familyIsActive {
		return support.Access{}, support.ErrPermissionDenied
	}

	manage, denied, err := a.permissionDecision(
		ctx,
		actorID,
		permissionManageSupportCases,
		scopeFamily,
		familyID,
	)
	if err != nil {
		return support.Access{}, fmt.Errorf("authorization: checking support-management permission: %w", err)
	}
	if denied {
		manage = false
	}
	communicate, err := a.guardianHasFamilyPermission(
		ctx,
		actorID,
		familyID,
		family.PermissionCommunicateWithAcademy,
	)
	if err != nil {
		return support.Access{}, fmt.Errorf("authorization: checking family support scope: %w", err)
	}
	if !manage && !communicate {
		return support.Access{}, support.ErrPermissionDenied
	}
	return support.Access{
		CanViewFamily: communicate || manage,
		CanManage:     manage,
	}, nil
}

func (a *Postgres) AuthorizeSupportAssignee(
	ctx context.Context,
	actorID string,
	assigneeID string,
) error {
	if !platformid.ValidReference(actorID) || !platformid.ValidReference(assigneeID) {
		return support.ErrPermissionDenied
	}
	actorActive, err := a.activeUser(ctx, actorID)
	if err != nil {
		return fmt.Errorf("authorization: checking active support manager: %w", err)
	}
	assigneeActive, err := a.activeUser(ctx, assigneeID)
	if err != nil {
		return fmt.Errorf("authorization: checking active support assignee: %w", err)
	}
	if !actorActive || !assigneeActive {
		return support.ErrPermissionDenied
	}
	allowed, denied, err := a.permissionDecision(
		ctx,
		assigneeID,
		permissionManageSupportCases,
		scopeStudent,
		assigneeID,
	)
	if err != nil {
		return fmt.Errorf("authorization: checking support assignee permission: %w", err)
	}
	if denied || !allowed {
		return support.ErrPermissionDenied
	}
	return nil
}

func (a *Postgres) activeUser(ctx context.Context, actorID string) (bool, error) {
	var active bool
	err := a.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users
			WHERE id = $1::uuid AND status = 'active'
		)`, actorID).Scan(&active)
	return active, err
}

func (a *Postgres) activeFamily(ctx context.Context, familyID string) (bool, error) {
	var active bool
	err := a.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM families
			WHERE id = $1::uuid AND archived_at IS NULL
		)`, familyID).Scan(&active)
	return active, err
}

func (a *Postgres) permissionDecision(
	ctx context.Context,
	actorID string,
	permission string,
	scopeType string,
	scopeID string,
) (bool, bool, error) {
	var allowed, denied bool
	err := a.database.QueryRow(ctx, `
		SELECT
			EXISTS (
				SELECT 1
				FROM role_grants AS role_grant
				JOIN role_permission_defaults AS role_permission
				  ON role_permission.role = role_grant.role
				WHERE role_grant.user_id = $1::uuid
				  AND role_grant.revoked_at IS NULL
				  AND role_permission.permission_code = $2
			) OR EXISTS (
				SELECT 1
				FROM permission_grants AS permission_grant
				WHERE permission_grant.user_id = $1::uuid
				  AND permission_grant.permission_code = $2
				  AND permission_grant.effect = 'allow'
				  AND permission_grant.revoked_at IS NULL
				  AND (permission_grant.expires_at IS NULL OR permission_grant.expires_at > now())
				  AND (
					permission_grant.scope_type = 'global' OR
					(permission_grant.scope_type = $3 AND permission_grant.scope_id = $4::uuid)
				  )
			),
			EXISTS (
			SELECT 1
			FROM permission_grants AS permission_grant
			WHERE permission_grant.user_id = $1::uuid
			  AND permission_grant.permission_code = $2
			  AND permission_grant.effect = 'deny'
			  AND permission_grant.revoked_at IS NULL
			  AND (permission_grant.expires_at IS NULL OR permission_grant.expires_at > now())
			  AND (
				permission_grant.scope_type = 'global' OR
				(permission_grant.scope_type = $3 AND permission_grant.scope_id = $4::uuid)
			  )
			)`, actorID, permission, scopeType, scopeID).Scan(&allowed, &denied)
	return allowed, denied, err
}

func (a *Postgres) guardianHasFamilyPermission(
	ctx context.Context,
	actorID string,
	familyID string,
	permission family.Permission,
) (bool, error) {
	var allowed bool
	err := a.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM guardian_student_relationships
			WHERE guardian_user_id = $1::uuid
			  AND family_id = $2::uuid
			  AND status = 'active'
			  AND $3 = ANY(permissions)
		)`, actorID, familyID, string(permission)).Scan(&allowed)
	return allowed, err
}

func (a *Postgres) guardianHasStudentPermission(
	ctx context.Context,
	actorID string,
	studentID string,
	permission family.Permission,
) (bool, error) {
	var allowed bool
	err := a.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM guardian_student_relationships
			WHERE guardian_user_id = $1::uuid
			  AND student_profile_id = $2::uuid
			  AND status = 'active'
			  AND $3 = ANY(permissions)
		)`, actorID, studentID, string(permission)).Scan(&allowed)
	return allowed, err
}

func (a *Postgres) isAssignedCoach(
	ctx context.Context,
	actorID string,
	studentID string,
) (bool, error) {
	var allowed bool
	err := a.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM enrolments
			JOIN batches ON batches.id = enrolments.batch_id
			WHERE enrolments.student_profile_id = $2::uuid
			  AND enrolments.status IN ('active', 'paused')
			  AND batches.coach_user_id = $1::uuid
			  AND batches.status NOT IN ('archived', 'cancelled')
		)`, actorID, studentID).Scan(&allowed)
	return allowed, err
}

func (a *Postgres) isStudent(
	ctx context.Context,
	actorID string,
	studentID string,
) (bool, error) {
	var allowed bool
	err := a.database.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM student_profiles
			WHERE id = $2::uuid AND user_id = $1::uuid
		)`, actorID, studentID).Scan(&allowed)
	return allowed, err
}
