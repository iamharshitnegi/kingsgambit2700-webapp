//go:build integration

package learning

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestLegacyUUIDAccountsLoadWorkspaceAndTeach(t *testing.T) {
	legacyID := func() string { id := platformid.New(); return id[:14] + "4" + id[15:] }
	f := setupWithIDs(t, legacyID)
	ctx := context.Background()
	actors := []string{f.coach, f.student, f.guardian, f.academic}
	for _, role := range []string{"super_admin", "operations_admin", "finance_admin"} {
		actor := legacyID()
		if _, err := f.pool.Exec(ctx, `INSERT INTO users(id,display_name,status,created_at,updated_at) VALUES($1::uuid,$2,'active',now(),now())`, actor, role); err != nil {
			t.Fatal(err)
		}
		if _, err := f.pool.Exec(ctx, `INSERT INTO role_grants(id,user_id,role,granted_at) VALUES($1::uuid,$2::uuid,$3,now())`, platformid.New(), actor, role); err != nil {
			t.Fatal(err)
		}
		actors = append(actors, actor)
	}
	for _, actor := range actors {
		if _, err := f.service.Workspace(ctx, actor); err != nil {
			t.Fatalf("existing UUIDv4 account cannot load dashboard: %v", err)
		}
	}
	store := &memoryFiles{objects: map[string][]byte{}}
	if err := f.service.ConfigureFiles(store, resultScanner{clean: true}, 20<<20); err != nil {
		t.Fatal(err)
	}
	uploaded, err := f.service.UploadFile(ctx, f.coach, UploadCommand{Filename: "study.pgn", Title: "Central control", Visibility: "personal"}, bytes.NewBufferString("1. e4 e5 *"))
	if err != nil {
		t.Fatal(err)
	}
	_, body, err := f.service.DownloadFile(ctx, f.coach, uploaded.File.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.Copy(io.Discard, body); err != nil {
		t.Fatal(err)
	}
	body.Close()
	command := Command{Action: "createAssignment", StudentIDs: []string{f.profile}, BatchID: f.batch, Title: "Explain central control", Description: "Explain your first move.", Kind: "study", Status: "assigned"}
	if _, err = f.service.Action(ctx, f.outsider, command); !errors.Is(err, ErrDenied) {
		t.Fatalf("unassigned legacy coach: %v", err)
	}
	w := mustAction(t, f, f.coach, command)
	a := w.Assignments[0]
	w = mustAction(t, f, f.student, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version, Answer: "Control the central squares."})
	a = w.Assignments[0]
	mustAction(t, f, f.coach, Command{Action: "reviewAssignment", ID: a.ID, Version: a.Version, Feedback: "Good reasoning."})
	if _, err = f.pool.Exec(ctx, `INSERT INTO permission_grants(id,user_id,permission_code,scope_type,scope_id,effect,reason,granted_by,granted_at) VALUES($1::uuid,$2::uuid,'learning.manage','student',$3::uuid,'deny','Test deny',$4::uuid,now())`, platformid.New(), f.coach, f.profile, f.academic); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Action(ctx, f.coach, command); !errors.Is(err, ErrDenied) {
		t.Fatalf("explicit deny ignored for legacy account: %v", err)
	}
	if _, err = f.pool.Exec(ctx, `UPDATE users SET status='suspended' WHERE id=$1::uuid`, f.guardian); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.Workspace(ctx, f.guardian); !errors.Is(err, ErrDenied) {
		t.Fatalf("suspended legacy account: %v", err)
	}
}
