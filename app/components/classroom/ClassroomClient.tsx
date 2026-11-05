"use client";

import { Chess, type Square } from "chess.js";
import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import Link from "next/link";
import BrandLogo from "@/app/components/BrandLogo";
import { Icon } from "@/app/components/Icon";
import { APIError, apiFetch } from "@/app/lib/api/client";
import { createRealtimeJoinTicket } from "@/app/lib/api/typed-client";
import {
  CLASSROOM_SUBPROTOCOL,
  ClassroomProtocolError,
  createUUIDv7,
  decodeServerMessage,
  deriveRealtimeURL,
  isUUIDv7,
  type ClassroomClientMessage,
  type ClassroomToolCommand,
} from "@/app/lib/realtime/classroom-protocol";
import {
  classroomReducer,
  initialClassroomState,
  type ClassroomPhase,
} from "@/app/lib/realtime/classroom-reducer";
import { useSession } from "@/app/lib/use-session";
import type { AcademyWorkspaceData } from "@/app/lib/academy-types";
import { safeMeetingURL } from "@/app/lib/realtime/lesson-game";
import ClassroomLessonTools from "./ClassroomLessonTools";
import "./classroom.css";

type PreflightState = {
  checked: boolean;
  websocket: boolean;
  online: boolean;
  configured: boolean;
  realtimeURL?: string;
  issue?: string;
};

const statusLabels: Record<ClassroomPhase, string> = {
  idle: "Ready to join",
  ticketing: "Checking access",
  connecting: "Connecting",
  joining: "Joining board",
  ready: "Live",
  recovering: "Synchronizing",
  reconnecting: "Reconnecting",
  offline: "Offline",
  error: "Needs attention",
  stopped: "Left board",
};

const squarePattern = /^[a-h][1-8]$/;

