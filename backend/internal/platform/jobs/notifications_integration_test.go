//go:build integration

package jobs

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kingsgambit2700/platform/backend/internal/domain/academy"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
	"github.com/kingsgambit2700/platform/backend/internal/platform/notification"
	"github.com/kingsgambit2700/platform/backend/internal/provider/fake"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

type notificationFixture struct {
	pool                            *pgxpool.Pool
	worker                          *DispatchNotificationWorker
	sender                          *fake.Email
	root, guardian, student, family string
}

func newNotificationFixture(t *testing.T) notificationFixture {
	t.Helper()
	database := os.Getenv("KG_TEST_DATABASE_URL")
	if database == "" {
		t.Skip("KG_TEST_DATABASE_URL not set")
	}
	pool := integrationtest.OpenMigratedSchema(t, database)
	root := platformid.New()
	if _, err := pool.Exec(t.Context(), `INSERT INTO users(id,display_name,status,created_at,updated_at) VALUES($1::uuid,'Owner','active',now(),now());`, root); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,'super_admin',now())`, platformid.New(), root); err != nil {
		t.Fatal(err)
	}
	repo, err := academy.NewPostgresRepository(pool)
	if err != nil {
		t.Fatal(err)
	}
	service, err := academy.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	student, err := service.CreateStudent(t.Context(), root, academy.StudentInput{Name: "Child", Country: "IN", AgeBand: "8_12", TimeZone: "UTC", GuardianEmail: "family@example.test", GuardianName: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	var guardian string
	if err = pool.QueryRow(t.Context(), `SELECT id::text FROM users WHERE email='family@example.test'`).Scan(&guardian); err != nil {
		t.Fatal(err)
	}
	sender := &fake.Email{}
	return notificationFixture{pool: pool, worker: &DispatchNotificationWorker{pool: pool, sender: sender, webURL: "https://academy.example.test", providerName: "fake"}, sender: sender, root: root, guardian: guardian, student: student.ID, family: student.FamilyID}
}
func (f notificationFixture) enqueue(t *testing.T, message notification.Message) string {
	t.Helper()
	if message.RecipientID == "" {
		message.RecipientID = f.guardian
	}
	if message.EventKey == "" {
		message.EventKey = platformid.New()
	}
	if message.Title == "" {
		message.Title = "New academy homework"
	}
	if message.Section == "" {
		message.Section = "homework"
	}
	if message.StudentID == "" {
		message.StudentID = f.student
	}
	if message.GuardianPermission == "" {
		message.GuardianPermission = "view_progress"
	}
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(t.Context())
	if err = notification.EnqueueEmail(t.Context(), tx, message); err != nil {
		t.Fatal(err)
	}
	if err = notification.EnqueueEmail(t.Context(), tx, message); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	var id string
	if err = f.pool.QueryRow(t.Context(), `SELECT id::text FROM notifications WHERE deduplication_key LIKE $1`, message.EventKey+":%").Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}
func TestNotifications_DurableRollbackDispatchAndRetry(t *testing.T) {
	f := newNotificationFixture(t)
	ctx := t.Context()
	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = notification.EnqueueEmail(ctx, tx, notification.Message{RecipientID: f.guardian, Title: "Rolled back", EventKey: "rollback"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM notifications`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("rollback leaked outbox event")
	}
	id := f.enqueue(t, notification.Message{Title: "New homework <script>", EventKey: "delivered"})
	if err = f.worker.dispatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err = f.worker.dispatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(f.sender.Messages) != 1 || !strings.Contains(f.sender.Messages[0].TextBody, "/guardian/homework") || strings.Contains(f.sender.Messages[0].HTMLBody, "<script>") {
		t.Fatalf("delivery=%+v", f.sender.Messages)
	}
	f.sender.Fail = errors.New("provider unavailable")
	retry := f.enqueue(t, notification.Message{EventKey: "retry"})
	if err = f.worker.dispatch(ctx, retry); err == nil {
		t.Fatal("failed provider reported success")
	}
	var status string
	if err = f.pool.QueryRow(ctx, `SELECT status FROM notification_delivery_attempts WHERE notification_id=$1::uuid`, retry).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "failed" {
		t.Fatal(status)
	}
	if err = f.worker.dispatch(ctx, retry); err == nil {
		t.Fatal("retry did not wait for scheduled backoff")
	}
	f.sender.Fail = nil
	if _, err = f.pool.Exec(ctx, `UPDATE notifications SET scheduled_at=now() WHERE id=$1::uuid`, retry); err != nil {
		t.Fatal(err)
	}
	if err = f.worker.dispatch(ctx, retry); err != nil {
		t.Fatal(err)
	}
	if len(f.sender.Messages) != 2 {
		t.Fatal("retry never delivered")
	}
}
func TestNotifications_RecheckFamilyDenyAndRevokedAccess(t *testing.T) {
	f := newNotificationFixture(t)
	ctx := t.Context()
	id := f.enqueue(t, notification.Message{})
	grant := platformid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,scope_type,scope_id,effect,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'learning.view','family',$3::uuid,'deny','Do not share progress',$4::uuid,now())`, grant, f.guardian, f.family, f.root); err != nil {
		t.Fatal(err)
	}
	if err := f.worker.dispatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(f.sender.Messages) != 0 {
		t.Fatal("explicit family deny leaked email")
	}
	if _, err := f.pool.Exec(ctx, `UPDATE permission_grants SET revoked_at=now(),revoked_by=$2::uuid,revoked_reason='test regrant' WHERE id=$1::uuid`, grant, f.root); err != nil {
		t.Fatal(err)
	}
	id = f.enqueue(t, notification.Message{})
	if _, err := f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET permissions=ARRAY['view_profile'] WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	if err := f.worker.dispatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(f.sender.Messages) != 0 {
		t.Fatal("revoked progress scope leaked email")
	}
}
func TestNotifications_SuppressStaleClassAndCompletedHomework(t *testing.T) {
	f := newNotificationFixture(t)
	ctx := t.Context()
	class := platformid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO class_sessions(id,kind,starts_at,ends_at,source_time_zone,source_local_start,join_opens_at,status,revision,created_at,updated_at) VALUES($1::uuid,'regular',now()+interval '1 hour',now()+interval '2 hours','UTC',now()+interval '1 hour',now(),'scheduled',1,now(),now())`, class); err != nil {
		t.Fatal(err)
	}
	id := f.enqueue(t, notification.Message{Section: "calendar", GuardianPermission: "view_schedule", ClassID: class, ClassRevision: 0, RequireUpcoming: true})
	if err := f.worker.dispatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(f.sender.Messages) != 0 {
		t.Fatal("old schedule reminder delivered")
	}
	due := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	assignment := platformid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO learning_assignments(id,assignment_key,student_profile_id,title,instructions,assignment_type,version,status,due_at,authored_by,created_at) VALUES($1::uuid,$1::uuid,$2::uuid,'Study','Study the position','study',1,'assigned',$3,$4::uuid,now())`, assignment, f.student, due, f.root); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, `INSERT INTO assignment_attempts(id,assignment_id,student_profile_id,attempt_number,status,created_at) VALUES($1::uuid,$2::uuid,$3::uuid,1,'submitted',now())`, platformid.New(), assignment, f.student); err != nil {
		t.Fatal(err)
	}
	id = f.enqueue(t, notification.Message{AssignmentID: assignment, AssignmentDueAt: &due})
	if err := f.worker.dispatch(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(f.sender.Messages) != 0 {
		t.Fatal("submitted homework received due reminder")
	}
}

func TestNotifications_OutboxQueuesRiverWorkOnce(t *testing.T) {
	f := newNotificationFixture(t)
	ctx := t.Context()
	f.enqueue(t, notification.Message{})
	var schema string
	if err := f.pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatal(err)
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(f.pool), &rivermigrate.Config{Schema: schema, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(f.pool, slog.New(slog.NewTextHandler(io.Discard, nil)), f.sender, "https://academy.example.test", "fake")
	if err != nil {
		t.Fatal(err)
	}
	sweeper := NotificationOutboxWorker{pool: f.pool, client: client}
	if err = sweeper.Work(ctx, nil); err != nil {
		t.Fatal(err)
	}
	if err = sweeper.Work(ctx, nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind='dispatch_notification'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("duplicate River jobs=%d", count)
	}
}
