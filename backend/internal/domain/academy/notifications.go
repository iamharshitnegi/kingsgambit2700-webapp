package academy

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
)

func notifyClass(ctx context.Context, tx pgx.Tx, id, title string, now time.Time, reminders bool) error {
	lesson, err := loadClass(ctx, tx, id)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT DISTINCT recipient,student FROM (
 SELECT a.user_id::text AS recipient,COALESCE(s.id::text,'') AS student FROM session_assignments a LEFT JOIN student_profiles s ON s.user_id=a.user_id WHERE a.class_session_id=$1::uuid
 UNION SELECT g.guardian_user_id::text,p.student_profile_id::text FROM academy_class_participants p JOIN guardian_student_relationships g ON g.student_profile_id=p.student_profile_id WHERE p.class_session_id=$1::uuid AND g.status='active' AND 'view_schedule'=ANY(g.permissions)) recipients`, id)
	if err != nil {
		return err
	}
	type recipient struct{ id, student string }
	recipients := []recipient{}
	for rows.Next() {
		var next recipient
		if err = rows.Scan(&next.id, &next.student); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, next)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, recipient := range recipients {
		var home string
		if err = tx.QueryRow(ctx, `SELECT CASE WHEN EXISTS(SELECT 1 FROM role_grants WHERE user_id=$1::uuid AND revoked_at IS NULL AND role IN('super_admin','operations_admin','academic_manager','finance_admin')) THEN '/admin' WHEN EXISTS(SELECT 1 FROM role_grants WHERE user_id=$1::uuid AND revoked_at IS NULL AND role='coach') THEN '/coach' WHEN EXISTS(SELECT 1 FROM role_grants WHERE user_id=$1::uuid AND revoked_at IS NULL AND role='guardian') THEN '/guardian' ELSE '/student' END`, recipient.id).Scan(&home); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO learning_notifications(id,recipient_id,student_id,title,href) VALUES($1::uuid,$2::uuid,NULLIF($3,'')::uuid,$4,$5)`, platformid.New(), recipient.id, recipient.student, title, home+"/calendar"); err != nil {
			return err
		}
		message := notification.Message{RecipientID: recipient.id, StudentID: recipient.student, Title: title, Section: "calendar", EventKey: fmt.Sprintf("class:%s:%d:%s", id, lesson.Revision, recipient.student), At: now, GuardianPermission: "view_schedule", ClassID: id, ClassRevision: lesson.Revision}
		if err = notification.EnqueueEmail(ctx, tx, message); err != nil {
			return err
		}
		if reminders {
			message.Title = "Your academy class starts soon"
			message.RequireUpcoming = true
			for _, lead := range []time.Duration{24 * time.Hour, time.Hour} {
				at := lesson.StartsAt.Add(-lead)
				if !at.After(now) {
					continue
				}
				message.At = at
				message.EventKey = fmt.Sprintf("class-reminder:%s:%d:%s:%s", id, lesson.Revision, recipient.student, lead)
				if err = notification.EnqueueEmail(ctx, tx, message); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