export default function ClassroomClient({ sessionId }: { sessionId: string }) {
  const { session } = useSession();
  const [workspace, setWorkspace] = useState<AcademyWorkspaceData | null>(null);
  const [workspaceError, setWorkspaceError] = useState("");
  const [workspaceSequence, setWorkspaceSequence] = useState(0);
  const [drawMode, setDrawMode] = useState<"move" | "arrow" | "highlight">("move");
  const [drawingFrom, setDrawingFrom] = useState<string | null>(null);
  const [drawingColor, setDrawingColor] = useState("#b87628");
  const canControl = Boolean(session?.authenticated && session.user?.roles.some((role) => ["coach", "academic_manager", "operations_admin", "super_admin"].includes(role)));
  const assignedClass = workspace?.classes.find((item) => item.id === sessionId);
  const roster = workspace?.students.filter((student) => assignedClass?.studentIds.includes(student.id)) ?? [];
  const saveStudents = roster.filter((student) => canControl || (session?.user?.roles.includes("student") && student.userId === session.user.id));
  const meetingURL = safeMeetingURL(assignedClass?.meetingUrl);
  useEffect(() => {
    if (!session?.authenticated) return;
    let active = true;
    void apiFetch<AcademyWorkspaceData>("/academy/workspace").then((value) => { if (active) { setWorkspace(value); setWorkspaceError(""); } }).catch((error) => { if (active) setWorkspaceError(error instanceof Error ? error.message : "Lesson details could not be loaded."); });
    return () => { active = false; };
  }, [session?.authenticated, session?.user?.id, workspaceSequence]);
  const [state, dispatch] = useReducer(classroomReducer, initialClassroomState);
  const [preflight, setPreflight] = useState<PreflightState>({
    checked: false,
    websocket: false,
    online: true,
    configured: false,
  });
  const [started, setStarted] = useState(false);
  const [connectionSequence, setConnectionSequence] = useState(0);
  const [orientation, setOrientation] = useState<"white" | "black">("white");
  const [focused, setFocused] = useState(false);
  const stateRef = useRef(state);
  const startedRef = useRef(started);
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTimerRef = useRef<number | null>(null);
  const heartbeatRef = useRef<number | null>(null);
  const shouldReconnectRef = useRef(false);
  const validSessionId = isUUIDv7(sessionId);

  useEffect(() => {
    stateRef.current = state;
  }, [state]);

  useEffect(() => {
    startedRef.current = started;
  }, [started]);

  useEffect(() => {
    try {
      setPreflight({
        checked: true,
        websocket: typeof WebSocket !== "undefined",
        online: navigator.onLine,
        configured: true,
        realtimeURL: deriveRealtimeURL(),
      });
    } catch (error) {
      setPreflight({
        checked: true,
        websocket: typeof WebSocket !== "undefined",
        online: navigator.onLine,
        configured: false,
        issue: error instanceof Error ? error.message : "The realtime service is not configured.",
      });
    }
  }, []);

  useEffect(() => {
    function handleOffline() {
      setPreflight((current) => ({ ...current, online: false }));
      dispatch({ type: "offline" });
      socketRef.current?.close(4000, "network offline");
    }

    function handleOnline() {
      setPreflight((current) => ({ ...current, online: true }));
      if (!startedRef.current) return;
      if (reconnectTimerRef.current !== null) window.clearTimeout(reconnectTimerRef.current);
      setConnectionSequence((current) => current + 1);
    }

    window.addEventListener("offline", handleOffline);
    window.addEventListener("online", handleOnline);
    return () => {
      window.removeEventListener("offline", handleOffline);
      window.removeEventListener("online", handleOnline);
    };
  }, []);

  useEffect(() => {
    if (!started || !preflight.realtimeURL || !validSessionId || !preflight.websocket) return;

    let disposed = false;
    let socket: WebSocket | null = null;
    let resumeInFlight = false;
    let reconnectResumeSent = false;
    const reconnecting = connectionSequence > 0;
    const lastKnownRevision = stateRef.current.snapshot?.revision ?? 0;

    function clearHeartbeat() {
      if (heartbeatRef.current !== null) {
        window.clearInterval(heartbeatRef.current);
        heartbeatRef.current = null;
      }
    }

    function send(message: ClassroomClientMessage) {
      if (!socket || socket.readyState !== WebSocket.OPEN) return false;
      socket.send(JSON.stringify(message));
      return true;
    }

    function requestResume(afterRevision: number) {
      if (resumeInFlight) return;
      resumeInFlight = send({ type: "resume", commandId: createUUIDv7(), afterRevision });
    }

    function scheduleReconnect(notice: string) {
      if (disposed || !shouldReconnectRef.current) return;
      const nextAttempt = connectionSequence + 1;
      const offline = navigator.onLine === false;
      const delay = offline ? 3_000 : Math.min(1_000 * (2 ** Math.min(nextAttempt - 1, 4)), 15_000);
      dispatch(offline
        ? { type: "offline" }
        : { type: "reconnectScheduled", reconnectAttempt: nextAttempt, notice });
      reconnectTimerRef.current = window.setTimeout(() => {
        if (!disposed && shouldReconnectRef.current) setConnectionSequence(nextAttempt);
      }, delay);
    }

    function failProtocol(message: string) {
      shouldReconnectRef.current = false;
      dispatch({ type: "fatal", error: message });
      socket?.close(1002, "protocol error");
    }

    async function connect() {
      dispatch({ type: "connectionStarted", reconnectAttempt: connectionSequence });
      try {
        const joinTicket = await createRealtimeJoinTicket(sessionId);
        if (disposed) return;
        if (joinTicket.ticket.length < 40 || joinTicket.ticket.length > 64 || Date.parse(joinTicket.expiresAt) <= Date.now()) {
          throw new ClassroomProtocolError("The academy service returned an invalid or expired classroom ticket.");
        }

        socket = new WebSocket(preflight.realtimeURL as string, CLASSROOM_SUBPROTOCOL);
        socketRef.current = socket;

        socket.onopen = () => {
          if (!socket || socket.protocol !== CLASSROOM_SUBPROTOCOL) {
            failProtocol("The realtime service did not negotiate classroom protocol version 2.");
            return;
          }
          dispatch({ type: "socketOpened" });
          send({ type: "join", commandId: createUUIDv7(), ticket: joinTicket.ticket });
        };

        socket.onmessage = (event) => {
          if (typeof event.data !== "string") {
            failProtocol("The realtime service sent an unsupported binary frame.");
            return;
          }

          let message;
          try {
            message = decodeServerMessage(event.data);
          } catch (error) {
            failProtocol(error instanceof Error ? error.message : "The realtime service sent an invalid message.");
            return;
          }

          if ("sessionId" in message && message.sessionId !== sessionId) {
            failProtocol("The realtime service returned state for a different classroom.");
            return;
          }
          if (message.type === "snapshot" && message.snapshot.sessionId !== sessionId) {
            failProtocol("The realtime snapshot belongs to a different classroom.");
            return;
          }

          switch (message.type) {
            case "snapshot":
              dispatch({ type: "snapshotReceived", message });
              resumeInFlight = false;
              if (reconnecting && !reconnectResumeSent) {
                reconnectResumeSent = true;
                requestResume(lastKnownRevision);
              }
              if (heartbeatRef.current === null) {
                heartbeatRef.current = window.setInterval(() => {
                  send({ type: "ping", commandId: createUUIDv7() });
                }, 25_000);
              }
              break;
            case "accepted":
              dispatch({ type: "accepted", message });
              break;
            case "rejected":
              dispatch({ type: "rejected", message });
              if (message.code === "revision_conflict") {
                requestResume(stateRef.current.snapshot?.revision ?? 0);
              }
              break;
            case "revisionGap":
              dispatch({ type: "revisionGap", message });
              if (message.revision > (stateRef.current.snapshot?.revision ?? 0)) {
                requestResume(stateRef.current.snapshot?.revision ?? 0);
              }
              break;
            case "error":
              dispatch({ type: "fatal", error: message.detail });
              if (message.code === "invalid_message") shouldReconnectRef.current = false;
              socket?.close(1011, message.code);
              break;
            case "presence":
            case "privateResponse":
            case "exerciseReveal":
            case "providerState":
              // Feature events never mutate the canonical board in this client.
              break;
          }
        };

        socket.onerror = () => {
          // Browsers deliberately hide WebSocket error details; onclose owns recovery.
        };

        socket.onclose = () => {
          clearHeartbeat();
          if (socketRef.current === socket) socketRef.current = null;
          scheduleReconnect("The live connection closed. Reconnecting with a fresh ticket…");
        };
      } catch (error) {
        if (disposed) return;
        if (error instanceof APIError && (error.status === 401 || error.status === 403 || error.status === 422)) {
          shouldReconnectRef.current = false;
          dispatch({ type: "fatal", error: error.message });
          return;
        }
        if (error instanceof ClassroomProtocolError) {
          shouldReconnectRef.current = false;
          dispatch({ type: "fatal", error: error.message });
          return;
        }
        scheduleReconnect("The academy service could not be reached. Trying again…");
      }
    }

    void connect();
    return () => {
      disposed = true;
      clearHeartbeat();
      if (reconnectTimerRef.current !== null) {
        window.clearTimeout(reconnectTimerRef.current);
        reconnectTimerRef.current = null;
      }
      if (socket) {
        socket.onclose = null;
        socket.close(1000, "connection replaced");
      }
      if (socketRef.current === socket) socketRef.current = null;
    };
  }, [connectionSequence, preflight.realtimeURL, preflight.websocket, sessionId, started, validSessionId]);

  useEffect(() => () => {
    shouldReconnectRef.current = false;
    if (reconnectTimerRef.current !== null) window.clearTimeout(reconnectTimerRef.current);
    if (heartbeatRef.current !== null) window.clearInterval(heartbeatRef.current);
    socketRef.current?.close(1000, "classroom left");
  }, []);

  const canJoin = preflight.checked
    && preflight.websocket
    && preflight.configured
    && preflight.online
    && validSessionId;
  const toolsReady = state.phase === "ready"
    && state.snapshot?.status === "open"
    && state.pendingCommandIds.length === 0
    && socketRef.current?.readyState === WebSocket.OPEN;
  const canMove = canControl || Boolean(session?.user?.roles.includes("student") && (state.snapshot?.mode === "free_analysis" || (state.snapshot?.mode === "student_turn" && state.snapshot.activeActorId === session.user.id)));
  const boardInteractive = canMove && drawMode === "move" && state.phase === "ready"
    && state.snapshot?.status === "open"
    && state.pendingCommandIds.length === 0
    && socketRef.current?.readyState === WebSocket.OPEN;

  const sendTool = useCallback((command: ClassroomToolCommand) => {
    const current = stateRef.current;
    const socket = socketRef.current;
    if (!canControl || current.phase !== "ready" || current.snapshot?.status !== "open" || current.pendingCommandIds.length || socket?.readyState !== WebSocket.OPEN) return false;
    const commandId = createUUIDv7();
    socket.send(JSON.stringify({ ...command, commandId, expectedRevision: current.snapshot.revision } satisfies ClassroomClientMessage));
    dispatch({ type: "commandSent", commandId });
    return true;
  }, [canControl]);

  const boardOptions = useMemo(() => ({
    id: `classroom-board-${sessionId}`,
    position: state.snapshot?.fen,
    boardOrientation: orientation,
    allowDragging: boardInteractive,
    allowDrawingArrows: false,
    arrows: (state.snapshot?.lesson?.annotations ?? []).filter((annotation) => annotation.startSquare !== annotation.endSquare),
    squareStyles: Object.fromEntries([
      ...(state.snapshot?.lesson?.annotations ?? []).filter((annotation) => annotation.startSquare === annotation.endSquare).map((annotation) => [annotation.startSquare, { boxShadow: `inset 0 0 0 5px ${annotation.color}` }]),
      ...(drawingFrom ? [[drawingFrom, { boxShadow: `inset 0 0 0 5px ${drawingColor}` }]] : []),
    ]),
    onSquareClick: ({ square }: { square: string }) => {
      if (!canControl || !toolsReady || drawMode === "move") return;
      if (drawMode === "arrow" && !drawingFrom) { setDrawingFrom(square); return; }
      const from = drawMode === "highlight" ? square : drawingFrom!;
      setDrawingFrom(null);
      if (drawMode === "arrow" && from === square) return;
      const annotations = stateRef.current.snapshot?.lesson?.annotations ?? [];
      const exists = annotations.some((annotation) => annotation.startSquare === from && annotation.endSquare === square);
      sendTool({ type: "annotation", annotations: exists ? annotations.filter((annotation) => annotation.startSquare !== from || annotation.endSquare !== square) : [...annotations, { startSquare: from, endSquare: square, color: drawingColor }].slice(-64) });
    },
    animationDurationInMs: 180,
    darkSquareStyle: { backgroundColor: "var(--board-dark, #B08C5C)" },
    lightSquareStyle: { backgroundColor: "var(--board-light, #EDDEC1)" },
    boardStyle: { borderRadius: 7, overflow: "hidden" },
    onPieceDrop: ({ sourceSquare, targetSquare }: { sourceSquare: string; targetSquare: string | null }) => {
      const current = stateRef.current;
      const socket = socketRef.current;
      if (!boardInteractive || !targetSquare
        || !squarePattern.test(sourceSquare)
        || !squarePattern.test(targetSquare)
        || current.phase !== "ready"
        || current.snapshot?.status !== "open"
        || current.pendingCommandIds.length
        || !socket
        || socket.readyState !== WebSocket.OPEN) return false;

      try {
        const chess = new Chess(current.snapshot.fen);
        const move = chess.move({ from: sourceSquare as Square, to: targetSquare as Square, promotion: "q" });
        if (!move) return false;
        const commandId = createUUIDv7();
        socket.send(JSON.stringify({
          type: "move",
          commandId,
          expectedRevision: current.snapshot.revision,
          move: `${move.from}${move.to}${move.promotion || ""}`,
        } satisfies ClassroomClientMessage));
        dispatch({ type: "commandSent", commandId });
      } catch {
        return false;
      }
      return false;
    },
  }), [boardInteractive, canControl, drawMode, drawingColor, drawingFrom, orientation, sendTool, sessionId, state.snapshot?.fen, state.snapshot?.lesson?.annotations, toolsReady]);

  function joinBoard() {
    if (!canJoin) return;
    shouldReconnectRef.current = true;
    setConnectionSequence(0);
    setStarted(true);
  }

  function retryConnection() {
    if (!canJoin) return;
    shouldReconnectRef.current = true;
    setStarted(true);
    setConnectionSequence((current) => current + 1);
  }

  function leaveBoard() {
    shouldReconnectRef.current = false;
    setStarted(false);
    setFocused(false);
    socketRef.current?.close(1000, "user left classroom");
    dispatch({ type: "stopped" });
  }

  return (
    <section className={`classroom-client kg-classroom ${focused ? "classroom-focused" : ""}`} aria-label="Live classroom board">
      <header className="classroom-room-header">
        <Link href="/portal" className="classroom-brand" aria-label="Return to your workspace"><BrandLogo size={42} /><span>KingsGambit<small>2700 · CLASSROOM</small></span></Link>
        <div className="classroom-room-title"><h1>{assignedClass?.title || "Live classroom"}</h1><p>A shared board. Room to grow.</p></div>
        <div className="classroom-connection-status" role="status">
          <span className={`connection-dot connection-${state.phase}`} aria-hidden="true" />
          <strong>{statusLabels[state.phase]}</strong>
        </div>
      </header>

      {!state.snapshot ? (
        <div className="classroom-preflight-panel">
          <div className="classroom-preflight-copy">
            <span className="eyebrow">Before your lesson</span>
            <h2>Your next move<br />starts here.</h2>
            <p>Your classroom opens shortly before your scheduled lesson. Join with your student or coach account, then follow the shared board.</p>
            <ul className="preflight-checks">
              <li className={validSessionId ? "is-ready" : "is-blocked"}><span>{validSessionId ? "✓" : "!"}</span><div><strong>Scheduled session link</strong><small>{validSessionId ? "Your class link is ready." : "Open the classroom from an assigned schedule link."}</small></div></li>
              <li className={preflight.websocket ? "is-ready" : "is-blocked"}><span>{preflight.websocket ? "✓" : "!"}</span><div><strong>Browser support</strong><small>{preflight.websocket ? "Secure realtime boards are supported." : "Try a current version of Chrome, Firefox, Edge or Safari."}</small></div></li>
              <li className={preflight.online ? "is-ready" : "is-blocked"}><span>{preflight.online ? "✓" : "!"}</span><div><strong>Network</strong><small>{preflight.online ? "Your device reports an active connection." : "Reconnect to the internet before joining."}</small></div></li>
              <li className={preflight.configured ? "is-ready" : "is-blocked"}><span>{preflight.configured ? "✓" : "!"}</span><div><strong>Academy service</strong><small>{preflight.configured ? "Your academy connection is ready." : preflight.issue}</small></div></li>
            </ul>
            {state.error && <p className="classroom-error" role="alert">{state.error}</p>}
            <div className="classroom-join-actions">
              <button className="button button-dark" onClick={state.phase === "error" ? retryConnection : joinBoard} disabled={!canJoin || ["ticketing", "connecting", "joining", "reconnecting"].includes(state.phase)}>
                {state.phase === "error" ? "Try again" : state.phase === "idle" || state.phase === "stopped" ? "Join classroom board" : statusLabels[state.phase]}
              </button>
              <small>No camera or microphone permission is requested.</small>
            </div>
          </div>
          <aside className="classroom-device-note">
            <span className="classroom-welcome-piece" aria-hidden="true">♞</span>
            <span className="eyebrow">Settle in. Find your focus.</span>
            <h3>A little preparation.<br />A great lesson.</h3>
            <p>Use a current browser, keep this tab open, connect your charger, and prefer a stable network. If the connection drops, we will reconnect you and restore the latest position.</p>
            <dl><div><dt>Video</dt><dd>Off</dd></div><div><dt>Recording</dt><dd>Off</dd></div><div><dt>Transcription</dt><dd>Off</dd></div></dl>
          </aside>
        </div>
      ) : (
        <div className="classroom-stage">
          <div className="classroom-board-column">
            <div className="classroom-board-heading"><div><span className="eyebrow">Explore. Think. Play.</span><h2>Your next good move.</h2></div><span className="classroom-turn-pill">{state.snapshot.fen.split(" ")[1] === "b" ? "Black" : "White"} to move</span></div>
            <div className="classroom-board-frame">
            <div className="classroom-board-label"><span className={`classroom-piece-dot ${orientation === "white" ? "is-black" : ""}`} /><strong>{orientation === "white" ? "Black" : "White"} pieces</strong><small>Shared lesson board</small></div>
            <div className="classroom-board-live" aria-label={`Chessboard at revision ${state.snapshot.revision}`}>
              <Chessboard options={boardOptions} />
              {state.phase !== "ready" && <div className="classroom-board-overlay"><span>{statusLabels[state.phase]}…</span></div>}
            </div>
            <div className="classroom-board-label"><span className={`classroom-piece-dot ${orientation === "black" ? "is-black" : ""}`} /><strong>{orientation === "white" ? "White" : "Black"} pieces</strong><small>Revision {state.snapshot.revision}</small></div>
            </div>
            {canControl && <div className="classroom-drawing-tools" aria-label="Shared board markings">
              {(["move", "arrow", "highlight"] as const).map((tool) => <button key={tool} type="button" disabled={!toolsReady} aria-pressed={drawMode === tool} onClick={() => { setDrawMode(tool); setDrawingFrom(null); }}>{tool === "move" ? "Move pieces" : tool === "arrow" ? "Draw arrow" : "Highlight square"}</button>)}
              <input type="color" aria-label="Marking color" value={drawingColor} onChange={(event) => setDrawingColor(event.target.value)} />
              {drawMode !== "move" && <small>{drawMode === "arrow" ? drawingFrom ? `Now choose the target square for ${drawingFrom}.` : "Click a starting square, then a target square." : "Click a square to highlight it or remove its highlight."} Everyone sees these markings.</small>}
            </div>}
            <div className="classroom-board-actions">
              <button type="button" onClick={() => setOrientation((current) => current === "white" ? "black" : "white")}><Icon name="settings" size={17} /> Flip board</button>
              <button type="button" onClick={() => setFocused((current) => !current)} aria-pressed={focused}><Icon name="expand" size={17} />{focused ? "Show lesson tools" : "Focus board"}</button>
            </div>
            <p className="demo-board-instruction"><Icon name="spark" size={18} /><span>{boardInteractive ? "Drag a piece to play a legal move. Look for checks, captures and threats." : canControl && drawMode !== "move" ? "Choose squares to add a shared marking." : toolsReady ? "Follow your coach’s board. You can move when your coach invites you." : "Board interaction is paused. Your position will update when the connection is ready."}</span></p>
            {focused && state.error && <p className="classroom-error" role="alert">{state.error}</p>}
            {focused && state.phase === "error" && <button className="button button-dark" onClick={retryConnection} disabled={!canJoin}>Reconnect</button>}
            {focused && state.notice && <p role="status">{state.notice}</p>}
          </div>

          <aside className="classroom-side-panel">
            <section className="classroom-state-card">
              <span className="eyebrow">Lesson status</span>
              <h3>A space to think.</h3>
              <dl>
                <div><dt>Status</dt><dd>{state.snapshot.status}</dd></div>
                <div><dt>Mode</dt><dd>{state.snapshot.mode.replaceAll("_", " ")}</dd></div>
                <div><dt>Revision</dt><dd>{state.snapshot.revision}</dd></div>
                <div><dt>Connection</dt><dd>{statusLabels[state.phase]}</dd></div>
              </dl>
              {state.notice && <p aria-live="polite">{state.notice}</p>}
              {state.error && <p className="classroom-error" role="alert">{state.error}</p>}
            </section>

            <section className="classroom-moves-card">
              <div><span className="eyebrow">Move record</span><strong>{state.snapshot.moves.length} moves</strong></div>
              {state.snapshot.moves.length ? (
                <ol>{state.snapshot.moves.slice(-12).map((move, index) => <li key={`${state.snapshot?.revision}-${index}-${move}`}><span>{state.snapshot!.moves.length - Math.min(12, state.snapshot!.moves.length) + index + 1}</span>{move}</li>)}</ol>
              ) : <p>No moves have been recorded in this position.</p>}
            </section>

            <ClassroomLessonTools board={state.snapshot} canControl={canControl} disabled={!toolsReady} onCommand={sendTool} students={roster} saveStudents={saveStudents} title={assignedClass?.title || "Lesson game"} />
            <section className="classroom-privacy-card">
              <strong>{meetingURL ? "Meet your coach" : "Your lesson space"}</strong>
              <p>The shared board stays here. Camera, microphone and recording are off on this page.</p>
              {meetingURL && <a className="classroom-meeting-link" href={meetingURL} target="_blank" rel="noopener noreferrer">Open lesson meeting ↗</a>}
              {workspaceError && <><p role="alert">{workspaceError}</p><button type="button" onClick={() => setWorkspaceSequence((value) => value + 1)}>Retry lesson details</button></>}
            </section>

            <div className="classroom-session-actions">
              {state.phase === "error" && <button className="button button-dark" onClick={retryConnection} disabled={!canJoin}>Reconnect</button>}
              {state.phase === "stopped" ? <button className="button button-dark" onClick={joinBoard} disabled={!canJoin}>Join classroom board</button> : <button className="text-link" onClick={leaveBoard}>Leave board</button>}
              <Link href="/portal" className="text-link">Back to workspace</Link>
            </div>
          </aside>
          <footer className="classroom-room-footer"><span><Icon name="shield" size={16} /> Camera, microphone and recording are off.</span><span className="classroom-connection-status"><span className={`connection-dot connection-${state.phase}`} aria-hidden="true" />{statusLabels[state.phase]}</span>{focused && <button type="button" className="classroom-leave-focus" onClick={leaveBoard}>Leave board</button>}</footer>
        </div>
      )}
    </section>
  );
}
