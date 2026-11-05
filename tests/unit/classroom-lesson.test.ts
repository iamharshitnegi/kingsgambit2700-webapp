import { describe, expect, it } from "vitest";
import { Chess } from "chess.js";
import { decodeServerMessage, type ClassroomBoardState, type ClassroomEvent, type SnapshotMessage } from "@/app/lib/realtime/classroom-protocol";
import { classroomReducer, initialClassroomState, replayClassroomSnapshot } from "@/app/lib/realtime/classroom-reducer";
import { exportLessonPGN, importLessonPGN, safeMeetingURL } from "@/app/lib/realtime/lesson-game";

const id = "01900000-0000-7000-8000-000000000001";
const initialFen = new Chess().fen();
const board: ClassroomBoardState = { sessionId: id, fen: initialFen, moves: [], revision: 0, mode: "demonstration", status: "open", lesson: { initialFen, positions: [initialFen], annotations: [], checkpoints: [] } };
const after: ClassroomBoardState = { ...board, revision: 1, lesson: { ...board.lesson!, annotations: [{ startSquare: "e2", endSquare: "e4", color: "#123456" }], countdownEndsAt: "2026-09-22T12:00:00Z" } };
const event: ClassroomEvent = { id, sessionId: id, revision: 1, kind: "lesson_updated", actorId: id, occurredAt: "2026-09-22T11:59:00Z", state: after };
const snapshot: SnapshotMessage = { type: "snapshot", protocolVersion: 2, commandId: id, snapshot: board, events: [event] };

describe("shared lesson recovery", () => {
  it("decodes the v2 contract and replays tools after the saved snapshot", () => {
    const decoded = decodeServerMessage(JSON.stringify(snapshot));
    expect(decoded.type).toBe("snapshot");
    const recovered = classroomReducer(initialClassroomState, { type: "snapshotReceived", message: decoded as SnapshotMessage });
    expect(recovered.snapshot).toEqual(after);
    expect(recovered.phase).toBe("ready");
  });
  it("replays move events from a legacy snapshot", () => {
    const game = new Chess(); game.move("e4");
    const recovered = replayClassroomSnapshot({ ...snapshot, events: [{ ...event, kind: "move_applied", state: undefined, fen: game.fen(), move: "e2e4" }] });
    expect(recovered.moves).toEqual(["e2e4"]);
    expect(recovered.fen).toBe(game.fen());
  });
  it("rejects gaps and cross-class events without making the board interactive", () => {
    for (const invalid of [{ ...event, revision: 2 }, { ...event, sessionId: "01900000-0000-7000-8000-000000000002" }]) {
      const state = classroomReducer(initialClassroomState, { type: "snapshotReceived", message: { ...snapshot, events: [invalid] } });
      expect(state.phase).toBe("error"); expect(state.snapshot).toBeNull();
    }
  });
  it("does not roll a newer canonical board back to an older recovery response", () => {
    const current = { ...initialClassroomState, snapshot: { ...after, revision: 5 } };
    expect(classroomReducer(current, { type: "snapshotReceived", message: snapshot }).snapshot?.revision).toBe(5);
  });
  it("rejects invalid annotations and nested state revision mismatches", () => {
    const malformed = structuredClone(snapshot); malformed.events[0].state!.revision = 9;
    expect(() => decodeServerMessage(JSON.stringify(malformed))).toThrow();
    const badColor = structuredClone(snapshot); badColor.snapshot.lesson!.annotations = [{ startSquare: "z9", endSquare: "e4", color: "url(bad)" }];
    expect(() => decodeServerMessage(JSON.stringify(badColor))).toThrow();
  });
});

describe("lesson game exchange", () => {
  it("round trips PGN move history and final position", () => {
    const imported = importLessonPGN('[Event "Opening"]\n\n1. e4 e5 2. Nf3 Nc6 *');
    expect(imported.moves).toEqual(["e2e4", "e7e5", "g1f3", "b8c6"]);
    const pgn = exportLessonPGN({ ...board, ...imported, lesson: { ...board.lesson!, initialFen: imported.initialFen } }, "My lesson");
    const reimported = importLessonPGN(pgn);
    expect(reimported).toEqual(imported);
    expect(pgn).toContain('[Event "My lesson"]');
  });
  it("preserves a custom starting position and promotion", () => {
    const imported = importLessonPGN('[SetUp "1"]\n[FEN "7k/P7/8/8/8/8/8/7K w - - 0 1"]\n\n1. a8=Q+ *');
    expect(imported.moves).toEqual(["a7a8q"]);
    expect(imported.initialFen).toContain("P7");
  });
  it("rejects illegal and oversized PGN", () => {
    expect(() => importLessonPGN("1. e5")).toThrow();
    expect(() => importLessonPGN("x".repeat(64_001))).toThrow(/64 KB/);
  });
  it("opens only configured HTTPS meeting links", () => {
    expect(safeMeetingURL("https://meet.example.test/lesson")).toBe("https://meet.example.test/lesson");
    expect(safeMeetingURL("javascript:alert(1)")).toBeUndefined();
    expect(safeMeetingURL("https://user:secret@example.test/")).toBeUndefined();
  });
});
