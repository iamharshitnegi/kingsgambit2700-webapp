package learning

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func academicPlacementRole(ctx context.Context, q querier, actor string) (bool, error) {
	var allowed bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM role_grants r JOIN users u ON u.id=r.user_id WHERE r.user_id=$1::uuid AND r.role IN('academic_manager','super_admin') AND r.revoked_at IS NULL AND u.status='active')`, actor).Scan(&allowed)
	return allowed, err
}

const currentLevelSQL = `COALESCE((SELECT level_slug FROM learning_placements WHERE student_id=s.id ORDER BY created_at DESC,id DESC LIMIT 1),(SELECT b.level_slug FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=s.id AND e.status='active' ORDER BY e.created_at DESC,e.id DESC LIMIT 1),'')`

func (s *Service) placeStudent(ctx context.Context, tx pgx.Tx, actor string, c Command) error {
	role, err := academicPlacementRole(ctx, tx, actor)
	if err != nil {
		return err
	}
	if !role {
		return ErrDenied
	}
	if err = authorizeStudent(ctx, tx, actor, c.StudentID, "learning.publish", "", false); err != nil {
		return err
	}
	if !required(c.Feedback, 3000) || len(c.Feedback) < 5 || !platformid.ValidReference(c.AssessmentID) {
		return invalid("add a placement reason and a published report as evidence")
	}
	var previous string
	if _, err = tx.Exec(ctx, `SELECT id FROM student_profiles WHERE id=$1::uuid FOR UPDATE`, c.StudentID); err != nil {
		return err
	}
	if err = tx.QueryRow(ctx, `SELECT `+currentLevelSQL+` FROM student_profiles s WHERE s.id=$1::uuid`, c.StudentID).Scan(&previous); err != nil {
		return err
	}
	if previous == c.LevelSlug {
		return invalid("the learner already has this level placement")
	}
	var version string
	err = tx.QueryRow(ctx, `SELECT id::text FROM curriculum_versions WHERE status='published' AND effective_at<=now() AND jsonb_path_exists(public_json,'$.tracks[*].levels[*] ? (@.slug == $level)',jsonb_build_object('level',$1::text)) ORDER BY effective_at DESC,created_at DESC LIMIT 1`, c.LevelSlug).Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return invalid("choose a level from the published academy curriculum")
	}
	if err != nil {
		return err
	}
	var reportMatches bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM progress_report_versions v JOIN progress_reports p ON p.id=v.progress_report_id WHERE v.id=$1::uuid AND p.student_profile_id=$2::uuid AND v.published_at IS NOT NULL)`, c.AssessmentID, c.StudentID).Scan(&reportMatches)
	if err != nil {
		return err
	}
	if !reportMatches {
		return invalid("choose a published report for this learner")
	}
	_, err = tx.Exec(ctx, `INSERT INTO learning_placements(id,student_id,curriculum_version_id,level_slug,previous_level_slug,report_version_id,reason,placed_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6::uuid,$7,$8::uuid)`, platformid.New(), c.StudentID, version, c.LevelSlug, previous, c.AssessmentID, c.Feedback, actor)
	if err != nil {
		return err
	}
	return notifyStudent(ctx, tx, c.StudentID, "Your curriculum placement has been updated", "/portal/progress")
}
