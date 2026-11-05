package learning

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func (s *Service) assessmentAction(ctx context.Context, tx pgx.Tx, actor string, c Command) error {
	if c.Action == "revokeCertificate" {
		var student, status string
		err := tx.QueryRow(ctx, `SELECT student_id::text,status FROM learning_certificates WHERE id=$1::uuid FOR UPDATE`, c.ID).Scan(&student, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err = authorizeStudent(ctx, tx, actor, student, "learning.publish", "", false); err != nil {
			return err
		}
		if status != "issued" {
			return ErrConflict
		}
		if !required(c.Feedback, 3000) {
			return invalid("add a reason for revocation")
		}
		_, err = tx.Exec(ctx, `UPDATE learning_certificates SET status='revoked',reason=$2,revoked_at=now() WHERE id=$1::uuid`, c.ID, c.Feedback)
		return err
	}
	if c.Action == "createAssessment" {
		if err := authorizeStudent(ctx, tx, actor, c.StudentID, "learning.manage", "", false); err != nil {
			return err
		}
		var dimensions []string
		err := tx.QueryRow(ctx, `SELECT dimensions FROM learning_rubrics WHERE id=$1::uuid`, c.RubricID).Scan(&dimensions)
		if errors.Is(err, pgx.ErrNoRows) {
			return invalid("choose an available rubric")
		}
		if err != nil {
			return err
		}
		if err = validateAssessment(c, dimensions); err != nil {
			return err
		}
		content, err := encode(c.AssessmentContent)
		if err != nil {
			return err
		}
		id := platformid.New()
		date, _ := time.Parse("2006-01", c.Period)
		period, err := report.NewMonthlyPeriod(date.Year(), date.Month())
		if err != nil {
			return err
		}
		r, err := report.New(report.NewParams{ID: platformid.New(), StudentID: c.StudentID, Period: period, VersionID: id, RubricVersionID: c.RubricID, Content: canonicalContent(c.AssessmentContent), ActorID: actor, CreatedAt: nowUTC()})
		if err != nil {
			return err
		}
		if err = saveCanonical(ctx, tx, r, 0); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO learning_assessments(id,report_id,report_version_id,student_id,rubric_id,author_id,title,period,content,status) VALUES($1::uuid,$2::uuid,$1::uuid,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8::jsonb,'draft')`, id, r.ID(), c.StudentID, c.RubricID, actor, c.Title, c.Period, content); err != nil {
			return err
		}
		return assessmentHistory(ctx, tx, id, actor, c.Action, content)
	}
	a, r, sidecar, err := loadAssessment(ctx, tx, c.ID, actor, true)
	if err != nil {
		return err
	}
	content, err := encode(a.Content)
	if err != nil {
		return err
	}
	if err = conflictVersion(a.Version, c); err != nil {
		return err
	}
	code := "learning.manage"
	if c.Action == "publishAssessment" || c.Action == "returnAssessment" || c.Action == "issueCertificate" {
		code = "learning.publish"
	}
	if c.Action == "acknowledgeAssessment" {
		if a.Status != "published" {
			return ErrConflict
		}
		if err = authorizeStudent(ctx, tx, actor, a.StudentID, "learning.view", "view_progress", false); err != nil {
			return err
		}
		var guardian bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM guardian_student_relationships WHERE guardian_user_id=$1::uuid AND student_profile_id=$2::uuid AND status='active' AND 'view_progress'=ANY(permissions))`, actor, a.StudentID).Scan(&guardian); err != nil {
			return err
		}
		if !guardian {
			return ErrDenied
		}
		updated, changed, ackErr := r.Acknowledge(a.ID, actor, nowUTC())
		if ackErr != nil {
			return ackErr
		}
		if !changed {
			return nil
		}
		return saveCanonical(ctx, tx, updated, r.Revision())
	}
	if err = authorizeStudent(ctx, tx, actor, a.StudentID, code, "", false); err != nil {
		return err
	}
	if c.Action == "correctAssessment" {
		return correctAssessment(ctx, tx, actor, c, a, r)
	}
	if !sidecar {
		if c.Action == "editAssessment" && a.Status == "draft" {
			return adoptLegacyDraft(ctx, tx, actor, c, a, r)
		}
		return invalid("start a correction with an academy rubric to update this legacy report")
	}
	switch c.Action {
	case "editAssessment":
		if a.Status != "draft" && a.Status != "returned" {
			return ErrConflict
		}
		var dimensions []string
		if err = tx.QueryRow(ctx, `SELECT dimensions FROM learning_rubrics WHERE id=$1::uuid`, a.RubricID).Scan(&dimensions); err != nil {
			return err
		}
		if err = validateAssessment(c, dimensions); err != nil {
			return err
		}
		if c.Period != a.Period {
			return invalid("a monthly report cannot change its reporting period")
		}
		updated, editErr := r.EditDraft(a.ID, canonicalContent(c.AssessmentContent), actor, nowUTC())
		if editErr != nil {
			return editErr
		}
		if err = saveCanonical(ctx, tx, updated, r.Revision()); err != nil {
			return err
		}
		content, err = encode(c.AssessmentContent)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE learning_assessments SET title=$2,period=$3,content=$4::jsonb,status='draft',version=version+1,updated_at=now() WHERE id=$1::uuid`, a.ID, c.Title, c.Period, content)
	case "submitAssessment":
		if a.Status != "draft" && a.Status != "returned" {
			return ErrConflict
		}
		_, err = tx.Exec(ctx, `UPDATE learning_assessments SET status='submitted',version=version+1,updated_at=now() WHERE id=$1::uuid`, a.ID)
	case "returnAssessment":
		if a.Status != "submitted" {
			return ErrConflict
		}
		if !required(c.Feedback, 3000) {
			return invalid("add feedback explaining the requested changes")
		}
		if _, err = tx.Exec(ctx, `UPDATE learning_assessments SET status='returned',version=version+1,updated_at=now() WHERE id=$1::uuid`, a.ID); err != nil {
			return err
		}
		content, err = encode(map[string]any{"feedback": c.Feedback, "assessment": a.Content})
		if err != nil {
			return err
		}
	case "publishAssessment":
		if a.Status != "submitted" {
			return ErrConflict
		}
		updated, publishErr := r.Publish(a.ID, actor, nowUTC())
		if publishErr != nil {
			return publishErr
		}
		if err = saveCanonical(ctx, tx, updated, r.Revision()); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE learning_assessments SET status='published',published_at=now(),version=version+1,updated_at=now() WHERE id=$1::uuid`, a.ID); err != nil {
			return err
		}
		err = notifyStudent(ctx, tx, a.StudentID, "New learning update: "+a.Title, "/portal/progress")
	case "issueCertificate":
		if a.Status != "published" || !a.Content.Eligible {
			return invalid("a published assessment must recommend level completion")
		}
		var level string
		if err = tx.QueryRow(ctx, `SELECT level_slug FROM learning_rubrics WHERE id=$1::uuid`, a.RubricID).Scan(&level); err != nil {
			return err
		}
		tag, insertErr := tx.Exec(ctx, `INSERT INTO learning_certificates(id,student_id,assessment_id,level_slug,issuer_id,status) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,'issued') ON CONFLICT(student_id,assessment_id) DO NOTHING`, platformid.New(), a.StudentID, a.ID, level, actor)
		if insertErr != nil {
			return insertErr
		}
		if tag.RowsAffected() == 0 {
			return ErrConflict
		}
		err = notifyStudent(ctx, tx, a.StudentID, "Level completion recorded: "+level, "/portal/progress")
	default:
		return invalid("unknown assessment action")
	}
	if err != nil {
		return err
	}
	return assessmentHistory(ctx, tx, a.ID, actor, c.Action, content)
}
func assessmentHistory(ctx context.Context, tx pgx.Tx, id, actor, action string, content []byte) error {
	_, err := tx.Exec(ctx, `INSERT INTO learning_assessment_history(id,assessment_id,actor_id,action,content) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb)`, platformid.New(), id, actor, action, content)
	return err
}
