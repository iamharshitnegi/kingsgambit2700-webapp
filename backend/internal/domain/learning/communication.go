package learning

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
)

func batchStudents(ctx context.Context, q rowsQuerier, batch string) ([]string, error) {
	rows, err := q.Query(ctx, `SELECT DISTINCT e.student_profile_id::text FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.batch_id=$1::uuid AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.status='active' ORDER BY e.student_profile_id::text`, batch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func threadAccess(ctx context.Context, q interface {
	querier
	rowsQuerier
}, actor, student, batch string, send bool) error {
	if student != "" {
		return authorizeStudent(ctx, q, actor, student, "learning.communicate", "communicate_with_academy", true)
	}
	ids, err := batchStudents(ctx, q, batch)
	if err != nil {
		return err
	}
	if send {
		allowed, checkErr := permission(ctx, q, actor, "learning.communicate")
		if checkErr != nil {
			return checkErr
		}
		var assigned bool
		if err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM batches b JOIN role_grants r ON r.user_id=b.coach_user_id JOIN users u ON u.id=r.user_id WHERE b.id=$1::uuid AND b.status='active' AND b.coach_user_id=$2::uuid AND r.role='coach' AND r.revoked_at IS NULL AND u.status='active') AND NOT EXISTS(SELECT 1 FROM permission_grants WHERE user_id=$2::uuid AND permission_code='learning.communicate' AND effect='deny' AND scope_type='global' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now()))`, batch, actor).Scan(&assigned); err != nil {
			return err
		}
		if !allowed && !assigned {
			return ErrDenied
		}
		for _, id := range ids {
			if err = authorizeStudent(ctx, q, actor, id, "learning.communicate", "", false); err != nil {
				return err
			}
		}
		return nil
	}
	if allowed, checkErr := permission(ctx, q, actor, "learning.communicate"); checkErr != nil {
		return checkErr
	} else if allowed {
		return nil
	}
	for _, id := range ids {
		if err = authorizeStudent(ctx, q, actor, id, "learning.communicate", "communicate_with_academy", true); err == nil {
			return nil
		} else if !errors.Is(err, ErrDenied) {
			return err
		}
	}
	return ErrDenied
}
func (s *Service) threadAction(ctx context.Context, tx pgx.Tx, actor string, c Command) error {
	id := c.ID
	student, batch := c.StudentID, c.BatchID
	if c.Action == "createThread" {
		if (student == "") == (batch == "") {
			return invalid("choose one learner or one batch")
		}
		if !required(c.Title, 200) || !required(c.Body, 10000) {
			return invalid("thread title and first message are required")
		}
		if err := threadAccess(ctx, tx, actor, student, batch, true); err != nil {
			return err
		}
		id = platformid.New()
		if _, err := tx.Exec(ctx, `INSERT INTO learning_threads(id,student_id,batch_id,title,created_by) VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5::uuid)`, id, nullID(student), nullID(batch), c.Title, actor); err != nil {
			return err
		}
	} else {
		err := tx.QueryRow(ctx, `SELECT COALESCE(student_id::text,''),COALESCE(batch_id::text,'') FROM learning_threads WHERE id=$1::uuid FOR UPDATE`, id).Scan(&student, &batch)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err = threadAccess(ctx, tx, actor, student, batch, c.Action == "sendMessage"); err != nil {
			return err
		}
	}
	if c.Action != "readThread" {
		if !required(c.Body, 10000) {
			return invalid("message must contain 1–10000 characters")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO learning_messages(id,thread_id,actor_id,body) VALUES($1::uuid,$2::uuid,$3::uuid,$4)`, platformid.New(), id, actor, c.Body); err != nil {
			return err
		}
		ids := []string{student}
		if batch != "" {
			var err error
			ids, err = batchStudents(ctx, tx, batch)
			if err != nil {
				return err
			}
		}
		for _, learner := range ids {
			rows, err := tx.Query(ctx, `SELECT user_id::text FROM student_profiles WHERE id=$1::uuid UNION SELECT guardian_user_id::text FROM guardian_student_relationships WHERE student_profile_id=$1::uuid AND status='active' AND 'communicate_with_academy'=ANY(permissions)
    UNION SELECT b.coach_user_id::text FROM enrolments e JOIN batches b ON b.id=e.batch_id WHERE e.student_profile_id=$1::uuid AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.status='active' AND b.coach_user_id IS NOT NULL`, learner)
			if err != nil {
				return err
			}
			recipients := []string{}
			for rows.Next() {
				var recipient string
				if err = rows.Scan(&recipient); err != nil {
					rows.Close()
					return err
				}
				recipients = append(recipients, recipient)
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
			for _, recipient := range recipients {
				if recipient == actor {
					continue
				}
				noticeID := platformid.New()
				if _, err = tx.Exec(ctx, `INSERT INTO learning_notifications(id,recipient_id,student_id,title,href) VALUES($1::uuid,$2::uuid,$3::uuid,'New academy message','/portal/messages')`, noticeID, recipient, learner); err != nil {
					return err
				}
				if err = notification.EnqueueEmail(ctx, tx, notification.Message{RecipientID: recipient, StudentID: learner, Title: "New academy message", Section: "messages", EventKey: noticeID, GuardianPermission: "communicate_with_academy"}); err != nil {
					return err
				}
			}
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO learning_thread_reads(thread_id,user_id,read_at) VALUES($1::uuid,$2::uuid,now()) ON CONFLICT(thread_id,user_id) DO UPDATE SET read_at=excluded.read_at`, id, actor)
	return err
}
