import { getAPIBaseURL } from "../env";

export const CLASSROOM_SUBPROTOCOL = "kg2700.classroom.v2";

const UUID_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const UUID_V7_PATTERN = /^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export type ClassroomAnnotation = { startSquare: string; endSquare: string; color: string };
export type ClassroomCheckpoint = { id: string; title: string; fen: string; initialFen: string; moves: string[]; positions: string[]; annotations: ClassroomAnnotation[] };
export type ClassroomLesson = {
  initialFen: string;
  positions: string[];
  annotations: ClassroomAnnotation[];
  checkpoints: ClassroomCheckpoint[];
  countdownEndsAt?: string;
  countdownLabel?: string;
};
export type ClassroomBoardState = {
  sessionId: string;
  fen: string;
  moves: string[];
  mode: string;
  activeActorId?: string;
  revision: number;
  status: "open" | "closed";
  lesson?: ClassroomLesson;
};

export type ClassroomEvent = {
  id: string;
  sessionId: string;
  revision: number;
  kind: "move_applied" | "mode_changed" | "position_loaded" | "board_closed" | "lesson_updated";
  actorId: string;
  occurredAt: string;
  state?: ClassroomBoardState;
  move?: string;
  fen?: string;
  mode?: string;
  activeActorId?: string;
};

export type SnapshotMessage = {
  type: "snapshot";
  protocolVersion: 1 | 2;
  commandId: string;
  snapshot: ClassroomBoardState;
  events: ClassroomEvent[];
};

export type AcceptedMessage = {
  type: "accepted";
  commandId: string;
  operation?: string;
  state?: ClassroomBoardState;
  event?: ClassroomEvent;
};

export type RejectedMessage = {
  type: "rejected";
  commandId: string;
  code: "not_implemented" | "invalid_message" | "unauthorized" | "illegal_move" | "revision_conflict" | "board_closed";
  detail: string;
  currentRevision?: number;
};

export type RevisionGapMessage = {
  type: "revisionGap";
  sessionId: string;
  revision: number;
};

export type FeatureEventMessage = {
  type: "presence" | "privateResponse" | "exerciseReveal" | "providerState";
  sessionId: string;
  revision: number;
  payload: Record<string, unknown>;
};

export type ProtocolErrorMessage = {
  type: "error";
  commandId?: string;
  code: "invalid_ticket" | "invalid_message" | "internal_error";
  detail: string;
};

export type ClassroomServerMessage =
  | SnapshotMessage
  | AcceptedMessage
  | RejectedMessage
  | RevisionGapMessage
  | FeatureEventMessage
  | ProtocolErrorMessage;

export type ClassroomClientMessage =
  | { type: "join"; commandId: string; ticket: string }
  | { type: "resume"; commandId: string; afterRevision: number }
  | { type: "move"; commandId: string; expectedRevision: number; move: string }
  | { type: "ping"; commandId: string }
  | ({ commandId: string; expectedRevision: number } & ClassroomToolCommand);

export type ClassroomToolCommand =
  | { type: "setMode"; mode: string; activeActorId?: string }
  | { type: "loadPosition"; fen: string; initialFen: string; moves: string[] }
  | { type: "annotation"; annotations: ClassroomAnnotation[] }
  | { type: "undo" | "closeBoard" }
  | { type: "checkpoint"; title: string }
  | { type: "restoreCheckpoint" | "deleteCheckpoint"; checkpointId: string }
  | { type: "countdown"; seconds: number; label?: string };

export class ClassroomProtocolError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ClassroomProtocolError";
  }
}

export function isUUIDv7(value: string) {
  return UUID_V7_PATTERN.test(value);
}

