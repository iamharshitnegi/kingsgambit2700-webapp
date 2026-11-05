package academy

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/domain/classroom"
	"github.com/kingsgambit2700/platform/backend/internal/domain/entitlement"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

type participant struct{ studentID, userID, enrolmentID, accountID, status string }

func roster(ctx context.Context, q querier, batch string, start, end time.Time) ([]participant, error) {
	rows, err := q.Query(ctx, `SELECT e.student_profile_id::text,s.user_id::text,e.id::text,a.id::text FROM enrolments e JOIN student_profiles s ON s.id=e.student_profile_id JOIN users u ON u.id=s.user_id JOIN entitlement_accounts a ON a.enrolment_id=e.id WHERE e.batch_id=$1::uuid AND e.status='active' AND s.status='active' AND u.status='active' AND e.starts_at<=$2 AND (e.ends_at IS NULL OR e.ends_at>=$3) ORDER BY s.id`, batch, start, end)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := []participant{}
	for rows.Next() {
		var person participant
		if err = rows.Scan(&person.studentID, &person.userID, &person.enrolmentID, &person.accountID); err != nil {
			return nil, err
		}
		person.status = "reserved"
		people = append(people, person)
	}
	return people, rows.Err()
}
func classParticipants(ctx context.Context, q querier, classID string) ([]participant, error) {
	rows, err := q.Query(ctx, `SELECT p.student_profile_id::text,s.user_id::text,p.enrolment_id::text,a.id::text,p.status FROM academy_class_participants p JOIN student_profiles s ON s.id=p.student_profile_id JOIN entitlement_accounts a ON a.enrolment_id=p.enrolment_id WHERE p.class_session_id=$1::uuid ORDER BY s.id`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := []participant{}
	for rows.Next() {
		var person participant
		if err = rows.Scan(&person.studentID, &person.userID, &person.enrolmentID, &person.accountID, &person.status); err != nil {
			return nil, err
		}
		people = append(people, person)
	}
	return people, rows.Err()
}
func loadBatch(ctx context.Context, q querier, id string) (Batch, string, error) {
	var batch Batch
	var coachName string
	err := q.QueryRow(ctx, `SELECT b.id::text,b.name,b.coach_user_id::text,b.format,b.level_slug,b.source_time_zone,b.hard_capacity,u.display_name FROM batches b JOIN users u ON u.id=b.coach_user_id WHERE b.id=$1::uuid AND b.status='active' AND u.status='active'`, id).Scan(&batch.ID, &batch.Name, &batch.CoachID, &batch.Format, &batch.Level, &batch.TimeZone, &batch.Capacity, &coachName)
	if err != nil {
		return batch, "", err
	}
	ok, err := role(ctx, q, batch.CoachID, "coach")
	if err != nil {
		return batch, "", err
	}
	if !ok {
		return batch, "", invalid("assigned coach is inactive")
	}
	return batch, coachName, nil
}
func conflict(ctx context.Context, q querier, coach string, people []participant, start, end time.Time, ignore string) error {
	return conflictsExcluding(ctx, q, coach, people, start, end, []string{ignore})
}
func conflictsExcluding(ctx context.Context, q querier, coach string, people []participant, start, end time.Time, ignore []string) error {
	users := []string{coach}
	for _, p := range people {
		users = append(users, p.userID)
	}
	var overlaps bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM class_sessions c JOIN session_assignments a ON a.class_session_id=c.id WHERE NOT(c.id::text=ANY($4::text[])) AND c.status NOT IN('cancelled_academy','cancelled_family','rescheduled','completed','technical_failure','no_show') AND c.starts_at<$3 AND c.ends_at>$2 AND a.user_id=ANY($1::uuid[]) AND (a.ends_at IS NULL OR a.ends_at>$2)) OR EXISTS(SELECT 1 FROM trial_slots WHERE assessor_user_id=$5::uuid AND status='published' AND starts_at<$3 AND ends_at>$2)`, users, start, end, ignore, coach).Scan(&overlaps)
	if err != nil {
		return err
	}
	if overlaps {
		return fmt.Errorf("%w: coach or student already has a class at this time", ErrConflict)
	}
	return nil
}
func checkStudentScope(ctx context.Context, q querier, actor string, people []participant, code string) error {
	for _, p := range people {
		if err := studentPermission(ctx, q, actor, p.studentID, code, "", true); err != nil {
			return err
		}
	}
	return nil
}
func (r *PostgresRepository) Schedule(ctx context.Context, actor string, in ClassInput, starts []time.Time, now time.Time, preview bool) (ClassResult, error) {
	out := ClassResult{Classes: []Class{}}
	err := r.within(ctx, func(tx pgx.Tx) error {
		if err := batchPermission(ctx, tx, actor, in.BatchID, "schedule.manage"); err != nil {
			return err
		}
		if !preview {
			found, err := replay(ctx, tx, actor, "schedule", in.IdempotencyKey, in, &out)
			if err != nil || found {
				return err
			}
		}
		batch, coachName, err := loadBatch(ctx, tx, in.BatchID)
		if err != nil {
			return err
		}
		rosters := make([][]participant, 0, len(starts))
		needed := map[string]int{}
		for _, start := range starts {
			end := start.Add(time.Duration(in.DurationMinutes) * time.Minute)
			people, err := roster(ctx, tx, in.BatchID, start, end)
			if err != nil {
				return err
			}
			if len(people) < 1 || len(people) > batch.Capacity || (batch.Format == "one_to_one" && len(people) != 1) {
				return invalid("every occurrence needs active enrolments within batch capacity")
			}
			if err = checkStudentScope(ctx, tx, actor, people, "schedule.manage"); err != nil {
				return err
			}
			for _, person := range people {
				consent, err := consentCurrent(ctx, tx, person.studentID)
				if err != nil {
					return err
				}
				if !consent {
					return ErrConsent
				}
				needed[person.accountID]++
			}
			if err = conflict(ctx, tx, batch.CoachID, people, start, end, ""); err != nil {
				return err
			}
			if err = coachAvailability(ctx, tx, batch.CoachID, start, end); err != nil {
				return err
			}
			lesson := Class{ID: platformid.New(), BatchID: batch.ID, Title: in.Title, CoachID: batch.CoachID, Coach: coachName, Format: batch.Format, StudentIDs: []string{}, StartsAt: start, EndsAt: end, TimeZone: in.TimeZone, Status: "scheduled", Attendance: map[string]string{}, MeetingURL: in.MeetingURL}
			for _, person := range people {
				lesson.StudentIDs = append(lesson.StudentIDs, person.studentID)
			}
			out.Classes = append(out.Classes, lesson)
			rosters = append(rosters, people)
		}
		// Check all reservations together before the first write. ApplyInTx repeats
		// the balance check while locking each account, so concurrent finance writes
		// cannot cause an unfunded occurrence or partial series.
		for account, count := range needed {
			var available int64
			if err = tx.QueryRow(ctx, `SELECT COALESCE(sum(available_delta),0) FROM entitlement_entries WHERE account_id=$1::uuid`, account).Scan(&available); err != nil {
				return err
			}
			if available < int64(count) {
				return ErrCredits
			}
		}
		if preview {
			return nil
		}
		recurrenceID := ""
		if len(starts) > 1 {
			recurrenceID = platformid.New()
			local, _ := time.Parse("2006-01-02T15:04", in.LocalStart)
			if _, err = tx.Exec(ctx, `UPDATE recurrence_rules SET status='superseded' WHERE owner_type='batch' AND owner_id=$1::uuid AND status='active'`, batch.ID); err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO recurrence_rules(id,owner_type,owner_id,version,rrule,source_time_zone,local_start_time,duration_minutes,effective_from,effective_until,status,created_by,created_at) VALUES($1::uuid,'batch',$10::uuid,(SELECT COALESCE(max(version),0)+1 FROM recurrence_rules WHERE owner_type='batch' AND owner_id=$10::uuid),$2,$3,$4::time,$5,$6::date,$7::date,'active',$8::uuid,$9)`, recurrenceID, fmt.Sprintf("FREQ=WEEKLY;COUNT=%d", len(starts)), in.TimeZone, local.Format("15:04"), in.DurationMinutes, local.Format("2006-01-02"), local.AddDate(0, 0, 7*(len(starts)-1)).Format("2006-01-02"), actor, now, batch.ID)
			if err != nil {
				return err
			}
		}
		for i, lesson := range out.Classes {
			if err = insertClass(ctx, tx, actor, lesson, rosters[i], recurrenceID, now); err != nil {
				return err
			}
			if err = notifyClass(ctx, tx, lesson.ID, "Academy class scheduled", now, true); err != nil {
				return err
			}
		}
		return remember(ctx, tx, actor, "schedule", in.IdempotencyKey, in, out, now)
	})
	return out, err
}

const initialFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

func insertClass(ctx context.Context, tx pgx.Tx, actor string, lesson Class, people []participant, recurrenceID string, now time.Time) error {
	zone, err := time.LoadLocation(lesson.TimeZone)
	if err != nil {
		return err
	}
	local := lesson.StartsAt.In(zone).Format("2006-01-02 15:04:05")
	kind := "regular"
	if lesson.Format == "one_to_one" {
		kind = "one_to_one"
	}
	join := lesson.StartsAt.Add(-15 * time.Minute)
	_, err = tx.Exec(ctx, `INSERT INTO class_sessions(id,batch_id,kind,title,starts_at,ends_at,source_time_zone,source_local_start,join_opens_at,status,revision,created_at,updated_at,recurrence_rule_id,meeting_url) VALUES($1::uuid,$2::uuid,$3,$4,$5,$6,$7,$8::timestamp,$9,'scheduled',0,$10,$10,NULLIF($11,'')::uuid,$12)`, lesson.ID, lesson.BatchID, kind, lesson.Title, lesson.StartsAt, lesson.EndsAt, lesson.TimeZone, local, join, now, recurrenceID, lesson.MeetingURL)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO session_assignments(id,class_session_id,user_id,role,starts_at,ends_at) VALUES($1::uuid,$2::uuid,$3::uuid,'coach',$4,$5)`, platformid.New(), lesson.ID, lesson.CoachID, join, lesson.EndsAt); err != nil {
		return err
	}
	for _, person := range people {
		if _, err = tx.Exec(ctx, `INSERT INTO session_assignments(id,class_session_id,user_id,role,starts_at,ends_at) VALUES($1::uuid,$2::uuid,$3::uuid,'student',$4,$5)`, platformid.New(), lesson.ID, person.userID, join, lesson.EndsAt); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO academy_class_participants(class_session_id,student_profile_id,enrolment_id,status) VALUES($1::uuid,$2::uuid,$3::uuid,'reserved')`, lesson.ID, person.studentID, person.enrolmentID); err != nil {
			return err
		}
		if err = ledger(ctx, tx, actor, lesson.ID, person, "reserve", entitlement.EntryKindReserved, entitlement.Delta{Available: -1, Reserved: 1}, "class reservation", now); err != nil {
			return err
		}
	}
	state, err := classroom.NewState(classroom.NewStateParams{SessionID: lesson.ID, FEN: initialFEN, Mode: classroom.ModeDemonstration})
	if err != nil {
		return err
	}
	body, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO classroom_states(class_session_id,revision,fen,moves,mode,board_status,updated_at) VALUES($1::uuid,0,$2,'{}','demonstration','open',$3)`, lesson.ID, initialFEN, now); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO classroom_snapshots(id,class_session_id,revision,state,created_at) VALUES($1::uuid,$2::uuid,0,$3,$4)`, platformid.New(), lesson.ID, body, now); err != nil {
		return err
	}
	return audit(ctx, tx, actor, "academy.class.scheduled", lesson.ID, now, map[string]string{"batchId": lesson.BatchID})
}
func ledger(ctx context.Context, tx pgx.Tx, actor, classID string, person participant, operation string, kind entitlement.EntryKind, delta entitlement.Delta, reason string, now time.Time) error {
	_, err := entitlement.ApplyInTx(ctx, tx, entitlement.Entry{ID: platformid.New(), AccountID: person.accountID, StudentID: person.studentID, Kind: kind, Delta: delta, IdempotencyKey: classID + ":" + person.studentID + ":" + operation, ReferenceType: "class_session", ReferenceID: classID, Reason: reason, ActorID: actor, OccurredAt: now})
	return err
}
func mutableClass(lesson Class, revision uint64) error {
	if lesson.Revision != revision {
		return ErrConflict
	}
	switch lesson.Status {
	case "scheduled", "join_open", "live", "ended", "completion_pending":
		return nil
	default:
		return fmt.Errorf("%w: this class is already resolved", ErrConflict)
	}
}
func (r *PostgresRepository) Complete(ctx context.Context, actor, id string, in CompletionInput, now time.Time) (Class, error) {
	var out Class
	err := r.within(ctx, func(tx pgx.Tx) error {
		lesson, err := loadClass(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = batchPermission(ctx, tx, actor, lesson.BatchID, "teaching.manage"); err != nil {
			return err
		}
		found, err := replay(ctx, tx, actor, "complete:"+id, in.IdempotencyKey, in, &out)
		if err != nil || found {
			return err
		}
		if err = mutableClass(lesson, in.Revision); err != nil {
			return err
		}
		if now.Before(lesson.StartsAt) {
			return invalid("class completion opens when the class starts")
		}
		people, err := classParticipants(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = checkStudentScope(ctx, tx, actor, people, "teaching.manage"); err != nil {
			return err
		}
		if len(in.Attendance) != len(people) {
			return invalid("resolve attendance for every student")
		}
		for _, person := range people {
			status, ok := in.Attendance[person.studentID]
			if !ok {
				return invalid("resolve attendance for every student")
			}
			_, err = tx.Exec(ctx, `INSERT INTO attendance(id,class_session_id,student_profile_id,status,resolved_by,resolved_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid,$6) ON CONFLICT(class_session_id,student_profile_id) DO UPDATE SET status=EXCLUDED.status,resolved_by=EXCLUDED.resolved_by,resolved_at=EXCLUDED.resolved_at`, platformid.New(), id, person.studentID, status, actor, now)
			if err != nil {
				return err
			}
			if person.status != "reserved" {
				continue
			}
			next := "consumed"
			if status == "technical_failure" {
				next = "released"
				err = ledger(ctx, tx, actor, id, person, "resolve", entitlement.EntryKindReleased, entitlement.Delta{Available: 1, Reserved: -1}, "verified technical failure", now)
				if err == nil {
					err = makeUp(ctx, tx, id, person.studentID, "technical failure", now)
				}
			} else {
				err = ledger(ctx, tx, actor, id, person, "resolve", entitlement.EntryKindConsumed, entitlement.Delta{Reserved: -1, Consumed: 1}, "class completed", now)
			}
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE academy_class_participants SET status=$3 WHERE class_session_id=$1::uuid AND student_profile_id=$2::uuid`, id, person.studentID, next); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE class_sessions SET status='completed',lesson_notes=$2,revision=revision+1,updated_at=$3 WHERE id=$1::uuid`, id, in.Notes, now); err != nil {
			return err
		}
		if err = audit(ctx, tx, actor, "academy.class.completed", id, now, map[string]any{"attendance": in.Attendance}); err != nil {
			return err
		}
		out, err = loadClass(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = notifyClass(ctx, tx, id, "Academy class attendance recorded", now, false); err != nil {
			return err
		}
		return remember(ctx, tx, actor, "complete:"+id, in.IdempotencyKey, in, out, now)
	})
	return out, err
}
func makeUp(ctx context.Context, tx pgx.Tx, classID, student, reason string, now time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO academy_make_up_obligations(id,class_session_id,student_profile_id,reason,created_at) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5) ON CONFLICT(class_session_id,student_profile_id) DO NOTHING`, platformid.New(), classID, student, reason, now)
	return err
}
