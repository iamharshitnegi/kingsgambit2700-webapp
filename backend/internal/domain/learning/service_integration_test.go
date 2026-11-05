//go:build integration

package learning

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

type fixture struct {
	pool                                                         *pgxpool.Pool
	service                                                      *Service
	coach, student, guardian, academic, outsider, batch, profile string
}

func setup(t *testing.T) fixture {
	t.Helper()
	return setupWithIDs(t, platformid.New)
}

func setupWithIDs(t *testing.T, newID func() string) fixture {
	t.Helper()
	url := os.Getenv("KG_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KG_TEST_DATABASE_URL is not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, url)
	svc, err := NewService(pool)
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{pool: pool, service: svc, coach: newID(), student: newID(), guardian: newID(), academic: newID(), outsider: newID(), batch: newID(), profile: newID()}
	ctx := context.Background()
	for _, user := range []struct{ id, role string }{{f.coach, "coach"}, {f.student, "student"}, {f.guardian, "guardian"}, {f.academic, "academic_manager"}, {f.outsider, "coach"}} {
		if _, err = pool.Exec(ctx, `INSERT INTO users(id,display_name,status,created_at,updated_at) VALUES($1::uuid,$2,'active',now(),now())`, user.id, user.role); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,$3,now())`, platformid.New(), user.id, user.role); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO student_profiles(id,user_id,preferred_name,age_band,country,time_zone,locale,language,status,created_at,updated_at) VALUES($1::uuid,$2::uuid,'Learner','8_12','IN','Asia/Kolkata','en','en','active',now(),now())`, f.profile, f.student); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO batches(id,program_id,level_slug,coach_user_id,format,source_time_zone,target_capacity,hard_capacity,status,created_at,updated_at) SELECT $1::uuid,id,'foundation-1',$2::uuid,'group','Asia/Kolkata',4,6,'active',now(),now() FROM programs LIMIT 1`, f.batch, f.coach); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO enrolments(id,student_profile_id,batch_id,status,policy_snapshot,starts_at,created_at,updated_at) VALUES($1::uuid,$2::uuid,$3::uuid,'active','{}',now(),now(),now())`, platformid.New(), f.profile, f.batch); err != nil {
		t.Fatal(err)
	}
	family := platformid.New()
	if _, err = pool.Exec(ctx, `INSERT INTO families(id,display_name,created_at,updated_at) VALUES($1::uuid,'Family',now(),now())`, family); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO guardian_student_relationships(id,family_id,guardian_user_id,student_profile_id,permissions,status,created_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4::uuid,ARRAY['view_progress','communicate_with_academy'],'active',now())`, platformid.New(), family, f.guardian, f.profile); err != nil {
		t.Fatal(err)
	}
	return f
}
func mustAction(t *testing.T, f fixture, actor string, c Command) Workspace {
	t.Helper()
	w, err := f.service.Action(context.Background(), actor, c)
	if err != nil {
		t.Fatalf("%s: %v", c.Action, err)
	}
	return w
}
func TestAssignmentLifecycleAndHiddenSolutions(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := Command{Action: "createAssignment", StudentIDs: []string{f.profile}, Title: "Centre pawns", Description: "Find a good first move", Kind: "puzzle_set", Status: "assigned", Puzzles: []Puzzle{{Title: "Centre", FEN: initialFEN, Solutions: [][]string{{"e2e4"}, {"d2d4"}}}}}
	if _, err := f.service.Action(ctx, f.outsider, c); !errors.Is(err, ErrDenied) {
		t.Fatalf("unassigned coach create = %v", err)
	}
	w := mustAction(t, f, f.coach, c)
	if len(w.Assignments) != 1 {
		t.Fatalf("assignments: %+v", w.Assignments)
	}
	a := w.Assignments[0]
	learner, err := f.service.Workspace(ctx, f.student)
	if err != nil {
		t.Fatal(err)
	}
	if len(learner.Assignments[0].Puzzles[0].Solutions) != 0 {
		t.Fatal("hidden solutions leaked")
	}
	if _, err = f.service.Action(ctx, f.guardian, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version, Answer: "not the student"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("guardian submission = %v", err)
	}
	w = mustAction(t, f, f.student, Command{Action: "puzzleMove", ID: a.ID, Version: a.Version, Move: "e2e3"})
	a = w.Assignments[0]
	if a.Attempts[0].Metrics.Incorrect != 1 {
		t.Fatal("incorrect move not recorded")
	}
	if _, err = f.service.Action(ctx, f.student, Command{Action: "puzzleMove", ID: a.ID, Version: a.Version - 1, Move: "e2e4"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale mutation = %v", err)
	}
	w = mustAction(t, f, f.student, Command{Action: "puzzleMove", ID: a.ID, Version: a.Version, Move: "d2d4"})
	a = w.Assignments[0]
	if !a.Attempts[0].Puzzles[0].Solved {
		t.Fatal("alternative solution rejected")
	}
	w = mustAction(t, f, f.student, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version})
	a = w.Assignments[0]
	w = mustAction(t, f, f.coach, Command{Action: "returnAssignment", ID: a.ID, Version: a.Version, Feedback: "Explain your reasoning too"})
	a = w.Assignments[0]
	w = mustAction(t, f, f.student, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version, Answer: "I control the centre"})
	a = w.Assignments[0]
	if len(a.Attempts) != 2 {
		t.Fatal("attempt history not retained")
	}
	w = mustAction(t, f, f.coach, Command{Action: "reviewAssignment", ID: a.ID, Version: a.Version, Feedback: "Good explanation"})
	a = w.Assignments[0]
	if a.Attempts[1].Status != "reviewed" {
		t.Fatal("review not saved")
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,scope_type,scope_id,effect,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'learning.manage','student',$3::uuid,'deny','Test deny',$4::uuid,now())`, platformid.New(), f.coach, f.profile, f.academic); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Action(ctx, f.coach, Command{Action: "closeAssignment", ID: a.ID, Version: a.Version}); !errors.Is(err, ErrDenied) {
		t.Fatalf("explicit deny ignored: %v", err)
	}
}
func TestResourcesCommunicationAndAssessmentPublication(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := mustAction(t, f, f.coach, Command{Action: "createResource", Title: "Opening study", Kind: "pgn", Content: "[Event \"Study\"]\n\n1. e4 e5 *", Visibility: "shared"})
	resource := w.Resources[0]
	if _, err := f.service.Action(ctx, f.coach, Command{Action: "publishResource", ID: resource.ID, Version: resource.Version}); !errors.Is(err, ErrDenied) {
		t.Fatalf("coach published academy resource: %v", err)
	}
	mustAction(t, f, f.academic, Command{Action: "publishResource", ID: resource.ID, Version: resource.Version})
	w = mustAction(t, f, f.coach, Command{Action: "createThread", StudentID: f.profile, Title: "Practice conversation", Body: "Try the new exercise"})
	thread := w.Threads[0]
	guardian, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(guardian.Threads) != 1 || guardian.Threads[0].Unread != 1 {
		t.Fatal("guardian cannot see learner thread")
	}
	mustAction(t, f, f.guardian, Command{Action: "sendMessage", ID: thread.ID, Body: "We will practise this evening"})
	if _, err = f.service.Action(ctx, f.outsider, Command{Action: "sendMessage", ID: thread.ID, Body: "unauthorized"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("outsider message = %v", err)
	}
	w = mustAction(t, f, f.coach, Command{Action: "createThread", BatchID: f.batch, Title: "Batch announcement", Body: "Bring a game next class"})
	batchThread := w.Threads[0]
	if _, err = f.service.Action(ctx, f.student, Command{Action: "sendMessage", ID: batchThread.ID, Body: "broadcast"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("learner announcement = %v", err)
	}
	rubric := w.Rubrics[0]
	ratings := []Rating{}
	for _, d := range rubric.Dimensions {
		ratings = append(ratings, Rating{Dimension: d, Rating: "secure", Evidence: "Observed in class"})
	}
	content := AssessmentContent{Learned: "Checks", Strengths: "Piece safety", FocusNext: "Planning", PracticePlan: "Two puzzles", CoachComment: "Steady progress", Ratings: ratings, Eligible: true}
	w = mustAction(t, f, f.coach, Command{Action: "createAssessment", StudentID: f.profile, RubricID: rubric.ID, Title: "Monthly progress", Period: "2026-09", AssessmentContent: content})
	a := w.Assessments[0]
	guardian, err = f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(guardian.Assessments) != 0 {
		t.Fatal("draft assessment leaked")
	}
	w = mustAction(t, f, f.coach, Command{Action: "submitAssessment", ID: a.ID, Version: a.Version})
	a = w.Assessments[0]
	if _, err = f.service.Action(ctx, f.coach, Command{Action: "publishAssessment", ID: a.ID, Version: a.Version}); !errors.Is(err, ErrDenied) {
		t.Fatalf("coach self-published: %v", err)
	}
	w = mustAction(t, f, f.academic, Command{Action: "publishAssessment", ID: a.ID, Version: a.Version})
	a = w.Assessments[0]
	if _, err = f.service.Action(ctx, f.academic, Command{Action: "editAssessment", ID: a.ID, Version: a.Version, AssessmentContent: content}); !errors.Is(err, ErrConflict) {
		t.Fatalf("published assessment edit = %v", err)
	}
	w = mustAction(t, f, f.guardian, Command{Action: "acknowledgeAssessment", ID: a.ID, Version: a.Version})
	a = w.Assessments[0]
	w = mustAction(t, f, f.academic, Command{Action: "issueCertificate", ID: a.ID, Version: a.Version})
	if len(w.Certificates) != 1 {
		t.Fatal("certificate not issued")
	}
	if _, err = f.pool.Exec(ctx, `UPDATE batches SET level_slug='beginner' WHERE id=$1::uuid`, f.batch); err != nil {
		t.Fatal(err)
	}
	placement := Command{Action: "placeStudent", StudentID: f.profile, LevelSlug: "foundation-2", AssessmentID: a.ID, Feedback: "Published work supports the next level"}
	if _, err = f.service.Action(ctx, f.coach, placement); !errors.Is(err, ErrDenied) {
		t.Fatalf("coach placed learner: %v", err)
	}
	w = mustAction(t, f, f.academic, placement)
	if len(w.Placements) != 1 || w.Placements[0].PreviousLevelSlug != "beginner" || w.Placements[0].CurriculumVersion != "academy-framework-v2" || w.Students[0].LevelSlug != "foundation-2" {
		t.Fatal("placement did not preserve versioned legacy history")
	}
	var legacyLevel string
	if err = f.pool.QueryRow(ctx, `SELECT level_slug FROM batches WHERE id=$1::uuid`, f.batch).Scan(&legacyLevel); err != nil {
		t.Fatal(err)
	}
	if legacyLevel != "beginner" {
		t.Fatal("placement rewrote legacy batch")
	}
	placement.LevelSlug = "development-1"
	placement.AssessmentID = platformid.New()
	if _, err = f.service.Action(ctx, f.academic, placement); err == nil {
		t.Fatal("placement accepted unrelated report evidence")
	}

	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET status='revoked' WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	guardian, err = f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(guardian.Threads) != 0 || len(guardian.Assessments) != 0 || len(guardian.Notifications) != 0 {
		t.Fatal("revoked guardian retains learner data")
	}
}

func TestExistingReportCorrectionsPreservePublishedHistory(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w, err := f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	rubric := w.Rubrics[0]
	content := AssessmentContent{Learned: "Checks", Strengths: "Piece safety", FocusNext: "Planning", PracticePlan: "Two puzzles", CoachComment: "Steady progress"}
	for _, dimension := range rubric.Dimensions {
		content.Ratings = append(content.Ratings, Rating{Dimension: dimension, Rating: "secure", Evidence: "Observed in class"})
	}
	w = mustAction(t, f, f.coach, Command{Action: "createAssessment", StudentID: f.profile, RubricID: rubric.ID, Title: "Existing monthly progress", Period: "2026-08", AssessmentContent: content})
	a := w.Assessments[0]
	w = mustAction(t, f, f.coach, Command{Action: "submitAssessment", ID: a.ID, Version: a.Version})
	a = w.Assessments[0]
	w = mustAction(t, f, f.academic, Command{Action: "publishAssessment", ID: a.ID, Version: a.Version})
	a = w.Assessments[0]
	// Removing the optional rubric sidecar simulates a report published before the workspace existed.
	if _, err = f.pool.Exec(ctx, `DELETE FROM learning_assessment_history WHERE assessment_id=$1::uuid`, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.pool.Exec(ctx, `DELETE FROM learning_assessments WHERE id=$1::uuid`, a.ID); err != nil {
		t.Fatal(err)
	}
	w, err = f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Assessments) != 1 || w.Assessments[0].Content.Learned != "Checks" {
		t.Fatal("legacy published report missing")
	}
	a = w.Assessments[0]
	w = mustAction(t, f, f.guardian, Command{Action: "acknowledgeAssessment", ID: a.ID, Version: a.Version})
	a = w.Assessments[0]
	content.Learned = "Checks and forcing moves"
	w = mustAction(t, f, f.coach, Command{Action: "correctAssessment", ID: a.ID, Version: a.Version, RubricID: rubric.ID, Title: "Corrected monthly progress", AssessmentContent: content})
	if len(w.Assessments) != 2 {
		t.Fatal("correction did not preserve original")
	}
	var correction Assessment
	for _, item := range w.Assessments {
		if item.SourceVersionID == a.ID {
			correction = item
		}
	}
	if correction.ID == "" || correction.ReportID != a.ReportID || correction.Status != "draft" {
		t.Fatal("correction is disconnected from report")
	}
	family, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(family.Assessments) != 1 || family.Assessments[0].AcknowledgedAt == nil || family.Assessments[0].Content.Learned != "Checks" {
		t.Fatal("correction changed published content/receipt or leaked draft")
	}
	w = mustAction(t, f, f.coach, Command{Action: "submitAssessment", ID: correction.ID, Version: correction.Version})
	for _, item := range w.Assessments {
		if item.ID == correction.ID {
			correction = item
		}
	}
	mustAction(t, f, f.academic, Command{Action: "publishAssessment", ID: correction.ID, Version: correction.Version})
	family, err = f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(family.Assessments) != 2 {
		t.Fatal("published correction history missing")
	}
}

func TestHomeworkOutboxCommitsWithAssignmentAndSchedulesDueReminder(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	due := nowUTC().Add(48 * time.Hour)
	c := Command{Action: "createAssignment", StudentIDs: []string{f.profile, platformid.New()}, Title: "Private lesson title", Description: "Explain the idea", Kind: "study", Status: "assigned", DueAt: &due}
	if _, err := f.service.Action(ctx, f.coach, c); !errors.Is(err, ErrDenied) {
		t.Fatalf("unauthorized roster: %v", err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rolled-back assignment sent email work")
	}
	c.StudentIDs = []string{f.profile}
	w := mustAction(t, f, f.coach, c)
	if len(w.Assignments) != 1 {
		t.Fatal("assignment missing")
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE render_context->>'assignmentId'=$1 AND scheduled_at>now() AND render_context->>'title'='Academy homework is due soon'`, w.Assignments[0].ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected student and guardian reminder, got %d", count)
	}
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE render_context::text LIKE '%Private lesson title%'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("private lesson text leaked into email summaries")
	}
}

func TestNotificationPermissionsRemainCategorySpecific(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	if _, err := f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET permissions=ARRAY['view_schedule'] WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	for _, section := range []string{"calendar", "homework", "messages"} {
		if _, err := f.pool.Exec(ctx, `INSERT INTO learning_notifications(id,recipient_id,student_id,title,href) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5)`, platformid.New(), f.guardian, f.profile, section, "/portal/"+section); err != nil {
			t.Fatal(err)
		}
	}
	w, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Notifications) != 1 || w.Notifications[0].Title != "calendar" || len(w.Students) != 0 {
		t.Fatal("schedule-only guardian notification scope incorrect")
	}
	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET permissions=ARRAY['communicate_with_academy'] WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	w, err = f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Notifications) != 1 || w.Notifications[0].Title != "messages" || len(w.Students) != 1 || !w.Students[0].CanCommunicate || w.Students[0].LevelSlug != "" {
		t.Fatal("communication-only guardian scope incorrect")
	}
}
