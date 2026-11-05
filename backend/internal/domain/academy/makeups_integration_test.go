//go:build integration

package academy

import (
	"errors"
	"sync"
	"testing"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestPostgresAcademy_ConcurrentMakeUpReplacement(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	input := f.classInput()
	input.Occurrences = 3
	classes, err := f.service.Schedule(ctx, f.root, input, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		if _, err = f.service.Cancel(ctx, f.root, classes.Classes[i].ID, CancellationInput{Reason: "Academy unavailable", IdempotencyKey: platformid.New()}, false); err != nil {
			t.Fatal(err)
		}
	}
	workspace, err := f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.MakeUps) != 2 {
		t.Fatal("expected two obligations")
	}
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, owed := range workspace.MakeUps {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			_, err := f.service.FulfillMakeUp(ctx, f.coach, id, MakeUpFulfillment{ReplacementClassID: classes.Classes[2].ID, IdempotencyKey: platformid.New()})
			results <- err
		}(owed.ID)
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("one replacement satisfied %d obligations", successes)
	}
}

func TestPostgresAcademy_HistoricalClassesRemainDiscoverable(t *testing.T) {
	f := newAcademyFixture(t)
	ctx := t.Context()
	created, err := f.service.Schedule(ctx, f.coach, f.classInput(), false)
	if err != nil {
		t.Fatal(err)
	}
	past := f.now.AddDate(0, -6, 0)
	if _, err = f.pool.Exec(ctx, `UPDATE class_sessions SET starts_at=$2,ends_at=$3,status='completed' WHERE id=$1::uuid`, created.Classes[0].ID, past, past.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	workspace, err := f.service.Workspace(ctx, f.coach)
	if err != nil {
		t.Fatal(err)
	}
	if len(workspace.Classes) != 1 || workspace.Classes[0].ID != created.Classes[0].ID {
		t.Fatal("historical class disappeared from calendar")
	}
}
