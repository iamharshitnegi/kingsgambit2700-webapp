//go:build integration

package classroom

import (
	"os"
	"reflect"
	"testing"
	"time"

	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
	"github.com/kingsgambit2700/platform/backend/internal/platform/integrationtest"
)

func TestPostgresLessonToolsSurviveReloadAndSnapshotReplay(t *testing.T) {
	databaseURL := os.Getenv("KG_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("KG_TEST_DATABASE_URL required")
	}
	pool := integrationtest.OpenMigratedSchema(t, databaseURL)
	actorID, sessionID := platformid.New(), platformid.New()
	_, err := pool.Exec(t.Context(), `INSERT INTO users(id,email,display_name,status,created_at,updated_at) VALUES($1::uuid,$2,'Coach','active',now(),now())`, actorID, actorID+"@example.invalid")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(t.Context(), `INSERT INTO class_sessions(id,kind,starts_at,ends_at,source_time_zone,source_local_start,join_opens_at,status,created_at,updated_at) VALUES($1::uuid,'regular',now(),now()+interval '1 hour','UTC',now() AT TIME ZONE 'UTC',now(),'live',now(),now())`, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	repository, _ := NewPostgresRepository(pool, 3)
	service, _ := NewService(repository, ChessRules{})
	state, err := service.Create(t.Context(), NewStateParams{SessionID: sessionID, FEN: standardFEN, Mode: ModeDemonstration})
	if err != nil {
		t.Fatal(err)
	}
	coach := Actor{ID: actorID, Role: ActorRoleCoach}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, command := range []LessonCommand{
		{Operation: "annotation", Annotations: []Annotation{{StartSquare: "e2", EndSquare: "e4", Color: "#123456"}}},
		{Operation: "checkpoint", Title: "Opening idea"},
		{Operation: "countdown", Seconds: 120, Label: "Calculate"},
		{Operation: "annotation", Annotations: []Annotation{}},
	} {
		command.EventID = platformid.New()
		command.SessionID = sessionID
		command.Actor = coach
		command.ExpectedRevision = state.Revision
		command.OccurredAt = now
		accepted, err := service.UpdateLesson(t.Context(), command)
		if err != nil {
			t.Fatal(err)
		}
		state = accepted.State
	}
	reloaded, err := repository.State(t.Context(), sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(reloaded, state) {
		t.Fatalf("persisted lesson differs:\n%+v\n%+v", reloaded, state)
	}
	recovery, err := service.Reconnect(t.Context(), sessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Snapshot.Revision != 3 || len(recovery.Events) != 1 || recovery.Events[0].State == nil {
		t.Fatalf("invalid recovery: %+v", recovery)
	}
	if !reflect.DeepEqual(*recovery.Events[0].State, state) {
		t.Fatal("event replay lost lesson tools")
	}
	restored, err := service.UpdateLesson(t.Context(), LessonCommand{EventID: platformid.New(), SessionID: sessionID, Actor: coach, ExpectedRevision: state.Revision, OccurredAt: now, Operation: "restoreCheckpoint", CheckpointID: state.Lesson.Checkpoints[0].ID})
	if err != nil || len(restored.State.Lesson.Annotations) != 1 {
		t.Fatalf("restore persisted checkpoint: %+v %v", restored, err)
	}
}
