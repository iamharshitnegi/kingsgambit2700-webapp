package learning

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
)

func remindAssignment(ctx context.Context, tx pgx.Tx, id, student string, due *time.Time) error {
	if due == nil || !due.After(nowUTC()) {
		return nil
	}
	at := due.Add(-24 * time.Hour)
	if at.Before(nowUTC()) {
		at = nowUTC()
	}
	rows, err := tx.Query(ctx, `SELECT user_id::text FROM student_profiles WHERE id=$1::uuid UNION SELECT guardian_user_id::text FROM guardian_student_relationships WHERE student_profile_id=$1::uuid AND status='active' AND 'view_progress'=ANY(permissions)`, student)
	if err != nil {
		return err
	}
	recipients := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		recipients = append(recipients, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, recipient := range recipients {
		if err = notification.EnqueueEmail(ctx, tx, notification.Message{RecipientID: recipient, StudentID: student, Title: "Academy homework is due soon", Section: "homework", EventKey: "homework-due:" + id + ":" + due.UTC().Format(time.RFC3339Nano), At: at, GuardianPermission: "view_progress", AssignmentID: id, AssignmentDueAt: due}); err != nil {
			return err
		}
	}
	return nil
}
