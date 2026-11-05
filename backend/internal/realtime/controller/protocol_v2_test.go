package controller

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/classroom"
	platformid "github.com/kingsgambit2700/platform/backend/internal/platform/id"
)

func TestLessonCommandsRequireV2AndCoach(t *testing.T) {
	repository, _ := classroom.NewMemoryRepository(20)
	service, _ := classroom.NewService(repository, classroom.ChessRules{})
	sessionID := platformid.New()
	coach := classroom.Actor{ID: platformid.New(), Role: classroom.ActorRoleCoach}
	_, err := service.Create(t.Context(), classroom.NewStateParams{SessionID: sessionID, FEN: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1", Mode: classroom.ModeDemonstration})
	if err != nil {
		t.Fatal(err)
	}
	handler := &Controller{service: service}
	command := command{Type: "countdown", CommandID: platformid.New(), Seconds: 90}
	frame, accepted := handler.execute(t.Context(), sessionID, coach, command)
	if accepted != nil || frame.(fiber.Map)["code"] != "not_implemented" {
		t.Fatalf("legacy command = %v", frame)
	}
	command.Version = 2
	frame, accepted = handler.execute(t.Context(), sessionID, classroom.Actor{ID: platformid.New(), Role: classroom.ActorRoleStudent}, command)
	if accepted != nil || frame.(fiber.Map)["code"] != "unauthorized" {
		t.Fatalf("student tool = %v", frame)
	}
	_, accepted = handler.execute(t.Context(), sessionID, coach, command)
	if accepted == nil || accepted.State.Revision != 1 || accepted.State.Lesson.CountdownEndsAt == nil {
		t.Fatal("coach timer not accepted")
	}
	for _, private := range []string{"privateResponse", "exerciseAssign", "exerciseReveal"} {
		frame, accepted = handler.execute(t.Context(), sessionID, coach, commandWithType(command, private))
		if accepted != nil || frame.(fiber.Map)["code"] != "not_implemented" {
			t.Fatalf("%s exposed private content", private)
		}
	}
}
func commandWithType(value command, kind string) command { value.Type = kind; return value }

func TestV2EnvelopeRequiresRevisionAndRejectsUnrelatedFields(t *testing.T) {
	for _, item := range []struct {
		raw   string
		valid bool
	}{
		{`{"type":"countdown","commandId":"id","expectedRevision":0,"seconds":10}`, true},
		{`{"type":"countdown","commandId":"id","seconds":10}`, false},
		{`{"type":"undo","commandId":"id","expectedRevision":null}`, false},
		{`{"type":"undo","commandId":"id","expectedRevision":0,"payload":{}}`, false},
		{`{"type":"loadPosition","commandId":"id","expectedRevision":0,"fen":"x","moves":["e2e4"]}`, false},
	} {
		var command command
		if err := json.Unmarshal([]byte(item.raw), &command); err != nil {
			t.Fatal(err)
		}
		if got := validV2Envelope([]byte(item.raw), command); got != item.valid {
			t.Fatalf("envelope = %v for %s", got, item.raw)
		}
	}
}

func TestV1RecoveryProjectsToolsToCanonicalSnapshot(t *testing.T) {
	before := classroom.State{SessionID: platformid.New(), FEN: "before", Moves: []string{}, Mode: classroom.ModeDemonstration, Status: classroom.BoardStatusOpen}
	after := before
	after.Revision = 1
	after.Lesson = classroom.LessonState{CountdownLabel: "Private schema extension"}
	event := classroom.Event{ID: platformid.New(), SessionID: before.SessionID, Revision: 1, Kind: classroom.EventKindLessonUpdated, ActorID: platformid.New(), OccurredAt: time.Now(), State: &after}
	frame := legacyFrame(fiber.Map{"type": "snapshot", "protocolVersion": 2, "snapshot": before, "events": []classroom.Event{event}}).(fiber.Map)
	if frame["protocolVersion"] != 1 || len(frame["events"].([]classroom.Event)) != 0 {
		t.Fatal("v1 tail was not folded")
	}
	snapshot := frame["snapshot"].(fiber.Map)
	if snapshot["revision"] != uint64(1) || snapshot["lesson"] != nil {
		t.Fatalf("unexpected v1 shape: %v", snapshot)
	}
}
