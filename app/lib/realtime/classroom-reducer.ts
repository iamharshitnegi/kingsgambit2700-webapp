import { ClassroomProtocolError } from "./classroom-protocol";
import type { AcceptedMessage, ClassroomBoardState, RejectedMessage, RevisionGapMessage, SnapshotMessage } from "./classroom-protocol";

export type ClassroomPhase = "idle" | "ticketing" | "connecting" | "joining" | "ready" | "recovering" | "reconnecting" | "offline" | "error" | "stopped";

export type ClassroomClientState = {
  phase: ClassroomPhase;
  snapshot: ClassroomBoardState | null;
  pendingCommandIds: string[];
  reconnectAttempt: number;
  gapRevision?: number;
  notice?: string;
  error?: string;
};

export type ClassroomAction =
  | { type: "connectionStarted"; reconnectAttempt: number }
  | { type: "socketOpened" }
  | { type: "commandSent"; commandId: string }
  | { type: "snapshotReceived"; message: SnapshotMessage }
  | { type: "accepted"; message: AcceptedMessage }
  | { type: "rejected"; message: RejectedMessage }
  | { type: "revisionGap"; message: RevisionGapMessage }
  | { type: "reconnectScheduled"; reconnectAttempt: number; notice: string }
  | { type: "offline" }
  | { type: "fatal"; error: string }
  | { type: "stopped" };

export const initialClassroomState: ClassroomClientState = {
  phase: "idle",
  snapshot: null,
  pendingCommandIds: [],
  reconnectAttempt: 0,
};

function withoutCommand(state: ClassroomClientState, commandId: string) {
  return state.pendingCommandIds.filter((id) => id !== commandId);
}

export function classroomReducer(state: ClassroomClientState, action: ClassroomAction): ClassroomClientState {
  switch (action.type) {
    case "connectionStarted":
      return {
        ...state,
        phase: "ticketing",
        reconnectAttempt: action.reconnectAttempt,
        pendingCommandIds: [],
        notice: action.reconnectAttempt ? "Requesting fresh classroom access…" : "Checking classroom access…",
        error: undefined,
      };
    case "socketOpened":
      return { ...state, phase: "joining", notice: "Joining the live board…", error: undefined };
    case "commandSent":
      return {
        ...state,
        pendingCommandIds: [...state.pendingCommandIds, action.commandId],
        notice: "Sending your board change…",
        error: undefined,
      };
    case "snapshotReceived": {
      let snapshot: ClassroomBoardState;
      try { snapshot = replayClassroomSnapshot(action.message); }
      catch (error) { return { ...state, phase: "error", pendingCommandIds: [], error: error instanceof Error ? error.message : "Could not recover the board." }; }
      if (state.snapshot && state.snapshot.sessionId === snapshot.sessionId && state.snapshot.revision > snapshot.revision) {
        return { ...state, phase: "ready", pendingCommandIds: [], gapRevision: undefined };
      }
      return {
        ...state,
        phase: "ready",
        snapshot,
        pendingCommandIds: [],
        gapRevision: undefined,
        notice: snapshot.status === "closed" ? "This board is closed." : "Board synchronized.",
        error: undefined,
      };
    }
    case "accepted": {
      const candidate = action.message.state;
      const snapshot = candidate && (!state.snapshot || candidate.revision >= state.snapshot.revision)
        ? candidate
        : state.snapshot;
      return {
        ...state,
        phase: "ready",
        snapshot,
        pendingCommandIds: withoutCommand(state, action.message.commandId),
        notice: candidate ? "Board change confirmed." : state.notice,
        error: undefined,
      };
    }
    case "rejected": {
      const recovering = action.message.code === "revision_conflict";
      return {
        ...state,
        phase: recovering ? "recovering" : state.snapshot ? "ready" : "error",
        pendingCommandIds: withoutCommand(state, action.message.commandId),
        gapRevision: recovering ? action.message.currentRevision : state.gapRevision,
        notice: recovering ? "The board changed elsewhere. Reloading the canonical position…" : state.notice,
        error: action.message.detail,
      };
    }
    case "revisionGap":
      if (state.snapshot && action.message.revision <= state.snapshot.revision) return state;
      return {
        ...state,
        phase: "recovering",
        gapRevision: action.message.revision,
        notice: "A newer board revision is available. Synchronizing…",
        error: undefined,
      };
    case "reconnectScheduled":
      return {
        ...state,
        phase: "reconnecting",
        reconnectAttempt: action.reconnectAttempt,
        pendingCommandIds: [],
        notice: action.notice,
      };
    case "offline":
      return { ...state, phase: "offline", pendingCommandIds: [], notice: "You are offline. The board will reconnect when the network returns." };
    case "fatal":
      return { ...state, phase: "error", pendingCommandIds: [], error: action.error, notice: undefined };
    case "stopped":
      return { ...initialClassroomState, phase: "stopped", snapshot: state.snapshot, notice: "You left the live board." };
    default:
      return state;
  }
}

// A persisted snapshot can be older than the room. Replaying the contiguous
// tail is necessary before accepting input or showing a recovered board.
export function replayClassroomSnapshot(message: SnapshotMessage): ClassroomBoardState {
  let state = message.snapshot;
  for (const event of message.events) {
    if (event.sessionId !== state.sessionId || event.revision !== state.revision + 1) {
      throw new ClassroomProtocolError("The board history has a gap. Reconnect to reload it.");
    }
    if (event.state) {
      if (event.state.sessionId !== state.sessionId || event.state.revision !== event.revision) {
        throw new ClassroomProtocolError("The recovered board revision is inconsistent.");
      }
      state = event.state;
      continue;
    }
    const next = { ...state, revision: event.revision };
    switch (event.kind) {
      case "move_applied":
        if (!event.fen || !event.move) throw new ClassroomProtocolError("The recovered move is incomplete.");
        next.fen = event.fen;
        next.moves = [...state.moves, event.move];
        if (state.lesson) next.lesson = { ...state.lesson, positions: [...state.lesson.positions, event.fen], annotations: [] };
        break;
      case "mode_changed":
        if (!event.mode) throw new ClassroomProtocolError("The recovered mode is incomplete.");
        next.mode = event.mode;
        next.activeActorId = event.activeActorId;
        break;
      case "board_closed": next.status = "closed"; break;
      default: throw new ClassroomProtocolError("The recovered lesson needs a fresh snapshot. Reconnect to reload it.");
    }
    state = next;
  }
  return state;
}
