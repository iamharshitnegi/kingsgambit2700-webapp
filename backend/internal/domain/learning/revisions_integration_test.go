//go:build integration

package learning

import (
	"context"
	"testing"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestLegacyDraftAdoptsCurrentRubricWithoutReplacingReport(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w, err := f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	rubric := w.Rubrics[0]
	content := AssessmentContent{Learned: "Central squares", Strengths: "Legal play", FocusNext: "Checks", PracticePlan: "Daily practice", CoachComment: "Measured progress"}
	for _, d := range rubric.Dimensions {
		content.Ratings = append(content.Ratings, Rating{Dimension: d, Rating: "secure", Evidence: "Observed in class"})
	}
	w = mustAction(t, f, f.coach, Command{Action: "createAssessment", StudentID: f.profile, RubricID: rubric.ID, Title: "Existing draft", Period: "2026-09", AssessmentContent: content})
	a := w.Assessments[0]
	for _, q := range []string{`DELETE FROM learning_assessment_history WHERE assessment_id=$1::uuid`, `DELETE FROM learning_assessments WHERE id=$1::uuid`} {
		if _, err = f.pool.Exec(ctx, q, a.ID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = f.pool.Exec(ctx, `UPDATE progress_report_versions SET content=jsonb_set(content, '{rubricVersionId}', to_jsonb($2::text)) WHERE id=$1::uuid`, a.ID, platformid.New()); err != nil {
		t.Fatal(err)
	}
	w, err = f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	a = w.Assessments[0]
	content.Learned = "Central squares and safe development"
	w = mustAction(t, f, f.coach, Command{Action: "editAssessment", ID: a.ID, Version: a.Version, RubricID: rubric.ID, Title: "Adopted draft", Period: a.Period, AssessmentContent: content})
	adopted := w.Assessments[0]
	if adopted.ID != a.ID || adopted.ReportID != a.ReportID || adopted.Content.Learned != content.Learned || adopted.RubricID != rubric.ID || len(adopted.History) != 1 {
		t.Fatalf("adoption replaced draft or lost evidence: %+v", adopted)
	}
	w = mustAction(t, f, f.coach, Command{Action: "submitAssessment", ID: a.ID, Version: adopted.Version})
	w = mustAction(t, f, f.academic, Command{Action: "publishAssessment", ID: a.ID, Version: w.Assessments[0].Version})
	if w.Assessments[0].Status != "published" {
		t.Fatal("legacy draft could not complete publication")
	}
}

func TestWorkspacePaginationAdvertisesAndReturnsFurtherRecords(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for _, title := range []string{"First study", "Second study", "Third study"} {
		mustAction(t, f, f.coach, Command{Action: "createResource", Title: title, Kind: "study", Content: "Study central squares", Visibility: "personal"})
		mustAction(t, f, f.coach, Command{Action: "createAssignment", StudentID: f.profile, Title: title, Kind: "study", Description: "Explain central squares", Status: "assigned"})
	}
	small, err := f.service.Workspace(WithWorkspaceLimit(ctx, 1), f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(small.Assignments) != 1 || len(small.Resources) != 1 || !small.Pagination.HasMore || small.Pagination.NextLimit != 501 {
		t.Fatalf("unadvertised truncation: %+v", small.Pagination)
	}
	next, err := f.service.Workspace(WithWorkspaceLimit(ctx, small.Pagination.NextLimit), f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Assignments) != 3 || len(next.Resources) != 3 || next.Pagination.HasMore {
		t.Fatalf("load more lost records: %+v", next.Pagination)
	}
}

func TestPublishedResourceRevisionPreservesPublishedSnapshot(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := mustAction(t, f, f.coach, Command{Action: "createResource", Title: "Original study", Kind: "study", Content: "Original explanation", Visibility: "personal", Status: "draft"})
	r := w.Resources[0]
	w = mustAction(t, f, f.coach, Command{Action: "publishResource", ID: r.ID, Version: r.Version})
	r = w.Resources[0]
	w = mustAction(t, f, f.coach, Command{Action: "createAssignment", StudentID: f.profile, Title: "Read original", Description: "Explain", Kind: "study", Status: "assigned", ResourceIDs: []string{r.ID}})
	assigned := w.Assignments[0]
	w = mustAction(t, f, f.coach, Command{Action: "reviseResource", ID: r.ID, Version: r.Version})
	if w.Resources[0].Status != "draft" || w.Resources[0].Version != r.Version+1 {
		t.Fatal("revision not drafted")
	}
	w = mustAction(t, f, f.coach, Command{Action: "editResource", ID: r.ID, Version: w.Resources[0].Version, Title: "Revised study", Kind: "study", Content: "Better explanation"})
	mustAction(t, f, f.coach, Command{Action: "publishResource", ID: r.ID, Version: w.Resources[0].Version})
	family, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if family.Assignments[0].Resources[0].Content != assigned.Resources[0].Content || family.Assignments[0].Resources[0].Version != r.Version {
		t.Fatal("revision changed assigned snapshot")
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM teaching_resource_versions WHERE resource_id=$1::uuid`, r.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("published snapshots: %d", count)
	}
}

func TestWorkspaceAuthorizationDoesNotDependOnVisibleStudentPage(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	user, profile := platformid.New(), platformid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO users(id,display_name,status,created_at,updated_at) VALUES($1::uuid,'Zulu learner','active',now(),now())`, user); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO student_profiles(id,user_id,preferred_name,age_band,country,time_zone,locale,language,status,created_at,updated_at) VALUES($1::uuid,$2::uuid,'Zulu learner','8_12','IN','Asia/Kolkata','en','en','active',now(),now())`, profile, user); err != nil {
		t.Fatal(err)
	}
	mustAction(t, f, f.academic, Command{Action: "createAssignment", StudentID: profile, Title: "Beyond the learner page", Kind: "study", Description: "Read this study", Status: "assigned"})
	w, err := f.service.Workspace(WithWorkspaceLimit(ctx, 1), f.academic)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Students) != 1 || w.Students[0].ID == profile {
		t.Fatal("fixture did not put the target learner beyond visible page")
	}
	if len(w.Assignments) != 1 || w.Assignments[0].StudentID != profile {
		t.Fatal("student page limit incorrectly restricted authorized records")
	}
	if !w.Pagination.HasMore {
		t.Fatal("additional learner not advertised")
	}
}
