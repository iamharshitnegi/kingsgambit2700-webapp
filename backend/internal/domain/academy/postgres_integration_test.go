//go:build integration

package academy

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/authorization"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	"github.com/kingsgambit2700/platform/backend/internal/domain/family"
	leadmodel "github.com/kingsgambit2700/platform/backend/internal/lead/model"
	leadrepository "github.com/kingsgambit2700/platform/backend/internal/lead/repository"
	leadservice "github.com/kingsgambit2700/platform/backend/internal/lead/service"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
	trialmodel "github.com/kingsgambit2700/platform/backend/internal/trial/model"
	trialrepository "github.com/kingsgambit2700/platform/backend/internal/trial/repository"
	trialservice "github.com/kingsgambit2700/platform/backend/internal/trial/service"
)

type academyFixture struct {
	pool                  *pgxpool.Pool
	service               *Service
	root, guardian, coach string
	student               Student
	batch                 Batch
	enrolment             Enrolment
	now                   time.Time
}

func newAcademyFixture(t *testing.T) academyFixture {
	t.Helper()
	return newAcademyFixtureWithStaffIDs(t, "", "")
}

func newAcademyFixtureWithStaffIDs(t *testing.T, rootID, coachID string) academyFixture {
	t.Helper()
	database := os.Getenv("KG_TEST_DATABASE_URL")
	if database == "" {
		t.Skip("KG_TEST_DATABASE_URL not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, database)
	root := rootID
	if root == "" {
		root = platformid.New()
	}
	now := time.Now().UTC().Add(-time.Second).Truncate(time.Microsecond)
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,display_name,status,created_at,updated_at) VALUES($1::uuid,'Academy owner','active',$2,$2)`, root, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,'super_admin',$3)`, platformid.New(), root, now); err != nil {
		t.Fatal(err)
	}
	repo, err := NewPostgresRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	service.now = func() time.Time { return now }
	var coach Coach
	if coachID == "" {
		coach, err = service.CreateStaff(t.Context(), root, StaffInput{Name: "Coach Maya", Email: "coach@example.test", Role: "coach"})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		coach.ID = coachID
		if _, err = pool.Exec(t.Context(), `INSERT INTO users(id,email,display_name,status,created_at,updated_at) VALUES($1::uuid,'coach@example.test','Coach Maya','active',$2,$2)`, coach.ID, now); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(t.Context(), `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,'coach',$3)`, platformid.New(), coach.ID, now); err != nil {
			t.Fatal(err)
		}
	}
	student, err := service.CreateStudent(t.Context(), root, StudentInput{Name: "Student One", AgeBand: "8_12", Country: "IN", TimeZone: "Asia/Kolkata", GuardianEmail: "guardian@example.test", GuardianName: "Guardian One"})
	if err != nil {
		t.Fatal(err)
	}
	var guardian string
	if err = pool.QueryRow(t.Context(), `SELECT id::text FROM users WHERE email='guardian@example.test'`).Scan(&guardian); err != nil {
		t.Fatal(err)
	}
	batch, err := service.CreateBatch(t.Context(), root, BatchInput{Name: "Foundation group", CoachID: coach.ID, Format: "group", Level: "beginner", TimeZone: "Asia/Kolkata", Capacity: 6})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.CreateEnrolment(t.Context(), root, EnrolmentInput{StudentID: student.ID, BatchID: batch.ID, StartsAt: now}); !errors.Is(err, ErrConsent) {
		t.Fatalf("missing consent error=%v", err)
	}
	if _, err = service.SaveSettings(t.Context(), root, Settings{Name: "Academy", LegalName: "Academy Example", Email: "help@example.test"}); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, slug := range []string{"privacy", "child-safety"} {
		policy, err := service.PublishPolicy(t.Context(), root, PolicyInput{Slug: slug, Title: slug, Version: "1", Content: "These are the academy policy terms for this integration test."})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, policy.ID)
	}
	if err = service.AcceptConsent(t.Context(), root, student.ID, ConsentInput{PolicyVersionIDs: ids}); !errors.Is(err, ErrDenied) {
		t.Fatalf("staff proxy consent error=%v", err)
	}
	if err = service.AcceptConsent(t.Context(), guardian, student.ID, ConsentInput{PolicyVersionIDs: ids}); err != nil {
		t.Fatal(err)
	}
	enrolment, err := service.CreateEnrolment(t.Context(), root, EnrolmentInput{StudentID: student.ID, BatchID: batch.ID, StartsAt: now})
	if err != nil {
		t.Fatal(err)
	}
	ledgerRepo, err := entitlement.NewPostgresRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	authorizer, err := authorization.NewPostgres(pool)
	if err != nil {
		t.Fatal(err)
	}
	ledgerService, err := entitlement.NewService(ledgerRepo, entitlement.WithAuthorizer(authorizer))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = ledgerService.GrantForActor(t.Context(), entitlement.GrantCommand{EntryID: platformid.New(), AccountID: enrolment.AccountID, IdempotencyKey: "opening", ReferenceID: "test", Units: 10, Reason: "authorized fixture credits", ActorID: root, OccurredAt: now}); err != nil {
		t.Fatal(err)
	}
	return academyFixture{pool: pool, service: service, root: root, guardian: guardian, coach: coach.ID, student: student, batch: batch, enrolment: enrolment, now: now}
}
func (f academyFixture) classInput() ClassInput {
	return ClassInput{BatchID: f.batch.ID, Title: "Tactical vision", LocalStart: f.now.Add(48 * time.Hour).UTC().Format("2006-01-02T15:04"), TimeZone: "UTC", DurationMinutes: 60, Occurrences: 1, IdempotencyKey: platformid.New()}
}
func TestPostgresAcademy_CompleteJourney(t *testing.T) {
	testCompleteJourney(t, newAcademyFixture(t))
}
func TestPostgresAcademy_LegacyStaffCompleteJourney(t *testing.T) {
	testCompleteJourney(t, newAcademyFixtureWithStaffIDs(t, "6a363478-47b5-4e51-8cdf-b55c22cddca8", "b47c153f-1877-4992-8d41-d8b46cbd0f7b"))
}
func testCompleteJourney(t *testing.T, f academyFixture) {
	t.Helper()
	ctx := t.Context()
	code, err := f.service.IssueCode(ctx, f.guardian, f.student.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(code.Code) < 12 || !code.ExpiresAt.After(f.now) {
		t.Fatal("missing secure student code")
	}
	// Provisioning must remain compatible with the existing relationship service.
	var relationshipID string
	if err = f.pool.QueryRow(ctx, `SELECT id::text FROM guardian_student_relationships WHERE student_profile_id=$1::uuid`, f.student.ID).Scan(&relationshipID); err != nil {
		t.Fatal(err)
	}
	familyRepo, err := family.NewPostgresRepository(f.pool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = familyRepo.Relationship(ctx, relationshipID); err != nil {
		t.Fatal(err)
	}
	in := f.classInput()
	in.Occurrences = 2
	preview, err := f.service.Schedule(ctx, f.coach, in, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Classes) != 2 {
		t.Fatal("preview missing occurrences")
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM class_sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("preview mutated classes")
	}
	scheduled, err := f.service.Schedule(ctx, f.coach, in, false)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.service.Schedule(ctx, f.coach, in, false)
	if err != nil {
		t.Fatal(err)
	}
	if again.Classes[0].ID != scheduled.Classes[0].ID {
		t.Fatal("retry duplicated classes")
	}
	in.Title = "Changed payload"
	if _, err = f.service.Schedule(ctx, f.coach, in, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("idempotency mismatch=%v", err)
	}
	lesson := scheduled.Classes[0]
	f.service.now = func() time.Time { return lesson.StartsAt.Add(time.Hour) }
	completed, err := f.service.Complete(ctx, f.coach, lesson.ID, CompletionInput{Revision: 0, Notes: "Practised forcing moves and found several forks.", Attendance: map[string]string{f.student.ID: "on_time"}, IdempotencyKey: "complete"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != "completed" {
		t.Fatal(completed.Status)
	}
	workspace, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Classes) != 2 || len(workspace.Enrolments) != 1 {
		t.Fatalf("workspace=%+v", workspace)
	}
	if workspace.Enrolments[0].Available != 8 || workspace.Enrolments[0].Reserved != 1 || workspace.Enrolments[0].Consumed != 1 {
		t.Fatalf("balance=%+v", workspace.Enrolments[0])
	}
	var boards int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM classroom_states`).Scan(&boards); err != nil {
		t.Fatal(err)
	}
	if boards != 2 {
		t.Fatal("new classes lack boards")
	}
}
func TestPostgresAcademy_ScopeConflictAndRollback(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	in := f.classInput()
	in.Occurrences = 11
	if _, err := f.service.Schedule(ctx, f.coach, in, false); !errors.Is(err, ErrCredits) {
		t.Fatalf("unfunded schedule error=%v", err)
	}
	var count int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM class_sessions`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("partial unfunded series saved")
	}
	in.Occurrences = 1
	if _, err := f.service.Schedule(ctx, f.guardian, in, false); !errors.Is(err, ErrDenied) {
		t.Fatalf("guardian schedule error=%v", err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,scope_type,scope_id,effect,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'schedule.manage','student',$3::uuid,'deny','test scope denial',$4::uuid,$5)`, platformid.New(), f.coach, f.student.ID, f.root, f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Schedule(ctx, f.coach, in, false); !errors.Is(err, ErrDenied) {
		t.Fatalf("explicit deny ignored: %v", err)
	}
	const attempts = 6
	results := make(chan error, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			input := in
			input.IdempotencyKey = platformid.New()
			_, err := f.service.Schedule(context.Background(), f.root, input, false)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatalf("unexpected concurrent error=%v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent schedules=%d", successes)
	}
}
func TestPostgresAcademy_GroupAbsenceAndAcademyCancellation(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	in := f.classInput()
	in.Occurrences = 2
	scheduled, err := f.service.Schedule(ctx, f.root, in, false)
	if err != nil {
		t.Fatal(err)
	}
	first := scheduled.Classes[0]
	preview, err := f.service.Cancel(ctx, f.guardian, first.ID, CancellationInput{Revision: 0, StudentID: f.student.ID, Reason: "Family unavailable"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if preview.Outcomes[0].Action != "consume" || preview.Outcomes[0].MakeUp {
		t.Fatal(preview)
	}
	if _, err = f.service.Cancel(ctx, f.guardian, first.ID, CancellationInput{Revision: 0, StudentID: f.student.ID, Reason: "Family unavailable", IdempotencyKey: "absence"}, false); err != nil {
		t.Fatal(err)
	}
	var status string
	if err = f.pool.QueryRow(ctx, `SELECT status FROM class_sessions WHERE id=$1::uuid`, first.ID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "scheduled" {
		t.Fatal("family absence cancelled group")
	}
	second := scheduled.Classes[1]
	out, err := f.service.Cancel(ctx, f.root, second.ID, CancellationInput{Revision: 0, Reason: "Academy cannot deliver", IdempotencyKey: "academy-cancel"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !out.Outcomes[0].MakeUp || out.Outcomes[0].Action != "restore" {
		t.Fatal(out)
	}
	var obligations int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM academy_make_up_obligations`).Scan(&obligations); err != nil {
		t.Fatal(err)
	}
	if obligations != 1 {
		t.Fatal("make-up missing")
	}
	workspace, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Enrolments[0].Available != 9 || workspace.Enrolments[0].Reserved != 0 || workspace.Enrolments[0].Consumed != 1 {
		t.Fatalf("balance=%+v", workspace.Enrolments[0])
	}
}

func TestPostgresAcademy_ReschedulingEditsAndAvailability(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	in := f.classInput()
	in.Occurrences = 3
	start, err := time.Parse("2006-01-02T15:04", in.LocalStart)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := f.service.SaveAvailability(ctx, f.coach, Availability{CoachID: f.coach, StartsAt: start.Add(-time.Minute), EndsAt: start.Add(2 * time.Hour), TimeZone: "UTC", Status: "blocked"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Schedule(ctx, f.coach, in, false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("blocked coach schedule=%v", err)
	}
	if err = f.service.DeleteAvailability(ctx, f.coach, blocked.ID); err != nil {
		t.Fatal(err)
	}
	lessons, err := f.service.Schedule(ctx, f.coach, in, false)
	if err != nil {
		t.Fatal(err)
	}
	edit := in
	edit.Occurrences = 1
	edit.Scope = "future"
	edit.LocalStart = start.Add(24 * time.Hour).Format("2006-01-02T15:04")
	edit.Title = "Updated tactical vision"
	if _, err = f.service.UpdateClass(ctx, f.coach, lessons.Classes[0].ID, edit); err != nil {
		t.Fatal(err)
	}
	workspace, err := f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	for _, lesson := range workspace.Classes {
		if lesson.Revision != 1 || lesson.Title != edit.Title {
			t.Fatalf("future edit missed occurrence: %+v", lesson)
		}
	}
	if _, err = f.service.UpdateClass(ctx, f.coach, lessons.Classes[0].ID, edit); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit=%v", err)
	}
	if _, err = f.service.UpdateEnrolment(ctx, f.root, f.enrolment.ID, EnrolmentUpdate{Status: "paused"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("paused reserved enrolment=%v", err)
	}
	updated, err := f.service.UpdateStudent(ctx, f.guardian, f.student.ID, StudentUpdate{Name: "Student Preferred", AgeBand: "8_12", TimeZone: "Asia/Kolkata", Country: "IN", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Student Preferred" {
		t.Fatal(updated)
	}
}

func TestPostgresAcademy_ProfilePrivacyAndOperations(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	if _, err := f.service.UpdateProfile(ctx, f.guardian, ProfileInput{DisplayName: "Parent preferred", TimeZone: "Asia/Kolkata"}); err != nil {
		t.Fatal(err)
	}
	views := []SavedView{{Name: "This week", Section: "schedule", Query: "tactics"}}
	if _, err := f.service.SavePreferences(ctx, f.guardian, Preferences{TimeZone: "Europe/London", SavedViews: views}); err != nil {
		t.Fatal(err)
	}
	mine, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if mine.Profile.DisplayName != "Parent preferred" || mine.Profile.TimeZone != "Europe/London" || len(mine.Profile.SavedViews) != 1 {
		t.Fatal(mine.Profile)
	}
	other, err := f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Profile.SavedViews) != 0 {
		t.Fatal("preferences leaked between users")
	}
	if _, err = f.service.Operations(ctx, f.guardian); !errors.Is(err, ErrDenied) {
		t.Fatalf("guardian operations=%v", err)
	}
	leadSvc := leadservice.New(leadrepository.NewPostgres(f.pool))
	lead, _, err := leadSvc.Create(ctx, leadmodel.CreateInput{GuardianName: "Guardian One", Email: "guardian@example.test", StudentAgeBand: "8_12", Country: "IN", TimeZone: "UTC", ContactConsent: true})
	if err != nil {
		t.Fatal(err)
	}
	slot, err := f.service.CreateTrialSlot(ctx, f.root, TrialSlot{CoachID: f.coach, StartsAt: f.now.Add(24 * time.Hour), EndsAt: f.now.Add(25 * time.Hour), TimeZone: "UTC", Format: "one_to_one", Capacity: 1, AgeBands: []string{"8_12"}})
	if err != nil {
		t.Fatal(err)
	}
	trialSvc := trialservice.New(trialrepository.NewPostgres(f.pool))
	booking, err := trialSvc.Book(ctx, trialmodel.BookingInput{SlotID: slot.ID, LeadID: lead.ID, GuardianName: "Guardian One", Email: "guardian@example.test", StudentName: f.student.Name, StudentAgeBand: "8_12", TimeZone: "UTC", ContactConsent: true, IdempotencyKey: platformid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.UpdateTrial(ctx, f.root, booking.ID, TrialUpdate{Status: "attended", Notes: "Ready for a batch"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("premature attendance=%v", err)
	}
	f.service.now = func() time.Time { return slot.EndsAt.Add(time.Minute) }
	if _, err = f.service.UpdateTrial(ctx, f.root, booking.ID, TrialUpdate{Status: "attended", Notes: "Ready for a batch"}); err != nil {
		t.Fatal(err)
	}
	converted, err := f.service.ConvertLead(ctx, f.root, lead.ID, ConversionInput{StudentID: f.student.ID})
	if err != nil {
		t.Fatal(err)
	}
	if converted.Status != "won" || converted.StudentID != f.student.ID {
		t.Fatal(converted)
	}
	operations, err := f.service.Operations(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations.TrialBookings) != 1 || operations.TrialBookings[0].Notes != "Ready for a batch" {
		t.Fatal(operations)
	}
}

func TestPostgresAcademy_TrialRescheduleCapacityRetryAndNotifications(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	leadSvc := leadservice.New(leadrepository.NewPostgres(f.pool))
	lead, _, err := leadSvc.Create(ctx, leadmodel.CreateInput{GuardianName: "Guardian One", Email: "guardian@example.test", StudentAgeBand: "8_12", Country: "IN", TimeZone: "UTC", ContactConsent: true})
	if err != nil {
		t.Fatal(err)
	}
	slots := []TrialSlot{}
	for _, hours := range []int{24, 48, 72} {
		slot, err := f.service.CreateTrialSlot(ctx, f.root, TrialSlot{CoachID: f.coach, StartsAt: f.now.Add(time.Duration(hours) * time.Hour), EndsAt: f.now.Add(time.Duration(hours+1) * time.Hour), TimeZone: "UTC", Format: "one_to_one", Capacity: 1, AgeBands: []string{"8_12"}})
		if err != nil {
			t.Fatal(err)
		}
		slots = append(slots, slot)
	}
	trialSvc := trialservice.New(trialrepository.NewPostgres(f.pool))
	book := func(slot string) trialmodel.Booking {
		t.Helper()
		booking, err := trialSvc.Book(ctx, trialmodel.BookingInput{SlotID: slot, LeadID: lead.ID, GuardianName: "Guardian One", Email: "guardian@example.test", StudentName: f.student.Name, StudentAgeBand: "8_12", TimeZone: "UTC", ContactConsent: true, IdempotencyKey: platformid.New()})
		if err != nil {
			t.Fatal(err)
		}
		return booking
	}
	booking := book(slots[0].ID)
	_ = book(slots[2].ID)
	input := TrialReschedule{SlotID: slots[2].ID, Reason: "Family requested a different time", IdempotencyKey: "trial-change"}
	if _, err = f.service.RescheduleTrial(ctx, f.root, booking.ID, input); !errors.Is(err, ErrConflict) {
		t.Fatalf("full trial slot accepted: %v", err)
	}
	var original string
	if err = f.pool.QueryRow(ctx, `SELECT slot_id::text FROM trial_bookings WHERE id=$1::uuid`, booking.ID).Scan(&original); err != nil {
		t.Fatal(err)
	}
	if original != slots[0].ID {
		t.Fatal("capacity failure partially moved trial")
	}
	input.SlotID = slots[1].ID
	if _, err = f.service.RescheduleTrial(ctx, f.guardian, booking.ID, input); !errors.Is(err, ErrDenied) {
		t.Fatalf("guardian reschedule allowed: %v", err)
	}
	updated, err := f.service.RescheduleTrial(ctx, f.root, booking.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SlotID != slots[1].ID {
		t.Fatal(updated)
	}
	again, err := f.service.RescheduleTrial(ctx, f.root, booking.ID, input)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != booking.ID {
		t.Fatal("retry duplicated trial")
	}
	operations, err := f.service.Operations(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	for _, slot := range operations.TrialSlots {
		if slot.ID == slots[0].ID && slot.AvailableSeats != 1 {
			t.Fatal("old slot capacity not released")
		}
		if slot.ID == slots[1].ID && slot.AvailableSeats != 0 {
			t.Fatal("new slot capacity not reserved")
		}
	}
	var notifications int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications WHERE deduplication_key LIKE $1`, "trial-rescheduled:"+booking.ID+":%").Scan(&notifications); err != nil {
		t.Fatal(err)
	}
	if notifications != 2 {
		t.Fatalf("reschedule notifications=%d", notifications)
	}
}

func TestPostgresAcademy_FamilyScheduleScopeAndCancellationReversal(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	classes, err := f.service.Schedule(ctx, f.coach, f.classInput(), false)
	if err != nil {
		t.Fatal(err)
	}
	lesson := classes.Classes[0]
	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET permissions=ARRAY['view_profile'] WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	workspace, err := f.service.Workspace(ctx, f.guardian)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Students) != 1 || len(workspace.Classes) != 0 {
		t.Fatal("profile-only guardian obtained class schedule")
	}
	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET permissions=ARRAY['view_profile','view_schedule','manage_schedule'] WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Cancel(ctx, f.guardian, lesson.ID, CancellationInput{StudentID: f.student.ID, Revision: 0, Reason: "Family absence", IdempotencyKey: "family"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Cancel(ctx, f.root, lesson.ID, CancellationInput{Revision: 1, Reason: "Academy cannot provide class", IdempotencyKey: "academy"}, false); err != nil {
		t.Fatal(err)
	}
	workspace, err = f.service.Workspace(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	if workspace.Enrolments[0].Available != 10 || workspace.Enrolments[0].Consumed != 0 {
		t.Fatal("academy cancellation did not reverse prior family consumption")
	}
	if _, err = f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,scope_type,scope_id,effect,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'schedule.manage','family',$3::uuid,'deny','No family scheduling',$4::uuid,now())`, platformid.New(), f.coach, f.student.FamilyID, f.root); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Schedule(ctx, f.coach, f.classInput(), false); !errors.Is(err, ErrDenied) {
		t.Fatalf("family deny ignored: %v", err)
	}
}

func TestPostgresAcademy_CreationRequestReplay(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := idempotency.WithKey(t.Context(), "new-learner-request")
	input := StudentInput{Name: "New Learner", AgeBand: "8_12", Country: "IN", TimeZone: "UTC", GuardianEmail: "guardian@example.test", GuardianName: "Parent"}
	created, err := f.service.CreateStudent(ctx, f.root, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.service.CreateStudent(ctx, f.root, input)
	if err != nil {
		t.Fatal(err)
	}
	if again.ID != created.ID {
		t.Fatal("request retry duplicated learner")
	}
	input.Name = "Changed learner"
	if _, err = f.service.CreateStudent(ctx, f.root, input); !errors.Is(err, ErrConflict) {
		t.Fatalf("key payload mismatch accepted: %v", err)
	}
	availability := Availability{CoachID: f.coach, StartsAt: f.now.Add(100 * time.Hour), EndsAt: f.now.Add(101 * time.Hour), TimeZone: "UTC", Status: "available"}
	ctx = idempotency.WithKey(t.Context(), "availability-request")
	first, err := f.service.SaveAvailability(ctx, f.coach, availability)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.service.SaveAvailability(ctx, f.coach, availability)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("generated IDs defeated request replay")
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM student_profiles`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("learners=%d", count)
	}
}

func TestPostgresAcademy_MakeUpFulfillmentAndWorkspacePagination(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	input := f.classInput()
	input.Occurrences = 3
	result, err := f.service.Schedule(ctx, f.coach, input, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Cancel(ctx, f.root, result.Classes[0].ID, CancellationInput{Revision: 0, Reason: "Academy cancelled", IdempotencyKey: "cancel-owed"}, false); err != nil {
		t.Fatal(err)
	}
	workspace, err := f.service.Workspace(WithWorkspaceLimit(ctx, 1), f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Classes) != 1 || !workspace.Pagination.HasMore || workspace.Pagination.NextLimit != 501 || len(workspace.MakeUps) != 1 {
		t.Fatalf("pagination or makeup=%+v", workspace)
	}
	if workspace.Workload.ScheduledClasses != 2 {
		t.Fatalf("limited discovery undercounted workload=%+v", workspace.Workload)
	}
	var ledgerBefore, ledgerAfter int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM entitlement_entries`).Scan(&ledgerBefore); err != nil {
		t.Fatal(err)
	}
	fulfillment := MakeUpFulfillment{ReplacementClassID: result.Classes[1].ID, IdempotencyKey: "link-replacement"}
	if _, err = f.service.FulfillMakeUp(ctx, f.guardian, workspace.MakeUps[0].ID, fulfillment); !errors.Is(err, ErrDenied) {
		t.Fatalf("guardian fulfilled make-up: %v", err)
	}
	linked, err := f.service.FulfillMakeUp(ctx, f.coach, workspace.MakeUps[0].ID, fulfillment)
	if err != nil {
		t.Fatal(err)
	}
	if linked.Status != "fulfilled" || linked.ReplacementClassID != result.Classes[1].ID {
		t.Fatal(linked)
	}
	if _, err = f.service.FulfillMakeUp(ctx, f.coach, linked.ID, fulfillment); err != nil {
		t.Fatal(err)
	}
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM entitlement_entries`).Scan(&ledgerAfter); err != nil {
		t.Fatal(err)
	}
	if ledgerBefore != ledgerAfter {
		t.Fatal("make-up link granted or consumed extra credits")
	}
	if _, err = f.service.Cancel(ctx, f.root, result.Classes[2].ID, CancellationInput{Revision: 0, Reason: "Academy cancelled", IdempotencyKey: "cancel-second"}, false); err != nil {
		t.Fatal(err)
	}
	var another string
	if err = f.pool.QueryRow(ctx, `SELECT id::text FROM academy_make_up_obligations WHERE status='open'`).Scan(&another); err != nil {
		t.Fatal(err)
	}
	fulfillment.IdempotencyKey = "duplicate-replacement"
	if _, err = f.service.FulfillMakeUp(ctx, f.root, another, fulfillment); !errors.Is(err, ErrInvalid) {
		t.Fatalf("replacement fulfilled twice: %v", err)
	}
	full, err := f.service.Workspace(WithWorkspaceLimit(ctx, workspace.Pagination.NextLimit), f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if full.Pagination.HasMore || len(full.Classes) != 3 {
		t.Fatal("load more did not exhaust records")
	}
	slot, err := f.service.CreateTrialSlot(ctx, f.root, TrialSlot{CoachID: f.coach, StartsAt: f.now.Add(120 * time.Hour), EndsAt: f.now.Add(121 * time.Hour), TimeZone: "UTC", Format: "group", Capacity: 4, AgeBands: []string{"8_12"}})
	if err != nil {
		t.Fatal(err)
	}
	full, err = f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(full.TrialSlots) != 1 || full.TrialSlots[0].ID != slot.ID {
		t.Fatal("assigned assessor cannot discover trial")
	}
}
