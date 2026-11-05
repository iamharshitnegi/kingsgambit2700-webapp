package ws_test

import (
	"bytes"
	"fmt"
	"os"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const testID = "01900000-0000-7000-8000-000000000099"

func TestClientFramesMatchSchema(t *testing.T) {
	schema := compile(t, "client-message.v1.schema.json")
	frames := []string{
		fmt.Sprintf(`{"type":"join","commandId":%q,"ticket":"1234567890123456789012345678901234567890"}`, testID),
		fmt.Sprintf(`{"type":"resume","commandId":%q,"afterRevision":2}`, testID),
		fmt.Sprintf(`{"type":"move","commandId":%q,"expectedRevision":2,"move":"e2e4"}`, testID),
		fmt.Sprintf(`{"type":"setMode","commandId":%q,"expectedRevision":2,"mode":"demonstration"}`, testID),
		fmt.Sprintf(`{"type":"loadPosition","commandId":%q,"expectedRevision":2,"fen":"8/8/8/8/8/8/8/K6k w - - 0 1","moves":[]}`, testID),
		fmt.Sprintf(`{"type":"closeBoard","commandId":%q,"expectedRevision":2}`, testID),
		fmt.Sprintf(`{"type":"ping","commandId":%q}`, testID),
	}
	for _, kind := range []string{"annotation", "undo", "checkpoint", "exerciseAssign", "privateResponse", "exerciseReveal", "presence", "providerState"} {
		frames = append(frames, fmt.Sprintf(`{"type":%q,"commandId":%q,"payload":{}}`, kind, testID))
	}
	validateFrames(t, schema, frames)
}

func TestServerFramesMatchSchema(t *testing.T) {
	schema := compile(t, "server-message.v1.schema.json")
	state := fmt.Sprintf(`{"sessionId":%q,"fen":"8/8/8/8/8/8/8/K6k w - - 0 1","moves":[],"mode":"demonstration","revision":0,"status":"open"}`, testID)
	frames := []string{
		fmt.Sprintf(`{"type":"snapshot","protocolVersion":1,"commandId":%q,"snapshot":%s,"events":[]}`, testID, state),
		fmt.Sprintf(`{"type":"accepted","commandId":%q,"operation":"ping"}`, testID),
		fmt.Sprintf(`{"type":"rejected","commandId":%q,"code":"not_implemented","detail":"disabled"}`, testID),
		fmt.Sprintf(`{"type":"revisionGap","sessionId":%q,"revision":1}`, testID),
		fmt.Sprintf(`{"type":"error","commandId":%q,"code":"invalid_message","detail":"invalid"}`, testID),
	}
	for _, kind := range []string{"presence", "privateResponse", "exerciseReveal", "providerState"} {
		frames = append(frames, fmt.Sprintf(`{"type":%q,"sessionId":%q,"revision":0,"payload":{}}`, kind, testID))
	}
	validateFrames(t, schema, frames)
}

func compile(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(name, document); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(name)
	if err != nil {
		t.Fatal(err)
	}
	return schema
}

func validateFrames(t *testing.T, schema *jsonschema.Schema, frames []string) {
	t.Helper()
	for _, frame := range frames {
		frame := frame
		t.Run(frame, func(t *testing.T) {
			instance, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(frame))
			if err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(instance); err != nil {
				t.Fatalf("frame does not match schema: %v", err)
			}
		})
	}
}

func TestV2LessonFramesMatchSchema(t *testing.T) {
	client := compile(t, "client-message.v2.schema.json")
	validateFrames(t, client, []string{
		fmt.Sprintf(`{"type":"annotation","commandId":%q,"expectedRevision":1,"annotations":[{"startSquare":"e2","endSquare":"e4","color":"#123456"}]}`, testID),
		fmt.Sprintf(`{"type":"undo","commandId":%q,"expectedRevision":1}`, testID),
		fmt.Sprintf(`{"type":"checkpoint","commandId":%q,"expectedRevision":1,"title":"Critical position"}`, testID),
		fmt.Sprintf(`{"type":"restoreCheckpoint","commandId":%q,"expectedRevision":1,"checkpointId":%q}`, testID, testID),
		fmt.Sprintf(`{"type":"deleteCheckpoint","commandId":%q,"expectedRevision":1,"checkpointId":%q}`, testID, testID),
		fmt.Sprintf(`{"type":"countdown","commandId":%q,"expectedRevision":1,"seconds":180,"label":"Calculate"}`, testID),
	})
	state := fmt.Sprintf(`{"sessionId":%q,"fen":"fen","moves":[],"mode":"demonstration","revision":1,"status":"open","lesson":{"initialFen":"fen","positions":["fen"],"annotations":[],"checkpoints":[],"countdownEndsAt":"2026-09-22T12:00:00Z","countdownLabel":"Calculate"}}`, testID)
	event := fmt.Sprintf(`{"id":%q,"sessionId":%q,"revision":1,"kind":"lesson_updated","actorId":%q,"occurredAt":"2026-09-22T11:57:00Z","state":%s}`, testID, testID, testID, state)
	validateFrames(t, compile(t, "server-message.v2.schema.json"), []string{fmt.Sprintf(`{"type":"accepted","commandId":%q,"state":%s,"event":%s}`, testID, state, event), fmt.Sprintf(`{"type":"snapshot","commandId":%q,"protocolVersion":2,"snapshot":%s,"events":[]}`, testID, state)})
	for _, invalid := range []string{
		fmt.Sprintf(`{"type":"countdown","commandId":%q,"seconds":10}`, testID),
		fmt.Sprintf(`{"type":"annotation","commandId":%q,"expectedRevision":1,"annotations":[{"startSquare":"z9","endSquare":"e4","color":"red"}]}`, testID),
	} {
		instance, err := jsonschema.UnmarshalJSON(bytes.NewBufferString(invalid))
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Validate(instance); err == nil {
			t.Fatalf("invalid v2 frame accepted: %s", invalid)
		}
	}
}
