package learning

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/domain/report"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

// Rubric assessments extend, rather than replace, the existing monthly report
// aggregate. The report owns narratives, published versions and family receipts.
func canonicalContent(c AssessmentContent) report.Content {
	return report.Content{Learned: report.Written(c.Learned), Strengths: report.Written(c.Strengths), FocusNext: report.Written(c.FocusNext), PracticePlan: report.Written(c.PracticePlan), CoachComment: report.Written(c.CoachComment)}
}
func sectionText(s report.Section) string {
	if s.Text != "" {
		return s.Text
	}
	return s.OmittedReason
}
func saveCanonical(ctx context.Context, tx pgx.Tx, r report.Report, expected uint64) error {
	err := report.SaveInTransaction(ctx, tx, r, expected)
	if errors.Is(err, report.ErrConflict) || errors.Is(err, report.ErrPublishedImmutable) || errors.Is(err, report.ErrDraftExists) {
		return ErrConflict
	}
	return err
}
func loadAssessment(ctx context.Context, tx pgx.Tx, id, actor string, lock bool) (Assessment, report.Report, bool, error) {
	var reportID string
	if err := tx.QueryRow(ctx, `SELECT progress_report_id::text FROM progress_report_versions WHERE id=$1::uuid`, id).Scan(&reportID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Assessment{}, report.Report{}, false, ErrNotFound
		}
		return Assessment{}, report.Report{}, false, err
	}
	r, err := report.LoadInTransaction(ctx, tx, reportID, lock)
	if err != nil {
		return Assessment{}, r, false, err
	}
	v, err := r.Version(id)
	if err != nil {
		return Assessment{}, r, false, err
	}
	a := Assessment{ID: id, ReportID: r.ID(), StudentID: r.StudentID(), RubricID: v.RubricVersionID, Title: "Monthly learning update", Period: r.Period().Start.Format("2006-01"), Status: string(v.Status), Version: int(r.Revision()), PublishedAt: v.PublishedAt, History: []AssessmentEvent{}}
	a.Content.Ratings = []Rating{}
	var raw []byte
	var sideVersion int
	err = tx.QueryRow(ctx, `SELECT title,rubric_id::text,content,status,version FROM learning_assessments WHERE id=$1::uuid`, id).Scan(&a.Title, &a.RubricID, &raw, &a.Status, &sideVersion)
	sidecar := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return a, r, false, err
	}
	if sidecar {
		if err = json.Unmarshal(raw, &a.Content); err != nil {
			return a, r, false, err
		}
		a.Version += sideVersion
	}
	if v.Status == report.VersionStatusPublished {
		a.Status = "published"
	}
	a.Content.Learned = sectionText(v.Content.Learned)
	a.Content.Strengths = sectionText(v.Content.Strengths)
	a.Content.FocusNext = sectionText(v.Content.FocusNext)
	a.Content.PracticePlan = sectionText(v.Content.PracticePlan)
	a.Content.CoachComment = sectionText(v.Content.CoachComment)
	a.SourceVersionID = v.SourceVersionID
	for _, receipt := range r.Receipts() {
		if receipt.VersionID == id && receipt.ViewerID == actor {
			a.AcknowledgedAt = receipt.AcknowledgedAt
		}
	}
	return a, r, sidecar, nil
}
func correctAssessment(ctx context.Context, tx pgx.Tx, actor string, c Command, a Assessment, r report.Report) error {
	if a.Status != "published" {
		return ErrConflict
	}
	if c.RubricID == "" {
		c.RubricID = a.RubricID
	}
	if c.Title == "" {
		c.Title = a.Title
	}
	c.Period = a.Period
	c.StudentID = a.StudentID
	if c.AssessmentContent.Learned == "" {
		c.AssessmentContent = a.Content
	}
	var dimensions []string
	err := tx.QueryRow(ctx, `SELECT dimensions FROM learning_rubrics WHERE id=$1::uuid`, c.RubricID).Scan(&dimensions)
	if errors.Is(err, pgx.ErrNoRows) {
		return invalid("choose an academy rubric for this correction")
	}
	if err != nil {
		return err
	}
	if err = validateAssessment(c, dimensions); err != nil {
		return err
	}
	id := platformid.New()
	updated, err := r.StartCorrection(id, a.ID, actor, nowUTC())
	if err != nil {
		return ErrConflict
	}
	record := updated.Record()
	record.Versions[len(record.Versions)-1].RubricVersionID = c.RubricID
	updated, err = report.Restore(record)
	if err != nil {
		return err
	}
	if err = saveCanonical(ctx, tx, updated, r.Revision()); err != nil {
		return err
	}
	// Reload the persisted version before its next mutation. PostgreSQL stores
	// timestamps at microsecond precision; the aggregate compares identities exactly.
	updated, err = report.LoadInTransaction(ctx, tx, r.ID(), true)
	if err != nil {
		return err
	}
	edited, err := updated.EditDraft(id, canonicalContent(c.AssessmentContent), actor, nowUTC())
	if err != nil {
		return err
	}
	if err = saveCanonical(ctx, tx, edited, updated.Revision()); err != nil {
		return err
	}
	content, err := encode(c.AssessmentContent)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO learning_assessments(id,report_id,report_version_id,student_id,rubric_id,author_id,title,period,content,status) VALUES($1::uuid,$2::uuid,$1::uuid,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8::jsonb,'draft')`, id, r.ID(), a.StudentID, c.RubricID, actor, c.Title, a.Period, content)
	if err != nil {
		return err
	}
	return assessmentHistory(ctx, tx, id, actor, c.Action, content)
}

func adoptLegacyDraft(ctx context.Context, tx pgx.Tx, actor string, c Command, a Assessment, r report.Report) error {
	if c.Period != a.Period {
		return invalid("a monthly report cannot change its reporting period")
	}
	var dimensions []string
	err := tx.QueryRow(ctx, `SELECT dimensions FROM learning_rubrics WHERE id=$1::uuid`, c.RubricID).Scan(&dimensions)
	if errors.Is(err, pgx.ErrNoRows) {
		return invalid("choose a current academy rubric for this draft")
	}
	if err != nil {
		return err
	}
	if err = validateAssessment(c, dimensions); err != nil {
		return err
	}
	updated, err := r.EditDraft(a.ID, canonicalContent(c.AssessmentContent), actor, nowUTC())
	if err != nil {
		return err
	}
	record := updated.Record()
	for i := range record.Versions {
		if record.Versions[i].ID == a.ID {
			record.Versions[i].RubricVersionID = c.RubricID
		}
	}
	updated, err = report.Restore(record)
	if err != nil {
		return err
	}
	if err = saveCanonical(ctx, tx, updated, r.Revision()); err != nil {
		return err
	}
	content, err := encode(c.AssessmentContent)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO learning_assessments(id,report_id,report_version_id,student_id,rubric_id,author_id,title,period,content,status) VALUES($1::uuid,$2::uuid,$1::uuid,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8::jsonb,'draft')`, a.ID, r.ID(), a.StudentID, c.RubricID, actor, c.Title, a.Period, content)
	if err != nil {
		return err
	}
	return assessmentHistory(ctx, tx, a.ID, actor, "adoptLegacyDraft", content)
}
