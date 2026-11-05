package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
	"github.com/kingsgambit2700/platform/backend/internal/provider"
	"github.com/riverqueue/river"
)

type NotificationOutboxArgs struct{}

func (NotificationOutboxArgs) Kind() string { return "notification_outbox" }

type NotificationOutboxWorker struct {
	river.WorkerDefaults[NotificationOutboxArgs]
	pool   *pgxpool.Pool
	client *river.Client[pgx.Tx]
}

func (w *NotificationOutboxWorker) Work(ctx context.Context, _ *river.Job[NotificationOutboxArgs]) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, `SELECT n.id::text FROM notifications n WHERE channel='email' AND template_key='academy_event' AND cancelled_at IS NULL AND scheduled_at<=now() AND NOT EXISTS(SELECT 1 FROM notification_delivery_attempts a WHERE a.notification_id=n.id AND a.status IN('sent','delivered','suppressed')) ORDER BY scheduled_at LIMIT 200 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, err = w.client.InsertTx(ctx, tx, DispatchNotificationArgs{NotificationID: id}, &river.InsertOpts{UniqueOpts: river.UniqueOpts{ByArgs: true}}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (w *DispatchNotificationWorker) dispatch(ctx context.Context, id string) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var raw []byte
	var recipient, address string
	var at time.Time
	var cancelled, done bool
	err = tx.QueryRow(ctx, `SELECT render_context,COALESCE(recipient_user_id::text,''),COALESCE(recipient_address,''),scheduled_at,cancelled_at IS NOT NULL,EXISTS(SELECT 1 FROM notification_delivery_attempts WHERE notification_id=n.id AND status IN('sent','delivered','suppressed')) FROM notifications n WHERE id=$1::uuid AND template_key='academy_event' FOR UPDATE`, id).Scan(&raw, &recipient, &address, &at, &cancelled, &done)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if cancelled || done {
		return nil
	}
	if time.Now().Before(at) {
		return river.JobSnooze(time.Until(at))
	}
	var message notification.Message
	if err = json.Unmarshal(raw, &message); err != nil {
		return fmt.Errorf("jobs: invalid notification context: %w", err)
	}
	home := "/free-trial"
	eligible := true
	if recipient != "" {
		var roles []string
		err = tx.QueryRow(ctx, `SELECT COALESCE(email::text,''),status='active',ARRAY(SELECT role FROM role_grants WHERE user_id=u.id AND revoked_at IS NULL) FROM users u WHERE id=$1::uuid`, recipient).Scan(&address, &eligible, &roles)
		if err != nil {
			return err
		}
		switch {
		case slices.Contains(roles, "super_admin") || slices.Contains(roles, "operations_admin") || slices.Contains(roles, "academic_manager") || slices.Contains(roles, "finance_admin"):
			home = "/admin"
		case slices.Contains(roles, "coach"):
			home = "/coach"
		case slices.Contains(roles, "guardian"):
			home = "/guardian"
		case slices.Contains(roles, "student"):
			home = "/student"
		default:
			eligible = false
		}
		if message.StudentID != "" && eligible {
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM student_profiles WHERE id=$2::uuid AND user_id=$1::uuid) OR EXISTS(SELECT 1 FROM guardian_student_relationships WHERE guardian_user_id=$1::uuid AND student_profile_id=$2::uuid AND status='active' AND $3=ANY(permissions)) OR EXISTS(SELECT 1 FROM enrolments e JOIN batches b ON b.id=e.batch_id JOIN role_grants g ON g.user_id=b.coach_user_id AND g.role='coach' AND g.revoked_at IS NULL WHERE e.student_profile_id=$2::uuid AND e.status='active' AND e.starts_at<=now() AND (e.ends_at IS NULL OR e.ends_at>now()) AND b.coach_user_id=$1::uuid AND b.status='active') OR EXISTS(SELECT 1 FROM role_grants WHERE user_id=$1::uuid AND role IN('super_admin','academic_manager','operations_admin') AND revoked_at IS NULL)`, recipient, message.StudentID, message.GuardianPermission).Scan(&eligible); err != nil {
				return err
			}
		}
		var denied bool
		permission := "learning.view"
		if message.Section == "messages" {
			permission = "learning.communicate"
		} else if message.GuardianPermission == "view_schedule" || message.Section == "calendar" {
			permission = "schedule.view"
		}
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM permission_grants g WHERE g.user_id=$1::uuid AND g.permission_code=$3 AND effect='deny' AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now()) AND (scope_type='global' OR(scope_type='student' AND scope_id=NULLIF($2,'')::uuid) OR(scope_type='family' AND scope_id IN(SELECT family_id FROM guardian_student_relationships WHERE student_profile_id=NULLIF($2,'')::uuid AND status='active'))))`, recipient, message.StudentID, permission).Scan(&denied); err != nil {
			return err
		}
		eligible = eligible && !denied
	}
	if message.ClassID != "" && eligible {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM class_sessions WHERE id=$1::uuid AND revision=$2 AND (NOT $3 OR(status IN('scheduled','join_open','live') AND ends_at>now() AND ($4='' OR EXISTS(SELECT 1 FROM academy_class_participants WHERE class_session_id=$1::uuid AND student_profile_id=NULLIF($4,'')::uuid AND status='reserved')))))`, message.ClassID, message.ClassRevision, message.RequireUpcoming, message.StudentID).Scan(&eligible); err != nil {
			return err
		}
	}
	if message.TrialID != "" && eligible {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM trial_bookings WHERE id=$1::uuid AND slot_id=$2::uuid AND (NOT $3 OR(status='confirmed' AND ends_at>now())))`, message.TrialID, message.TrialSlotID, message.RequireUpcoming).Scan(&eligible); err != nil {
			return err
		}
	}
	if message.AssignmentID != "" && eligible {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM learning_assignments a WHERE id=$1::uuid AND status='assigned' AND due_at=$2 AND due_at>now() AND COALESCE((SELECT status FROM assignment_attempts WHERE assignment_id=a.id ORDER BY attempt_number DESC LIMIT 1),'started') NOT IN('submitted','reviewed'))`, message.AssignmentID, message.AssignmentDueAt).Scan(&eligible); err != nil {
			return err
		}
	}
	status, providerID, errorCode := "suppressed", "", ""
	var sendErr error
	if eligible && address != "" {
		if recipient != "" && slices.Contains([]string{"homework", "calendar", "progress", "messages", "library", "games", "notifications", "settings", "operations"}, message.Section) {
			home += "/" + message.Section
		}
		link := strings.TrimRight(w.webURL, "/") + home
		sendCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		providerID, sendErr = w.sender.Send(sendCtx, provider.EmailMessage{To: address, Subject: message.Title, TextBody: message.Title + "\n\nOpen your academy portal for details: " + link, HTMLBody: "<p>" + html.EscapeString(message.Title) + "</p><p><a href=\"" + html.EscapeString(link) + "\">Open your academy portal</a></p>", Tags: map[string]string{"notification_id": id}})
		cancel()
		status = "sent"
		if sendErr != nil {
			status = "failed"
			errorCode = "provider_send_failed"
			providerID = ""
		}
	}
	var attempt int
	if err = tx.QueryRow(ctx, `SELECT COALESCE(max(attempt_number),0)+1 FROM notification_delivery_attempts WHERE notification_id=$1::uuid`, id).Scan(&attempt); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO notification_delivery_attempts(id,notification_id,attempt_number,provider,provider_message_id,status,error_code,occurred_at) VALUES($1::uuid,$2::uuid,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),now())`, platformid.New(), id, attempt, w.providerName, providerID, status, errorCode); err != nil {
		return err
	}
	if sendErr != nil {
		delay := time.Duration(min(attempt, 60)) * time.Minute
		if _, err = tx.Exec(ctx, `UPDATE notifications SET scheduled_at=$2 WHERE id=$1::uuid`, id, time.Now().Add(delay)); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if sendErr != nil {
		return errors.New("jobs: notification delivery failed; retry scheduled")
	}
	return nil
}
