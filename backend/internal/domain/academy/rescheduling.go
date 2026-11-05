package academy

import (
	"context"
	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"time"
)

type reschedulePlan struct {
	lesson     Class
	start, end time.Time
	sourceDate string
	people     []participant
}

func (r *PostgresRepository) UpdateClass(ctx context.Context, actor, id string, in ClassInput, start, now time.Time) (Class, error) {
	var out Class
	err := r.within(ctx, func(tx pgx.Tx) error {
		selected, err := loadClass(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = batchPermission(ctx, tx, actor, selected.BatchID, "schedule.manage"); err != nil {
			return err
		}
		if err = mutableClass(selected, in.Revision); err != nil {
			return err
		}
		if selected.BatchID != in.BatchID || !selected.StartsAt.After(now) {
			return invalid("only an upcoming class can be edited; its batch must stay the same")
		}
		ids := []string{id}
		var recurrenceID string
		if err = tx.QueryRow(ctx, `SELECT COALESCE(recurrence_rule_id::text,'') FROM class_sessions WHERE id=$1::uuid`, id).Scan(&recurrenceID); err != nil {
			return err
		}
		if in.Scope == "future" {
			if recurrenceID == "" {
				return invalid("this class is not part of a recurring series")
			}
			rows, err := tx.Query(ctx, `SELECT id::text FROM class_sessions WHERE recurrence_rule_id=$1::uuid AND starts_at>=$2 AND starts_at>$3 AND status IN('scheduled','join_open') ORDER BY starts_at`, recurrenceID, selected.StartsAt, now)
			if err != nil {
				return err
			}
			ids = []string{}
			for rows.Next() {
				var cid string
				if err = rows.Scan(&cid); err != nil {
					rows.Close()
					return err
				}
				ids = append(ids, cid)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		if len(ids) > 52 {
			return invalid("reschedule at most 52 occurrences at a time")
		}
		oldZone, err := time.LoadLocation(selected.TimeZone)
		if err != nil {
			return err
		}
		newZone, err := time.LoadLocation(in.TimeZone)
		if err != nil {
			return err
		}
		originalDate, err := time.Parse("2006-01-02", selected.StartsAt.In(oldZone).Format("2006-01-02"))
		if err != nil {
			return err
		}
		newWall, err := time.Parse("2006-01-02T15:04", in.LocalStart)
		if err != nil {
			return err
		}
		plans := []reschedulePlan{}
		for _, cid := range ids {
			lesson, err := loadClass(ctx, tx, cid)
			if err != nil {
				return err
			}
			people, err := classParticipants(ctx, tx, cid)
			if err != nil {
				return err
			}
			if err = checkStudentScope(ctx, tx, actor, people, "schedule.manage"); err != nil {
				return err
			}
			sourceDate := lesson.StartsAt.In(oldZone).Format("2006-01-02")
			occurrenceDate, err := time.Parse("2006-01-02", sourceDate)
			if err != nil {
				return err
			}
			days := int(occurrenceDate.Sub(originalDate) / (24 * time.Hour))
			one := in
			one.Scope = "one"
			one.Occurrences = 1
			one.RecurringUntil = ""
			one.LocalStart = newWall.AddDate(0, 0, days).Format("2006-01-02T15:04")
			starts, err := OccurrenceStarts(one, now)
			if err != nil {
				return err
			}
			next := starts[0]
			end := next.Add(time.Duration(in.DurationMinutes) * time.Minute)
			active := []participant{}
			for _, person := range people {
				if person.status != "reserved" {
					continue
				}
				var eligible bool
				err = tx.QueryRow(ctx, `SELECT status='active' AND starts_at<=$2 AND (ends_at IS NULL OR ends_at>=$3) FROM enrolments WHERE id=$1::uuid`, person.enrolmentID, next, end).Scan(&eligible)
				if err != nil {
					return err
				}
				if !eligible {
					return invalid("new date is outside a student's active enrolment")
				}
				active = append(active, person)
			}
			if err = conflictsExcluding(ctx, tx, lesson.CoachID, active, next, end, ids); err != nil {
				return err
			}
			if err = coachAvailability(ctx, tx, lesson.CoachID, next, end); err != nil {
				return err
			}
			for _, prior := range plans {
				if next.Before(prior.end) && end.After(prior.start) {
					return invalid("rescheduled occurrences overlap")
				}
			}
			plans = append(plans, reschedulePlan{lesson: lesson, start: next, end: end, sourceDate: sourceDate, people: active})
		}
		for _, plan := range plans {
			_, err = tx.Exec(ctx, `UPDATE class_sessions SET title=$2,starts_at=$3,ends_at=$4,source_time_zone=$5,source_local_start=$6::timestamp,join_opens_at=$7,revision=revision+1,updated_at=$8,meeting_url=$9 WHERE id=$1::uuid`, plan.lesson.ID, in.Title, plan.start, plan.end, in.TimeZone, plan.start.In(newZone).Format("2006-01-02 15:04:05"), plan.start.Add(-15*time.Minute), now, in.MeetingURL)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `UPDATE session_assignments SET starts_at=$2,ends_at=$3 WHERE class_session_id=$1::uuid AND user_id NOT IN(SELECT s.user_id FROM academy_class_participants p JOIN student_profiles s ON s.id=p.student_profile_id WHERE p.class_session_id=$1::uuid AND p.status<>'reserved')`, plan.lesson.ID, plan.start.Add(-15*time.Minute), plan.end)
			if err != nil {
				return err
			}
			if recurrenceID != "" {
				_, err = tx.Exec(ctx, `INSERT INTO recurrence_exceptions(id,recurrence_rule_id,source_local_date,action,replacement_local_start,replacement_duration_minutes,reason,created_by,created_at) VALUES($1::uuid,$2::uuid,$3::date,'replace',$4::timestamp,$5,'staff reschedule',$6::uuid,$7) ON CONFLICT(recurrence_rule_id,source_local_date) DO UPDATE SET replacement_local_start=EXCLUDED.replacement_local_start,replacement_duration_minutes=EXCLUDED.replacement_duration_minutes,created_by=EXCLUDED.created_by,created_at=EXCLUDED.created_at`, platformid.New(), recurrenceID, plan.sourceDate, plan.start.In(newZone).Format("2006-01-02 15:04:05"), in.DurationMinutes, actor, now)
				if err != nil {
					return err
				}
			}
			if err = audit(ctx, tx, actor, "academy.class.rescheduled", plan.lesson.ID, now, map[string]any{"startsAt": plan.start, "scope": in.Scope, "revision": plan.lesson.Revision + 1}); err != nil {
				return err
			}
			if err = notifyClass(ctx, tx, plan.lesson.ID, "Academy class time updated", now, true); err != nil {
				return err
			}
		}
		out, err = loadClass(ctx, tx, id)
		return err
	})
	return out, err
}
