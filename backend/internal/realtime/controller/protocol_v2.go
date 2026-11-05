package controller

import (
	"encoding/json"
	"github.com/gofiber/fiber/v3"
	"github.com/kingsgambit2700/platform/backend/internal/domain/classroom"
)

func disabledCommand(id string) fiber.Map {
	return fiber.Map{"type": "rejected", "commandId": id, "code": "not_implemented", "detail": "This feature is unavailable for this classroom protocol"}
}

func validV2Envelope(payload []byte, message command) bool {
	required := []string{"type", "commandId"}
	optional := []string{}
	switch message.Type {
	case "join":
		required = append(required, "ticket")
	case "resume":
		required = append(required, "afterRevision")
	case "ping":
	case "move":
		required = append(required, "expectedRevision", "move")
	case "setMode":
		required = append(required, "expectedRevision", "mode")
		optional = append(optional, "activeActorId")
	case "loadPosition":
		required = append(required, "expectedRevision", "fen", "moves")
		optional = append(optional, "initialFen")
		if len(message.Moves) > 0 && message.InitialFEN == "" {
			return false
		}
	case "closeBoard", "undo":
		required = append(required, "expectedRevision")
	case "annotation":
		required = append(required, "expectedRevision", "annotations")
	case "checkpoint":
		required = append(required, "expectedRevision", "title")
	case "restoreCheckpoint", "deleteCheckpoint":
		required = append(required, "expectedRevision", "checkpointId")
	case "countdown":
		required = append(required, "expectedRevision", "seconds")
		optional = append(optional, "label")
	case "exerciseAssign", "privateResponse", "exerciseReveal", "presence", "providerState":
		optional = append(optional, "expectedRevision", "payload")
	default:
		return false
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(payload, &fields) != nil {
		return false
	}
	for _, key := range required {
		value, exists := fields[key]
		if !exists || string(value) == "null" {
			return false
		}
		delete(fields, key)
	}
	for _, key := range optional {
		delete(fields, key)
	}
	return len(fields) == 0
}

func legacyState(state classroom.State) fiber.Map {
	value := fiber.Map{"sessionId": state.SessionID, "fen": state.FEN, "moves": state.Moves, "mode": state.Mode, "revision": state.Revision, "status": state.Status}
	if state.ActiveActorID != "" {
		value["activeActorId"] = state.ActiveActorID
	}
	return value
}

// v1 peers continue receiving the original shape. Fold recovery into one
// snapshot because v1 clients cannot interpret the new lesson event kinds.
func legacyFrame(frame any) any {
	original, ok := frame.(fiber.Map)
	if !ok {
		return frame
	}
	value := make(fiber.Map, len(original))
	for key, item := range original {
		value[key] = item
	}
	if value["type"] == "snapshot" {
		snapshot, ok := value["snapshot"].(classroom.State)
		if !ok {
			return frame
		}
		events, _ := value["events"].([]classroom.Event)
		for _, event := range events {
			if event.State != nil {
				snapshot = *event.State
				continue
			}
			snapshot.Revision = event.Revision
			switch event.Kind {
			case classroom.EventKindMoveApplied:
				snapshot.FEN = event.FEN
				snapshot.Moves = append(snapshot.Moves, event.Move)
			case classroom.EventKindModeChanged:
				snapshot.Mode, snapshot.ActiveActorID = event.Mode, event.ActiveActorID
			case classroom.EventKindPositionLoaded:
				snapshot.FEN = event.FEN
				snapshot.Moves = []string{}
			case classroom.EventKindBoardClosed:
				snapshot.Status = classroom.BoardStatusClosed
			}
		}
		value["protocolVersion"], value["snapshot"], value["events"] = 1, legacyState(snapshot), []classroom.Event{}
	}
	if state, ok := value["state"].(classroom.State); ok {
		value["state"] = legacyState(state)
	}
	if event, ok := value["event"].(classroom.Event); ok {
		event.State = nil
		value["event"] = event
	}
	return value
}
