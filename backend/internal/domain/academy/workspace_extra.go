package academy

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

type workspaceLimitKey struct{}

func WithWorkspaceLimit(ctx context.Context, limit int) context.Context {
	return context.WithValue(ctx, workspaceLimitKey{}, limit)
}
func workspaceLimit(ctx context.Context) int {
	limit, _ := ctx.Value(workspaceLimitKey{}).(int)
	if limit <= 0 {
		return 500
	}
	return limit
}
func bounded[T any](items []T, pagination *Pagination) []T {
	if len(items) > pagination.Limit {
		pagination.HasMore = true
		return items[:pagination.Limit]
	}
	return items
}

func workspaceStudentScope(ctx context.Context, q querier, actor string, all, coach bool) ([]string, []string, error) {
	rows, err := q.Query(ctx, `SELECT s.id::text,(s.user_id=$1::uuid OR EXISTS(SELECT 1 FROM guardian_student_relationships g WHERE g.student_profile_id=s.id AND g.guardian_user_id=$1::uuid AND g.status='active' AND 'view_schedule'=ANY(g.permissions))) FROM student_profiles s WHERE s.status<>'archived' AND ($2 OR s.user_id=$1::uuid OR EXISTS(SELECT 1 FROM guardian_student_relationships g WHERE g.student_profile_id=s.id AND g.guardian_user_id=$1::uuid AND g.status='active' AND ('view_profile'=ANY(g.permissions) OR 'view_schedule'=ANY(g.permissions))) OR($3 AND EXISTS(SELECT 1 FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=s.id AND e.status='active' AND b.coach_user_id=$1::uuid AND b.status='active')))`, actor, all, coach)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	ids, schedule := []string{}, []string{}
	for rows.Next() {
		var id string
		var related bool
		if err = rows.Scan(&id, &related); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
		if related {
			schedule = append(schedule, id)
		}
	}
	return ids, schedule, rows.Err()
}

