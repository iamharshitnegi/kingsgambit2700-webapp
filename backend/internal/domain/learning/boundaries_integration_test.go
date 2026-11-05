//go:build integration

package learning

import (
	"context"
	"errors"
	"testing"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestAssignedFileAuthorizationHasNoPreAuthorizationPageLimit(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	other, file := platformid.New(), platformid.New()
	if _, err := f.pool.Exec(ctx, `INSERT INTO student_profiles(id,user_id,preferred_name,age_band,country,time_zone,locale,language,status,created_at,updated_at) VALUES($1::uuid,$2::uuid,'Other learner','8_12','IN','Asia/Kolkata','en','en','active',now(),now())`, other, f.outsider); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO learning_assignments(id,assignment_key,student_profile_id,title,instructions,assignment_type,version,status,authored_by,created_at,content) SELECT id,id,$2::uuid,'File study','Read','study',1,'assigned',$4::uuid,now(),jsonb_build_object('resources',jsonb_build_array(jsonb_build_object('fileId',$3::text))) FROM unnest($1::uuid[]) id`
	ids := make([]string, 1001)
	for i := range ids {
		ids[i] = platformid.New()
	}
	if _, err := f.pool.Exec(ctx, insert, ids, other, file, f.academic); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(ctx, insert, []string{platformid.New()}, f.profile, file, f.academic); err != nil {
		t.Fatal(err)
	}
	allowed, err := assignedFileAccess(ctx, f.pool, f.guardian, file)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed {
		t.Fatal("unrelated assignments hid rightful file access")
	}
	if _, err = f.pool.Exec(ctx, `UPDATE guardian_student_relationships SET status='revoked' WHERE guardian_user_id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	allowed, err = assignedFileAccess(ctx, f.pool, f.guardian, file)
	if err != nil {
		t.Fatal(err)
	}
	if allowed {
		t.Fatal("revoked guardian retained assigned file access")
	}
}

func TestBatchMessagesFollowCurrentEnrollmentDates(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	w := mustAction(t, f, f.coach, Command{Action: "createThread", BatchID: f.batch, Title: "Batch announcement", Body: "Prepare your lesson board."})
	thread := w.Threads[0]
	for _, window := range []string{"starts_at=now()-interval '2 days',ends_at=now()-interval '1 day'", "starts_at=now()+interval '1 day',ends_at=NULL"} {
		if _, err := f.pool.Exec(ctx, `UPDATE enrolments SET `+window+` WHERE student_profile_id=$1::uuid`, f.profile); err != nil {
			t.Fatal(err)
		}
		w, err := f.service.Workspace(ctx, f.guardian)
		if err != nil {
			t.Fatal(err)
		}
		if len(w.Threads) != 0 {
			t.Fatal("non-current enrollment exposes batch messages")
		}
		if _, err = f.service.Action(ctx, f.guardian, Command{Action: "readThread", ID: thread.ID}); !errors.Is(err, ErrDenied) {
			t.Fatalf("non-current guardian thread read: %v", err)
		}
		if _, err = f.service.Action(ctx, f.student, Command{Action: "readThread", ID: thread.ID}); !errors.Is(err, ErrDenied) {
			t.Fatalf("non-current learner thread read: %v", err)
		}
		var before, after int
		if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM learning_notifications WHERE recipient_id=$1::uuid`, f.guardian).Scan(&before); err != nil {
			t.Fatal(err)
		}
		mustAction(t, f, f.academic, Command{Action: "sendMessage", ID: thread.ID, Body: "This update is for the currently enrolled class."})
		if err = f.pool.QueryRow(ctx, `SELECT count(*) FROM learning_notifications WHERE recipient_id=$1::uuid`, f.guardian).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Fatal("announcement notified a non-current enrollment")
		}
	}
}

func TestEmptyBatchCannotBypassCoachSuspensionOrCommunicationDeny(t *testing.T) {
	for _, state := range []string{"suspended", "denied"} {
		t.Run(state, func(t *testing.T) {
			f := setup(t)
			ctx := context.Background()
			thread := platformid.New()
			if _, err := f.pool.Exec(ctx, `DELETE FROM enrolments WHERE batch_id=$1::uuid`, f.batch); err != nil {
				t.Fatal(err)
			}
			if _, err := f.pool.Exec(ctx, `INSERT INTO learning_threads(id,batch_id,title,created_by) VALUES($1::uuid,$2::uuid,'Empty batch announcement',$3::uuid)`, thread, f.batch, f.academic); err != nil {
				t.Fatal(err)
			}
			if state == "suspended" {
				if _, err := f.pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1::uuid`, f.coach); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,scope_type,effect,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'learning.communicate','global','deny','Communication paused',$3::uuid,now())`, platformid.New(), f.coach, f.academic); err != nil {
					t.Fatal(err)
				}
			}
			for _, command := range []Command{{Action: "createThread", BatchID: f.batch, Title: "Bypass attempt", Body: "Should be denied"}, {Action: "sendMessage", ID: thread, Body: "Should be denied"}} {
				if _, err := f.service.Action(ctx, f.coach, command); !errors.Is(err, ErrDenied) {
					t.Fatalf("%s bypassed %s in empty batch: %v", command.Action, state, err)
				}
			}
			var count int
			if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM learning_messages`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatal("denied empty-batch message was persisted")
			}
		})
	}
}
