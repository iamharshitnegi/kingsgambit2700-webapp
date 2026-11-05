//go:build integration

package learning

import (
	"errors"
	"testing"

	"github.com/kingsgambit2700/platform/backend/internal/platform/idempotency"
)

func TestActionRequestReplayReturnsCurrentWorkspace(t *testing.T) {
	f := setup(t)
	ctx := idempotency.WithKey(t.Context(), "create-homework")
	command := Command{Action: "createAssignment", StudentIDs: []string{f.profile}, Title: "Review centre control", Description: "Write down two candidate moves", Kind: "study", Status: "assigned"}
	first, err := f.service.Action(ctx, f.coach, command)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Assignments) != 1 {
		t.Fatal("missing assignment")
	}
	a := first.Assignments[0]
	if _, err = f.service.Action(t.Context(), f.student, Command{Action: "submitAssignment", ID: a.ID, Version: a.Version, Answer: "I would develop a piece"}); err != nil {
		t.Fatal(err)
	}
	again, err := f.service.Action(ctx, f.coach, command)
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Assignments) != 1 || again.Assignments[0].ID != a.ID || len(again.Assignments[0].Attempts) != 1 {
		t.Fatal("replay duplicated assignment or returned stale workspace")
	}
	command.Title = "Changed request"
	if _, err = f.service.Action(ctx, f.coach, command); !errors.Is(err, ErrConflict) {
		t.Fatalf("payload mismatch=%v", err)
	}
	var rows int
	if err = f.pool.QueryRow(t.Context(), `SELECT count(*) FROM learning_assignments`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatal("creation duplicated")
	}
}