func workspaceExtras(ctx context.Context, tx pgx.Tx, actor string, out *Workspace, ids, scheduleIDs []string, isCoach, allClasses, familyView bool) error {
	limitSQL := strconv.Itoa(out.Pagination.Limit + 1)
	rows, err := tx.Query(ctx, `SELECT id::text,class_session_id::text,student_profile_id::text,reason,status,COALESCE(replacement_session_id::text,''),created_at FROM academy_make_up_obligations WHERE student_profile_id=ANY($1::uuid[]) ORDER BY created_at DESC LIMIT `+limitSQL, scheduleIDs)
	if err != nil {
		return err
	}
	for rows.Next() {
		var m MakeUp
		if err = rows.Scan(&m.ID, &m.ClassID, &m.StudentID, &m.Reason, &m.Status, &m.ReplacementClassID, &m.CreatedAt); err != nil {
			rows.Close()
			return err
		}
		out.MakeUps = append(out.MakeUps, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	out.MakeUps = bounded(out.MakeUps, &out.Pagination)
	filtered := out.MakeUps[:0]
	for _, item := range out.MakeUps {
		_, deny, err := permission(ctx, tx, actor, "schedule.view", item.StudentID)
		if err != nil {
			return err
		}
		if !deny {
			filtered = append(filtered, item)
		}
	}
	out.MakeUps = filtered
	rows, err = tx.Query(ctx, `SELECT s.id::text,COALESCE(s.assessor_user_id::text,''),s.starts_at,s.ends_at,s.source_time_zone,s.format,s.capacity,s.age_bands,s.status,s.capacity-count(b.id) FILTER(WHERE b.status IN('confirmed','attended')) FROM trial_slots s LEFT JOIN trial_bookings b ON b.slot_id=s.id WHERE ($2 OR($3 AND s.assessor_user_id=$1::uuid) OR EXISTS(SELECT 1 FROM trial_bookings mine JOIN users u ON u.email=mine.email WHERE mine.slot_id=s.id AND u.id=$1::uuid AND mine.status='confirmed')) GROUP BY s.id ORDER BY s.starts_at DESC LIMIT `+limitSQL, actor, allClasses, isCoach)
	if err != nil {
		return err
	}
	for rows.Next() {
		var slot TrialSlot
		if err = rows.Scan(&slot.ID, &slot.CoachID, &slot.StartsAt, &slot.EndsAt, &slot.TimeZone, &slot.Format, &slot.Capacity, &slot.AgeBands, &slot.Status, &slot.AvailableSeats); err != nil {
			rows.Close()
			return err
		}
		out.TrialSlots = append(out.TrialSlots, slot)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	out.TrialSlots = bounded(out.TrialSlots, &out.Pagination)
	// Aggregation reads every authorized row; discovery limits cannot undercount hours.
	rows, err = tx.Query(ctx, `SELECT c.starts_at,c.ends_at,c.status FROM class_sessions c WHERE ($2 OR($3 AND EXISTS(SELECT 1 FROM session_assignments a WHERE a.class_session_id=c.id AND a.user_id=$1::uuid AND a.role='coach')) OR ($5 AND EXISTS(SELECT 1 FROM academy_class_participants p WHERE p.class_session_id=c.id AND p.student_profile_id=ANY($4::uuid[])))) AND NOT EXISTS(SELECT 1 FROM permission_grants g WHERE g.user_id=$1::uuid AND g.permission_code='schedule.view' AND g.effect='deny' AND g.revoked_at IS NULL AND(g.expires_at IS NULL OR g.expires_at>now()) AND(g.scope_type='global' OR(g.scope_type='student' AND g.scope_id IN(SELECT student_profile_id FROM academy_class_participants WHERE class_session_id=c.id)) OR(g.scope_type='family' AND g.scope_id IN(SELECT r.family_id FROM academy_class_participants p JOIN guardian_student_relationships r ON r.student_profile_id=p.student_profile_id WHERE p.class_session_id=c.id AND r.status='active'))))`, actor, allClasses, isCoach, scheduleIDs, familyView)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	current := now.Format("2006-01")
	prior := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0).Format("2006-01")
	months := map[string]*MonthlyWorkload{current: {Month: current}, prior: {Month: prior}}
	out.Workload = Workload{}
	for rows.Next() {
		var start, end time.Time
		var status string
		if err = rows.Scan(&start, &end, &status); err != nil {
			rows.Close()
			return err
		}
		if status == "completed" {
			minutes := int(end.Sub(start) / time.Minute)
			out.Workload.CompletedClasses++
			out.Workload.CompletedMinutes += minutes
			if month := months[start.UTC().Format("2006-01")]; month != nil {
				month.CompletedClasses++
				month.CompletedClassMinutes += minutes
			}
		} else if status == "scheduled" || status == "join_open" || status == "live" || status == "ended" || status == "completion_pending" {
			out.Workload.ScheduledClasses++
			if !start.After(now) {
				out.Workload.PendingCompletion++
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	rows, err = tx.Query(ctx, `SELECT s.starts_at,s.ends_at FROM trial_slots s WHERE s.starts_at>=$4 AND ($2 OR($3 AND s.assessor_user_id=$1::uuid)) AND EXISTS(SELECT 1 FROM trial_bookings WHERE slot_id=s.id AND status='attended')`, actor, allClasses, isCoach, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0))
	if err != nil {
		return err
	}
	for rows.Next() {
		var start, end time.Time
		if err = rows.Scan(&start, &end); err != nil {
			rows.Close()
			return err
		}
		if month := months[start.UTC().Format("2006-01")]; month != nil {
			month.CompletedTrials++
			month.CompletedTrialMinutes += int(end.Sub(start) / time.Minute)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	out.Workload.Months = []MonthlyWorkload{*months[current], *months[prior]}
	return nil
}
