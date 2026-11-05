import { describe, expect, it } from "vitest";
import {
  classroomReducer,
  initialClassroomState,
  type ClassroomClientState,
} from "@/app/lib/realtime/classroom-reducer";
import type { ClassroomBoardState, SnapshotMessage } from "@/app/lib/realtime/classroom-protocol";

const sessionId = "018bcfe5-6800-7000-8000-000000000001";
const commandId = "018bcfe5-6800-7000-8000-000000000002";

function board(revision: number): ClassroomBoardState {
  return {
    sessionId,
    fen: "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1",
    moves: [],
    mode: "demonstration",
    revision,
    status: "open",
  };
}

function snapshot(revision: number): SnapshotMessage {
  return { type: "snapshot", protocolVersion: 1, commandId, snapshot: board(revision), events: [] };
}

describe("classroom connection reducer", () => {
  it("moves from ticket request to a canonical board and applies acknowledgements", () => {
    let state = classroomReducer(initialClassroomState, { type: "connectionStarted", reconnectAttempt: 0 });
    expect(state.phase).toBe("ticketing");
    state = classroomReducer(state, { type: "socketOpened" });
    expect(state.phase).toBe("joining");
    state = classroomReducer(state, { type: "snapshotReceived", message: snapshot(2) });
    expect(state).toMatchObject({ phase: "ready", snapshot: { revision: 2 } });

    state = classroomReducer(state, { type: "commandSent", commandId });
    expect(state.pendingCommandIds).toContain(commandId);
    state = classroomReducer(state, {
      type: "accepted",
      message: { type: "accepted", commandId, state: { ...board(3), moves: ["e2e4"] } },
    });
    expect(state).toMatchObject({ phase: "ready", snapshot: { revision: 3, moves: ["e2e4"] }, pendingCommandIds: [] });
  });

  it("ignores stale revision hints and recovers when a future revision is announced", () => {
    const ready = classroomReducer(initialClassroomState, { type: "snapshotReceived", message: snapshot(4) });
    const stale = classroomReducer(ready, { type: "revisionGap", message: { type: "revisionGap", sessionId, revision: 4 } });
    expect(stale).toBe(ready);

    const gap = classroomReducer(ready, { type: "revisionGap", message: { type: "revisionGap", sessionId, revision: 6 } });
    expect(gap).toMatchObject({ phase: "recovering", gapRevision: 6, snapshot: { revision: 4 } });

    const recovered = classroomReducer(gap, { type: "snapshotReceived", message: snapshot(6) });
    expect(recovered).toMatchObject({ phase: "ready", snapshot: { revision: 6 } });
    expect(recovered.gapRevision).toBeUndefined();
  });

  it("drops a pending command and requests canonical recovery after a revision conflict", () => {
    const ready: ClassroomClientState = {
      ...initialClassroomState,
      phase: "ready",
      snapshot: board(7),
      pendingCommandIds: [commandId],
    };
    const conflicted = classroomReducer(ready, {
      type: "rejected",
      message: {
        type: "rejected",
        commandId,
        code: "revision_conflict",
        detail: "Reload canonical state and retry",
        currentRevision: 8,
      },
    });
    expect(conflicted).toMatchObject({ phase: "recovering", gapRevision: 8, pendingCommandIds: [] });
    expect(conflicted.snapshot?.revision).toBe(7);
  });
});
