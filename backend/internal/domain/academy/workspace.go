package academy

import (
	"context"
	"github.com/jackc/pgx/v5"
	"html"
	"strconv"
	"strings"
	"time"
)

func (r *PostgresRepository) Workspace(ctx context.Context, actor string) (Workspace, error) {
	out := Workspace{Students: []Student{}, Coaches: []Coach{}, Batches: []Batch{}, Enrolments: []Enrolment{}, Classes: []Class{}, Policies: []Policy{}, Availability: []Availability{}, MakeUps: []MakeUp{}, TrialSlots: []TrialSlot{}, Pagination: Pagination{Limit: workspaceLimit(ctx), NextLimit: workspaceLimit(ctx) + 500}}
	limitSQL := strconv.Itoa(out.Pagination.Limit + 1)
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	var active bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1::uuid AND status='active')`, actor).Scan(&active); err != nil {
		return out, err
	}
	if !active {
		return out, ErrDenied
	}
	checks := []struct {
		code   string
		target *bool
	}{{"academy.manage", &out.Capabilities.ManageAcademy}, {"schedule.manage", &out.Capabilities.ManageScheduling}, {"teaching.manage", &out.Capabilities.ManageTeaching}, {"academy.staff.manage", &out.Capabilities.ManageStaff}, {"academy.policy.manage", &out.Capabilities.ManagePolicies}}
	for _, check := range checks {
		allow, deny, err := permission(ctx, tx, actor, check.code, "")
		if err != nil {
			return out, err
		}
		*check.target = allow && !deny
	}
	isCoach, err := role(ctx, tx, actor, "coach")
	if err != nil {
		return out, err
	}
	if isCoach {
		for _, check := range checks[1:3] {
			_, deny, err := permission(ctx, tx, actor, check.code, "")
			if err != nil {
				return out, err
			}
			if !deny {
				*check.target = true
			}
		}
	}
	finance, financeDeny, err := permission(ctx, tx, actor, "billing.entitlement.view", "")
	if err != nil {
		return out, err
	}
	scheduleView, scheduleDeny, err := permission(ctx, tx, actor, "schedule.view", "")
	if err != nil {
		return out, err
	}
	allStudents := out.Capabilities.ManageAcademy || (!isCoach && out.Capabilities.ManageTeaching) || (finance && !financeDeny) || (scheduleView && !scheduleDeny)
	rows, err := tx.Query(ctx, `SELECT s.id::text,s.user_id::text,COALESCE((SELECT r.family_id::text FROM guardian_student_relationships r WHERE r.student_profile_id=s.id AND r.status='active' ORDER BY r.created_at LIMIT 1),''),s.preferred_name,s.age_band,s.time_zone,s.status,s.country
 FROM student_profiles s WHERE s.status<>'archived' AND ($2 OR s.user_id=$1::uuid
 OR EXISTS(SELECT 1 FROM guardian_student_relationships g WHERE g.student_profile_id=s.id AND g.guardian_user_id=$1::uuid AND g.status='active' AND ('view_profile'=ANY(g.permissions) OR 'view_schedule'=ANY(g.permissions)))
 OR ($3 AND EXISTS(SELECT 1 FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=s.id AND e.status='active' AND b.coach_user_id=$1::uuid AND b.status='active')))
 ORDER BY s.preferred_name,s.id LIMIT `+limitSQL, actor, allStudents, isCoach)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var student Student
		if err = rows.Scan(&student.ID, &student.UserID, &student.FamilyID, &student.Name, &student.AgeBand, &student.TimeZone, &student.Status, &student.Country); err != nil {
			rows.Close()
			return out, err
		}
		out.Students = append(out.Students, student)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Students = bounded(out.Students, &out.Pagination)
	ids, scheduleIDs, err := workspaceStudentScope(ctx, tx, actor, allStudents, isCoach)
	if err != nil {
		return out, err
	}
	if isCoach || (scheduleView && !scheduleDeny) || out.Capabilities.ManageAcademy {
		scheduleIDs = ids
	}
	visible := map[string]bool{}
	for _, id := range scheduleIDs {
		visible[id] = true
	}
	for i := range out.Students {
		out.Students[i].Consent, err = consentCurrent(ctx, tx, out.Students[i].ID)
		if err != nil {
			return out, err
		}
	}
	// Staff discovery contains names and IDs only; unrelated family details never
	// enter the workspace response or become a client-side filtering responsibility.
	rows, err = tx.Query(ctx, `SELECT DISTINCT u.id::text,u.display_name FROM users u JOIN role_grants g ON g.user_id=u.id WHERE u.status='active' AND g.role='coach' AND g.revoked_at IS NULL AND ($2 OR u.id=$1::uuid OR EXISTS(SELECT 1 FROM batches b JOIN enrolments e ON e.batch_id=b.id WHERE b.coach_user_id=u.id AND e.student_profile_id=ANY($3::uuid[]) AND e.status='active')) ORDER BY u.display_name LIMIT `+limitSQL, actor, out.Capabilities.ManageAcademy, ids)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var coach Coach
		if err = rows.Scan(&coach.ID, &coach.Name); err != nil {
			rows.Close()
			return out, err
		}
		out.Coaches = append(out.Coaches, coach)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Coaches = bounded(out.Coaches, &out.Pagination)
	rows, err = tx.Query(ctx, `SELECT b.id::text,b.name,COALESCE(b.coach_user_id::text,''),b.format,b.level_slug,b.source_time_zone,b.hard_capacity,ARRAY(SELECT e.student_profile_id::text FROM enrolments e WHERE e.batch_id=b.id AND e.status='active' AND e.student_profile_id=ANY($3::uuid[]) ORDER BY e.student_profile_id)
 FROM batches b WHERE b.status='active' AND ($2 OR b.coach_user_id=$1::uuid OR EXISTS(SELECT 1 FROM enrolments e WHERE e.batch_id=b.id AND e.student_profile_id=ANY($3::uuid[]) AND e.status='active')) ORDER BY b.name LIMIT `+limitSQL, actor, out.Capabilities.ManageAcademy || (scheduleView && !scheduleDeny), ids)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var batch Batch
		if err = rows.Scan(&batch.ID, &batch.Name, &batch.CoachID, &batch.Format, &batch.Level, &batch.TimeZone, &batch.Capacity, &batch.StudentIDs); err != nil {
			rows.Close()
			return out, err
		}
		out.Batches = append(out.Batches, batch)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Batches = bounded(out.Batches, &out.Pagination)
	rows, err = tx.Query(ctx, `SELECT e.id::text,e.student_profile_id::text,COALESCE(e.batch_id::text,''),a.id::text,e.status,e.starts_at,e.ends_at,COALESCE(sum(l.available_delta),0),COALESCE(sum(l.reserved_delta),0),COALESCE(sum(l.consumed_delta),0) FROM enrolments e JOIN entitlement_accounts a ON a.enrolment_id=e.id LEFT JOIN entitlement_entries l ON l.account_id=a.id WHERE e.student_profile_id=ANY($1::uuid[]) GROUP BY e.id,a.id ORDER BY e.created_at DESC LIMIT `+limitSQL, ids)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var enrolment Enrolment
		if err = rows.Scan(&enrolment.ID, &enrolment.StudentID, &enrolment.BatchID, &enrolment.AccountID, &enrolment.Status, &enrolment.StartsAt, &enrolment.EndsAt, &enrolment.Available, &enrolment.Reserved, &enrolment.Consumed); err != nil {
			rows.Close()
			return out, err
		}
		out.Enrolments = append(out.Enrolments, enrolment)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	out.Enrolments = bounded(out.Enrolments, &out.Pagination)
	// Coaches need funding readiness for scheduling, but financial account history
	// remains under the separately authorized entitlement API.
	classIDs := []string{}
	isGuardian, err := role(ctx, tx, actor, "guardian")
	if err != nil {
		return out, err
	}
	isStudent, err := role(ctx, tx, actor, "student")
	if err != nil {
		return out, err
	}
	rows, err = tx.Query(ctx, `SELECT c.id::text FROM class_sessions c JOIN batches b ON b.id=c.batch_id WHERE ($2 OR ($3 AND EXISTS(SELECT 1 FROM session_assignments a WHERE a.class_session_id=c.id AND a.user_id=$1::uuid AND a.role='coach')) OR ($5 AND EXISTS(SELECT 1 FROM session_assignments a JOIN student_profiles s ON s.user_id=a.user_id WHERE a.class_session_id=c.id AND s.id=ANY($4::uuid[])))) ORDER BY c.starts_at DESC LIMIT `+limitSQL, actor, (scheduleView && !scheduleDeny) || out.Capabilities.ManageAcademy, isCoach, scheduleIDs, isGuardian || isStudent)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return out, err
		}
		classIDs = append(classIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	classIDs = bounded(classIDs, &out.Pagination)
	for _, id := range classIDs {
		lesson, err := loadClass(ctx, tx, id)
		if err != nil {
			return out, err
		}
		filtered := []string{}
		deniedClass := false
		for _, sid := range lesson.StudentIDs {
			_, deny, err := permission(ctx, tx, actor, "schedule.view", sid)
			if err != nil {
				return out, err
			}
			if deny {
				deniedClass = true
				break
			}
			if visible[sid] {
				filtered = append(filtered, sid)
			} else {
				delete(lesson.Attendance, sid)
			}
		}
		if deniedClass {
			continue
		}
		lesson.StudentIDs = filtered
		out.Classes = append(out.Classes, lesson)
		if lesson.Status == "completed" {
			out.Workload.CompletedClasses++
			out.Workload.CompletedMinutes += int(lesson.EndsAt.Sub(lesson.StartsAt) / time.Minute)
		} else if lesson.Status == "scheduled" || lesson.Status == "join_open" || lesson.Status == "live" {
			out.Workload.ScheduledClasses++
			if !lesson.StartsAt.After(time.Now()) {
				out.Workload.PendingCompletion++
			}
		}
	}
	if err = workspaceExtras(ctx, tx, actor, &out, ids, scheduleIDs, isCoach, (scheduleView && !scheduleDeny) || out.Capabilities.ManageAcademy, isGuardian || isStudent); err != nil {
		return out, err
	}
	if scheduleDeny {
		out.TrialSlots = []TrialSlot{}
		for i := range out.Workload.Months {
			out.Workload.Months[i].CompletedTrials = 0
			out.Workload.Months[i].CompletedTrialMinutes = 0
		}
	}
	out.Policies, err = currentPolicies(ctx, tx)
	if err != nil {
		return out, err
	}
	err = tx.QueryRow(ctx, `SELECT name,COALESCE(legal_name,''),COALESCE(email::text,'') FROM organization_settings`).Scan(&out.Settings.Name, &out.Settings.LegalName, &out.Settings.Email)
	if err != nil {
		return out, err
	}
	out.Profile, err = loadProfile(ctx, tx, actor)
	if err != nil {
		return out, err
	}
	if out.Capabilities.ManageScheduling {
		out.Availability, err = r.Availability(ctx, actor)
		if err != nil {
			return out, err
		}
	}
	out.Availability = bounded(out.Availability, &out.Pagination)
	return out, tx.Commit(ctx)
}
func currentPolicies(ctx context.Context, q querier) ([]Policy, error) {
	policies := []Policy{}
	rows, err := q.Query(ctx, `SELECT DISTINCT ON(p.slug) v.id::text,p.slug,p.title,v.version,v.content_html FROM policies p JOIN policy_versions v ON v.policy_id=p.id WHERE p.slug IN('privacy','child-safety','terms','cancellation-refund') AND v.status='published' AND v.effective_at<=now() ORDER BY p.slug,v.effective_at DESC,v.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var policy Policy
		if err = rows.Scan(&policy.ID, &policy.Slug, &policy.Title, &policy.Version, &policy.Content); err != nil {
			return nil, err
		}
		policy.Content = html.UnescapeString(strings.ReplaceAll(strings.ReplaceAll(policy.Content, "</p>", "\n"), "<p>", ""))
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}
func loadClass(ctx context.Context, q querier, id string) (Class, error) {
	lesson := Class{StudentIDs: []string{}, Attendance: map[string]string{}}
	err := q.QueryRow(ctx, `SELECT c.id::text,c.batch_id::text,c.title,COALESCE(ca.user_id::text,b.coach_user_id::text,''),COALESCE(u.display_name,''),b.format,c.starts_at,c.ends_at,c.source_time_zone,c.status,c.revision,c.lesson_notes,COALESCE(c.meeting_url,'') FROM class_sessions c JOIN batches b ON b.id=c.batch_id LEFT JOIN LATERAL(SELECT user_id FROM session_assignments WHERE class_session_id=c.id AND role='coach' ORDER BY starts_at DESC LIMIT 1) ca ON true LEFT JOIN users u ON u.id=COALESCE(ca.user_id,b.coach_user_id) WHERE c.id=$1::uuid`, id).Scan(&lesson.ID, &lesson.BatchID, &lesson.Title, &lesson.CoachID, &lesson.Coach, &lesson.Format, &lesson.StartsAt, &lesson.EndsAt, &lesson.TimeZone, &lesson.Status, &lesson.Revision, &lesson.Notes, &lesson.MeetingURL)
	if err != nil {
		return lesson, dbError(err)
	}
	rows, err := q.Query(ctx, `SELECT p.student_profile_id::text,COALESCE(a.status,'') FROM academy_class_participants p LEFT JOIN attendance a ON a.class_session_id=p.class_session_id AND a.student_profile_id=p.student_profile_id WHERE p.class_session_id=$1::uuid ORDER BY p.student_profile_id`, id)
	if err != nil {
		return lesson, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid, status string
		if err = rows.Scan(&sid, &status); err != nil {
			return lesson, err
		}
		lesson.StudentIDs = append(lesson.StudentIDs, sid)
		if status != "" {
			lesson.Attendance[sid] = status
		}
	}
	return lesson, rows.Err()
}