export function createUUIDv7(now = Date.now()) {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  let timestamp = Math.max(0, Math.floor(now));
  for (let index = 5; index >= 0; index -= 1) {
    bytes[index] = timestamp % 256;
    timestamp = Math.floor(timestamp / 256);
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x70;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

export function deriveRealtimeURL(apiBaseURL = getAPIBaseURL(), configuredURL = process.env.NEXT_PUBLIC_REALTIME_URL) {
  if (configuredURL?.trim()) {
    let configured: URL;
    try { configured = new URL(configuredURL); }
    catch { throw new ClassroomProtocolError("The academy realtime URL is invalid."); }
    if (!["ws:", "wss:"].includes(configured.protocol) || configured.pathname !== "/ws/v1" || configured.search || configured.hash || configured.username || configured.password || /[?#]/.test(configuredURL)) {
      throw new ClassroomProtocolError("The academy realtime URL must be a WS(S) origin followed by /ws/v1, without credentials or query parameters.");
    }
    return configured.toString();
  }
  if (!apiBaseURL) throw new ClassroomProtocolError("The academy realtime service is not configured.");
  let url: URL;
  try {
    url = new URL(apiBaseURL);
  } catch {
    throw new ClassroomProtocolError("The academy API URL is invalid.");
  }
  if (!["http:", "https:"].includes(url.protocol) || url.pathname !== "/api/v1" || url.search || url.hash || url.username || url.password) {
    throw new ClassroomProtocolError("The academy API URL must be an HTTP(S) origin followed by /api/v1.");
  }
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = "/ws/v1";
  return url.toString();
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function hasOnlyKeys(value: Record<string, unknown>, allowed: string[]) {
  return Object.keys(value).every((key) => allowed.includes(key));
}

function isRevision(value: unknown, minimum = 0): value is number {
  return Number.isInteger(value) && Number(value) >= minimum;
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isAnnotation(value: unknown): value is ClassroomAnnotation {
  return isRecord(value) && hasOnlyKeys(value, ["startSquare", "endSquare", "color"])
    && typeof value.startSquare === "string" && /^[a-h][1-8]$/.test(value.startSquare)
    && typeof value.endSquare === "string" && /^[a-h][1-8]$/.test(value.endSquare)
    && typeof value.color === "string" && /^#[0-9a-f]{6}$/i.test(value.color);
}
function isLesson(value: unknown): value is ClassroomLesson {
  return isRecord(value) && hasOnlyKeys(value, ["initialFen", "positions", "annotations", "checkpoints", "countdownEndsAt", "countdownLabel"])
    && typeof value.initialFen === "string" && isStringArray(value.positions) && value.positions.length > 0
    && Array.isArray(value.annotations) && value.annotations.length <= 64 && value.annotations.every(isAnnotation)
    && Array.isArray(value.checkpoints) && value.checkpoints.length <= 8 && value.checkpoints.every((checkpoint) =>
      isRecord(checkpoint) && hasOnlyKeys(checkpoint, ["id", "title", "fen", "initialFen", "moves", "positions", "annotations"])
      && typeof checkpoint.id === "string" && UUID_PATTERN.test(checkpoint.id)
      && typeof checkpoint.title === "string" && typeof checkpoint.fen === "string" && typeof checkpoint.initialFen === "string"
      && isStringArray(checkpoint.moves) && isStringArray(checkpoint.positions)
      && Array.isArray(checkpoint.annotations) && checkpoint.annotations.every(isAnnotation))
    && (value.countdownEndsAt === undefined || (typeof value.countdownEndsAt === "string" && Number.isFinite(Date.parse(value.countdownEndsAt))))
    && (value.countdownLabel === undefined || typeof value.countdownLabel === "string");
}
function isBoardState(value: unknown): value is ClassroomBoardState {
  if (!isRecord(value) || !hasOnlyKeys(value, ["sessionId", "fen", "moves", "mode", "activeActorId", "revision", "status", "lesson"])) return false;
  return typeof value.sessionId === "string"
    && UUID_PATTERN.test(value.sessionId)
    && typeof value.fen === "string"
    && isStringArray(value.moves)
    && typeof value.mode === "string"
    && (value.activeActorId === undefined || (typeof value.activeActorId === "string" && UUID_PATTERN.test(value.activeActorId)))
    && isRevision(value.revision)
    && (value.status === "open" || value.status === "closed")
    && (value.lesson === undefined || isLesson(value.lesson));
}

const eventKinds = ["move_applied", "mode_changed", "position_loaded", "board_closed", "lesson_updated"];

function isClassroomEvent(value: unknown): value is ClassroomEvent {
  if (!isRecord(value) || !hasOnlyKeys(value, ["id", "sessionId", "revision", "kind", "actorId", "occurredAt", "move", "fen", "mode", "activeActorId", "state"])) return false;
  return typeof value.id === "string"
    && UUID_PATTERN.test(value.id)
    && typeof value.sessionId === "string"
    && UUID_PATTERN.test(value.sessionId)
    && isRevision(value.revision, 1)
    && typeof value.kind === "string"
    && eventKinds.includes(value.kind)
    && typeof value.actorId === "string"
    && UUID_PATTERN.test(value.actorId)
    && typeof value.occurredAt === "string"
    && (value.state === undefined || (isBoardState(value.state) && value.state.sessionId === value.sessionId && value.state.revision === value.revision))
    && (value.move === undefined || typeof value.move === "string")
    && (value.fen === undefined || typeof value.fen === "string")
    && (value.mode === undefined || typeof value.mode === "string")
    && (value.activeActorId === undefined || (typeof value.activeActorId === "string" && UUID_PATTERN.test(value.activeActorId)));
}

function isCommandID(value: unknown): value is string {
  return typeof value === "string" && UUID_PATTERN.test(value);
}

export function decodeServerMessage(raw: string): ClassroomServerMessage {
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    throw new ClassroomProtocolError("The realtime service sent malformed JSON.");
  }
  if (!isRecord(value) || typeof value.type !== "string") {
    throw new ClassroomProtocolError("The realtime service sent an invalid message.");
  }

  if (value.type === "snapshot") {
    if (!hasOnlyKeys(value, ["type", "protocolVersion", "commandId", "snapshot", "events"])
      || ![1, 2].includes(Number(value.protocolVersion))
      || !isCommandID(value.commandId)
      || !isBoardState(value.snapshot)
      || (value.protocolVersion === 2 && !value.snapshot.lesson)
      || !Array.isArray(value.events)
      || !value.events.every(isClassroomEvent)) {
      throw new ClassroomProtocolError("The realtime snapshot did not match the classroom protocol.");
    }
    return value as SnapshotMessage;
  }

  if (value.type === "accepted") {
    if (!hasOnlyKeys(value, ["type", "commandId", "operation", "state", "event"])
      || !isCommandID(value.commandId)
      || (value.operation !== undefined && typeof value.operation !== "string")
      || (value.state !== undefined && !isBoardState(value.state))
      || (value.event !== undefined && !isClassroomEvent(value.event))) {
      throw new ClassroomProtocolError("The realtime acknowledgement was invalid.");
    }
    return value as AcceptedMessage;
  }

  if (value.type === "rejected") {
    const codes = ["not_implemented", "invalid_message", "unauthorized", "illegal_move", "revision_conflict", "board_closed"];
    if (!hasOnlyKeys(value, ["type", "commandId", "code", "detail", "currentRevision"])
      || !isCommandID(value.commandId)
      || typeof value.code !== "string"
      || !codes.includes(value.code)
      || typeof value.detail !== "string"
      || (value.currentRevision !== undefined && !isRevision(value.currentRevision))) {
      throw new ClassroomProtocolError("The realtime rejection was invalid.");
    }
    return value as RejectedMessage;
  }

  if (value.type === "revisionGap") {
    if (!hasOnlyKeys(value, ["type", "sessionId", "revision"])
      || typeof value.sessionId !== "string"
      || !UUID_PATTERN.test(value.sessionId)
      || !isRevision(value.revision, 1)) {
      throw new ClassroomProtocolError("The realtime revision notice was invalid.");
    }
    return value as RevisionGapMessage;
  }

  if (["presence", "privateResponse", "exerciseReveal", "providerState"].includes(value.type)) {
    if (!hasOnlyKeys(value, ["type", "sessionId", "revision", "payload"])
      || typeof value.sessionId !== "string"
      || !UUID_PATTERN.test(value.sessionId)
      || !isRevision(value.revision)
      || !isRecord(value.payload)) {
      throw new ClassroomProtocolError("The realtime feature event was invalid.");
    }
    return value as FeatureEventMessage;
  }

  if (value.type === "error") {
    const codes = ["invalid_ticket", "invalid_message", "internal_error"];
    if (!hasOnlyKeys(value, ["type", "commandId", "code", "detail"])
      || (value.commandId !== undefined && !isCommandID(value.commandId))
      || typeof value.code !== "string"
      || !codes.includes(value.code)
      || typeof value.detail !== "string") {
      throw new ClassroomProtocolError("The realtime error message was invalid.");
    }
    return value as ProtocolErrorMessage;
  }

  throw new ClassroomProtocolError(`Unsupported realtime message type: ${value.type}.`);
}
