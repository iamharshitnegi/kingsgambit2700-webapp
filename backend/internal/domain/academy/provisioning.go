package academy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"html"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func (r *PostgresRepository) CreateStudent(ctx context.Context, actor string, in StudentInput, now time.Time) (Student, error) {
	out := Student{ID: platformid.New(), UserID: platformid.New(), Name: in.Name, AgeBand: in.AgeBand, TimeZone: in.TimeZone, Status: "prospect", Country: in.Country}
	err := r.withCreation(ctx, actor, "students", in, &out, func(tx pgx.Tx) error {
		guardian := actor
		admin := requirePermission(ctx, tx, actor, "academy.manage")
		if admin != nil && !errors.Is(admin, ErrDenied) {
			return admin
		}
		if in.GuardianEmail != "" {
			if admin != nil {
				return ErrDenied
			}
			err := tx.QueryRow(ctx, `SELECT id::text FROM users WHERE email=$1 AND status='active'`, in.GuardianEmail).Scan(&guardian)
			if errors.Is(err, pgx.ErrNoRows) {
				guardian = platformid.New()
				name := in.GuardianName
				if strings.TrimSpace(name) == "" {
					name = in.GuardianEmail
				}
				if err = insertUser(ctx, tx, guardian, in.GuardianEmail, name, "guardian", now); err != nil {
					return err
				}
			} else if err != nil {
				return err
			}
			ok, err := role(ctx, tx, guardian, "guardian")
			if err != nil {
				return err
			}
			if !ok {
				return invalid("the selected email belongs to a staff account; use the guardian's email")
			}
		} else {
			ok, err := role(ctx, tx, actor, "guardian")
			if err != nil {
				return err
			}
			if !ok {
				return invalid("provide a guardian email")
			}
		}
		err := tx.QueryRow(ctx, `SELECT family_id::text FROM guardian_student_relationships WHERE guardian_user_id=$1::uuid AND status='active' ORDER BY created_at LIMIT 1`, guardian).Scan(&out.FamilyID)
		if errors.Is(err, pgx.ErrNoRows) {
			out.FamilyID = platformid.New()
			if _, err = tx.Exec(ctx, `INSERT INTO families(id,display_name,created_at,updated_at) VALUES($1::uuid,$2,$3,$3)`, out.FamilyID, in.Name+"'s family", now); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		if err = insertUser(ctx, tx, out.UserID, "", in.Name, "student", now); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO student_profiles(id,user_id,preferred_name,age_band,country,time_zone,locale,language,status,created_at,updated_at) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,'en','en','prospect',$7,$7)`, out.ID, out.UserID, out.Name, in.AgeBand, in.Country, in.TimeZone, now)
		if err != nil {
			return err
		}
		perms := []family.Permission{family.PermissionViewProfile, family.PermissionManageProfile, family.PermissionViewSchedule, family.PermissionManageSchedule, family.PermissionJoinClass, family.PermissionViewProgress, family.PermissionManageConsent, family.PermissionManageBilling, family.PermissionManageGuardians, family.PermissionCommunicateWithAcademy}
		relation, err := family.NewRelationship(family.NewRelationshipParams{ID: platformid.New(), FamilyID: out.FamilyID, GuardianID: guardian, StudentID: out.ID, Permissions: perms, GrantedBy: actor, GrantedAt: now})
		if err != nil {
			return err
		}
		if err = family.CreateInTx(ctx, tx, relation, family.Event{ID: platformid.New(), RelationshipID: relation.ID(), Revision: 1, Kind: family.EventKindGranted, ActorID: actor, OccurredAt: now, Permissions: perms}); err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.student.created", out.ID, now, map[string]string{"familyId": out.FamilyID})
	})
	return out, err
}
func insertUser(ctx context.Context, tx pgx.Tx, id, email, name, roleName string, now time.Time) error {
	if _, err := tx.Exec(ctx, `INSERT INTO users(id,email,display_name,status,created_at,updated_at) VALUES($1::uuid,NULLIF($2,''),$3,'active',$4,$4)`, id, email, name, now); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,$3,$4)`, platformid.New(), id, roleName, now)
	return err
}
func (r *PostgresRepository) CreateStaff(ctx context.Context, actor string, in StaffInput, now time.Time) (Coach, error) {
	out := Coach{ID: platformid.New(), Name: in.Name}
	err := r.withCreation(ctx, actor, "staff", in, &out, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.staff.manage"); err != nil {
			return err
		}
		if err := insertUser(ctx, tx, out.ID, in.Email, in.Name, in.Role, now); err != nil {
			return err
		}
		if in.Role == "coach" {
			if _, err := tx.Exec(ctx, `INSERT INTO coach_profiles(id,user_id,slug,status,is_public,created_at,updated_at) VALUES($1::uuid,$2::uuid,$3,'active',false,$4,$4)`, platformid.New(), out.ID, "coach-"+out.ID, now); err != nil {
				return err
			}
		}
		return audit(ctx, tx, actor, "academy.staff.created", out.ID, now, map[string]string{"role": in.Role})
	})
	return out, err
}
func (r *PostgresRepository) CreateBatch(ctx context.Context, actor string, in BatchInput, now time.Time) (Batch, error) {
	out := Batch{ID: platformid.New(), Name: in.Name, CoachID: in.CoachID, Format: in.Format, Level: in.Level, TimeZone: in.TimeZone, Capacity: in.Capacity, StudentIDs: []string{}}
	err := r.withCreation(ctx, actor, "batches", in, &out, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		ok, err := role(ctx, tx, in.CoachID, "coach")
		if err != nil {
			return err
		}
		if !ok {
			return invalid("choose an active coach")
		}
		program := "one-to-one-coaching"
		target := 1
		if in.Format == "group" {
			program = "small-group-academy"
			target = min(4, in.Capacity)
		}
		_, err = tx.Exec(ctx, `INSERT INTO batches(id,program_id,name,level_slug,coach_user_id,format,source_time_zone,target_capacity,hard_capacity,status,created_at,updated_at) VALUES($1::uuid,(SELECT id FROM programs WHERE slug=$2 AND status='active'),$3,$4,$5::uuid,$6,$7,$8,$9,'active',$10,$10)`, out.ID, program, out.Name, out.Level, out.CoachID, out.Format, out.TimeZone, target, out.Capacity, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.batch.created", out.ID, now, map[string]string{"coachId": out.CoachID})
	})
	return out, err
}
func (r *PostgresRepository) CreateEnrolment(ctx context.Context, actor string, in EnrolmentInput, now time.Time) (Enrolment, error) {
	out := Enrolment{ID: platformid.New(), StudentID: in.StudentID, BatchID: in.BatchID, AccountID: platformid.New(), Status: "active", StartsAt: in.StartsAt, EndsAt: in.EndsAt}
	err := r.withCreation(ctx, actor, "enrolments", in, &out, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.manage"); err != nil {
			return err
		}
		ok, err := consentCurrent(ctx, tx, in.StudentID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrConsent
		}
		var capacity, count int
		err = tx.QueryRow(ctx, `SELECT hard_capacity FROM batches WHERE id=$1::uuid AND status='active' FOR UPDATE`, in.BatchID).Scan(&capacity)
		if err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `SELECT count(*) FROM enrolments WHERE batch_id=$1::uuid AND status IN('active','pending') AND (ends_at IS NULL OR ends_at>$2) AND ($3::timestamptz IS NULL OR starts_at<$3)`, in.BatchID, in.StartsAt, in.EndsAt).Scan(&count)
		if err != nil {
			return err
		}
		if count >= capacity {
			return invalid("this batch has no available seats")
		}
		var duplicate bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM enrolments WHERE batch_id=$1::uuid AND student_profile_id=$2::uuid AND status='active' AND (ends_at IS NULL OR ends_at>$3) AND ($4::timestamptz IS NULL OR starts_at<$4))`, in.BatchID, in.StudentID, in.StartsAt, in.EndsAt).Scan(&duplicate)
		if err != nil {
			return err
		}
		if duplicate {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `INSERT INTO enrolments(id,student_profile_id,batch_id,status,policy_snapshot,starts_at,ends_at,created_at,updated_at) VALUES($1::uuid,$2::uuid,$3::uuid,'active','{"version":"academy-v1","oneToOneCancellationHours":24,"groupTarget":4,"groupMaximum":6,"groupFamilyAbsence":"consume","academyFailure":"release_and_make_up"}'::jsonb,$4,$5,$6,$6)`, out.ID, out.StudentID, out.BatchID, out.StartsAt, out.EndsAt, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO entitlement_accounts(id,enrolment_id,unit,created_at) VALUES($1::uuid,$2::uuid,'session_credit',$3)`, out.AccountID, out.ID, now)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE student_profiles SET status='active',updated_at=$2 WHERE id=$1::uuid`, in.StudentID, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.enrolment.created", out.ID, now, map[string]string{"studentId": out.StudentID, "batchId": out.BatchID})
	})
	return out, err
}
func (r *PostgresRepository) PublishPolicy(ctx context.Context, actor string, in PolicyInput, now time.Time) (Policy, error) {
	out := Policy{ID: platformid.New(), Slug: in.Slug, Title: in.Title, Version: in.Version, Content: in.Content}
	err := r.withCreation(ctx, actor, "policies", in, &out, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.policy.manage"); err != nil {
			return err
		}
		var policyID string
		err := tx.QueryRow(ctx, `INSERT INTO policies(id,slug,title,created_at) VALUES($1::uuid,$2,$3,$4) ON CONFLICT(slug) DO UPDATE SET title=EXCLUDED.title RETURNING id::text`, platformid.New(), in.Slug, in.Title, now).Scan(&policyID)
		if err != nil {
			return err
		}
		content := "<p>" + strings.ReplaceAll(html.EscapeString(in.Content), "\n", "</p><p>") + "</p>"
		hash := sha256.Sum256([]byte(content))
		_, err = tx.Exec(ctx, `INSERT INTO policy_versions(id,policy_id,version,locale,purpose,content_html,content_hash,status,effective_at,published_at,created_by,created_at) VALUES($1::uuid,$2::uuid,$3,'en',$4,$5,$6,'published',$7,$7,$8::uuid,$7)`, out.ID, policyID, in.Version, in.Slug, content, hash[:], now, actor)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.policy.published", out.ID, now, map[string]string{"slug": in.Slug, "version": in.Version})
	})
	return out, err
}
func (r *PostgresRepository) SaveSettings(ctx context.Context, actor string, in Settings, now time.Time) (Settings, error) {
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := requirePermission(ctx, tx, actor, "academy.policy.manage"); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE organization_settings SET name=$1,legal_name=$2,email=$3,updated_at=$4`, in.Name, in.LegalName, in.Email, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.settings.updated", "organization", now, struct{}{})
	})
	return in, err
}
func (r *PostgresRepository) AcceptConsent(ctx context.Context, actor, student string, in ConsentInput, now time.Time) error {
	return r.within(ctx, func(tx pgx.Tx) error {
		// Consent is guardian evidence, never an administrative proxy acceptance.
		if err := studentPermission(ctx, tx, actor, student, "family.consent.accept", "manage_consent", false); err != nil {
			return err
		}
		var userID, country string
		if err := tx.QueryRow(ctx, `SELECT user_id::text,country FROM student_profiles WHERE id=$1::uuid`, student).Scan(&userID, &country); err != nil {
			return err
		}
		policies, err := currentPolicies(ctx, tx)
		if err != nil {
			return err
		}
		consentPolicies := []Policy{}
		for _, policy := range policies {
			if policy.Slug == "privacy" || policy.Slug == "child-safety" {
				consentPolicies = append(consentPolicies, policy)
			}
		}
		policies = consentPolicies
		if len(policies) != 2 {
			return ErrConsent
		}
		for _, policy := range policies {
			found := false
			for _, id := range in.PolicyVersionIDs {
				if id == policy.ID {
					found = true
				}
			}
			if !found {
				return ErrConsent
			}
		}
		var ready bool
		if err := tx.QueryRow(ctx, `SELECT legal_name IS NOT NULL AND legal_name<>'' AND email IS NOT NULL FROM organization_settings`).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			return ErrConsent
		}
		for _, policy := range policies {
			_, err = tx.Exec(ctx, `INSERT INTO consent_records(id,policy_version_id,subject_user_id,actor_user_id,purpose,region,locale,evidence,accepted_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,$5,$6,'en','{"source":"guardian_portal","explicitAcceptance":true}'::jsonb,$7)`, platformid.New(), policy.ID, userID, actor, policy.Slug, country, now)
			if err != nil {
				return err
			}
		}
		return audit(ctx, tx, actor, "academy.consent.accepted", student, now, map[string]any{"policyVersionIds": in.PolicyVersionIDs})
	})
}
func (r *PostgresRepository) IssueCode(ctx context.Context, actor, student string, now time.Time) (AccessCode, error) {
	out := AccessCode{ExpiresAt: now.Add(15 * time.Minute)}
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := studentPermission(ctx, tx, actor, student, "academy.manage", "manage_profile", false); err != nil {
			return err
		}
		var userID string
		if err := tx.QueryRow(ctx, `SELECT s.user_id::text FROM student_profiles s JOIN users u ON u.id=s.user_id WHERE s.id=$1::uuid AND u.status='active'`, student).Scan(&userID); err != nil {
			return err
		}
		bytes := make([]byte, 18)
		if _, err := rand.Read(bytes); err != nil {
			return err
		}
		out.Code = base64.RawURLEncoding.EncodeToString(bytes)
		digest := sha256.Sum256([]byte(out.Code))
		if _, err := tx.Exec(ctx, `UPDATE student_login_codes SET consumed_at=$2 WHERE student_user_id=$1::uuid AND consumed_at IS NULL`, userID, now); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO student_login_codes(id,student_user_id,code_hash,expires_at,created_by,created_at) VALUES($1::uuid,$2::uuid,$3,$4,$5::uuid,$6)`, platformid.New(), userID, digest[:], out.ExpiresAt, actor, now)
		if err != nil {
			return err
		}
		return audit(ctx, tx, actor, "academy.student_code.issued", student, now, struct{}{})
	})
	return out, err
}
