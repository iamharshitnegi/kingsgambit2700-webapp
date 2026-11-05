"use client";

import Link from "next/link";
import { Chess, type Square } from "chess.js";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import { demoAction, fetchWorkspace } from "@/app/lib/demo/client";
import type { DemoWorkspace } from "@/app/lib/demo/data";
import type { SessionUser } from "@/app/lib/api/types";
import { landingPathForRoles } from "@/app/lib/role-routing";
import BrandLogo from "@/app/components/BrandLogo";
import { Icon } from "@/app/components/Icon";
import "./classroom.css";

export default function DemoClassroom({ sessionId, user }: { sessionId: string; user: SessionUser }) {
  const [workspace, setWorkspace] = useState<DemoWorkspace | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [orientation, setOrientation] = useState<"white" | "black">("white");
  const [joinedSession, setJoinedSession] = useState<string | null>(null);
  const [focused, setFocused] = useState(false);
  const [selectedSquare, setSelectedSquare] = useState<string | null>(null);
  const [promotion, setPromotion] = useState("q");
  const mutationPending = useRef(false);
  const teacher = user.roles.some((role) => ["coach", "super_admin"].includes(role));
  const observer = user.roles.some((role) => ["academic_manager", "operations_admin"].includes(role));
  const joinStorageKey = `kingsgambit:demo-classroom:${user.id}:${sessionId}`;
  const joined = joinedSession === joinStorageKey;

  useEffect(() => {
    try {
      setJoinedSession(window.sessionStorage.getItem(joinStorageKey) === "joined" ? joinStorageKey : null);
    } catch {
      // Joining still works when optional session persistence is unavailable.
      setJoinedSession(null);
    }
  }, [joinStorageKey]);

  function setClassroomJoined(nextJoined: boolean) {
    try {
      if (nextJoined) window.sessionStorage.setItem(joinStorageKey, "joined");
      else window.sessionStorage.removeItem(joinStorageKey);
    } catch {
      // Keep the current tab usable even if the browser blocks storage.
    }
    setJoinedSession(nextJoined ? joinStorageKey : null);
    if (!nextJoined) {
      setSelectedSquare(null);
      setFocused(false);
    }
  }

  const refresh = useCallback(async () => {
    try { setWorkspace(await fetchWorkspace(sessionId)); }
    catch (cause) { setError(cause instanceof Error ? cause.message : "The classroom could not be loaded."); }
  }, [sessionId]);

  useEffect(() => {
    void refresh();
    const sync = () => void refresh();
    window.addEventListener("storage", sync);
    window.addEventListener("kingsgambit:demo-change", sync);
    return () => {
      window.removeEventListener("storage", sync);
      window.removeEventListener("kingsgambit:demo-change", sync);
    };
  }, [refresh]);

  if (!workspace) return <section className="kg-classroom classroom-access">{error ? <><h2>We could not open this classroom.</h2><p role="alert">{error}</p><button className="button button-dark" onClick={() => void refresh()}>Try again</button></> : <p role="status">Preparing your chessboard…</p>}</section>;

  const lesson = workspace.classes.find((item) => item.id === sessionId);
  const student = workspace.students.find((item) => item.id === user.id);
  const inJoinWindow = lesson && Date.now() >= Date.parse(lesson.startsAt) - 15 * 60_000 && Date.now() <= Date.parse(lesson.endsAt) + 15 * 60_000;
  const permitted = lesson && lesson.status === "scheduled" && inJoinWindow && (teacher || observer || (user.roles.includes("student") && lesson.studentIds.includes(user.id) && student?.consent));
  const canMove = !!permitted && joined && (teacher || (user.roles.includes("student") && workspace.board.studentControl)) && !busy;
  const game = new Chess();
  try { if (workspace.board.pgn) game.loadPgn(workspace.board.pgn); else game.load(workspace.board.fen); }
  catch { game.load(workspace.board.fen); }
  const moves = game.history();
  const turn = game.turn() === "w" ? "White" : "Black";
  const gameState = game.isCheckmate() ? "Checkmate" : game.isDraw() ? "Draw" : `${turn} to move${game.isCheck() ? " · check" : ""}`;

  async function save(nextGame: Chess, message = "") {
    if (!workspace || mutationPending.current) return;
    mutationPending.current = true;
    setBusy(true);
    setError("");
    setNotice("");
    setSelectedSquare(null);
    try {
      setWorkspace(await demoAction("saveBoard", { classId: sessionId, fen: nextGame.fen(), pgn: nextGame.pgn(), revision: workspace.board.revision }));
      setNotice(message);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "The position could not be saved.");
      await refresh();
    } finally { mutationPending.current = false; setBusy(false); }
  }

  function play(from: string, to: string) {
    if (!canMove || !/^[a-h][1-8]$/.test(from) || !/^[a-h][1-8]$/.test(to)) return false;
    try {
      game.move({ from: from as Square, to: to as Square, promotion });
      void save(game);
      return true;
    } catch { setError("That move is not legal in this position. Try another move."); return false; }
  }

  async function submitMove(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const input = String(new FormData(form).get("move") || "").trim();
    if (!canMove) return;
    try { game.move(input); await save(game); form.reset(); }
    catch { setError("Enter a legal move such as e4, Nf3 or e2e4."); }
  }

  function downloadPGN() {
    const blob = new Blob([game.pgn()], { type: "application/x-chess-pgn" });
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = "kingsgambit-lesson.pgn";
    anchor.click();
    URL.revokeObjectURL(url);
    setNotice("Your lesson PGN has been downloaded.");
  }

  if (!permitted) return (
    <section className="kg-classroom classroom-access">
      <span className="eyebrow">Classroom access</span>
      <h2>{!lesson ? "This class could not be found." : !inJoinWindow ? "Your classroom opens shortly before class." : lesson.status !== "scheduled" ? "This class has ended." : "Your guardian needs to complete consent."}</h2>
      <p>{lesson ? "Return to your schedule to check the class time and student access. The room opens 15 minutes before class." : "Open an assigned class from your schedule to join the correct room."}</p>
      <Link className="button button-dark" href={landingPathForRoles(user.roles)}>Back to schedule</Link>
    </section>
  );

  return (
    <section className={`classroom-client kg-classroom ${focused ? "classroom-focused" : ""}`} aria-label="Local demo classroom">
      <header className="classroom-room-header">
        <Link href={landingPathForRoles(user.roles)} className="classroom-brand" aria-label="Return to your workspace">
          <BrandLogo size={42} />
          <span>KingsGambit<small>2700 · CLASSROOM</small></span>
        </Link>
        <div className="classroom-room-title"><h1>{lesson.title}</h1><p>{lesson.format} · {lesson.coach}</p></div>
        <div className="classroom-connection-status" role="status"><span className={`connection-dot connection-${joined ? "ready" : "idle"}`} aria-hidden="true" />{joined ? "In classroom" : "Ready to join"}</div>
      </header>
      <p className="demo-classroom-banner">Local classroom demo · Board changes sync between tabs on this browser. Video and audio are not connected.</p>
      {!joined ? (
        <div className="classroom-preflight-panel">
          <div className="classroom-preflight-copy">
            <span className="eyebrow">Make yourself comfortable</span>
            <h2>Your next move<br />starts here.</h2>
            <p>{lesson.coach} · {lesson.format}. Your lesson board is ready, with no camera or microphone needed for this local demo.</p>
            <div className="classroom-join-actions"><button className="button button-dark" onClick={() => setClassroomJoined(true)}>Join classroom board <Icon name="arrow" size={18} /></button></div>
            <p className="classroom-preflight-footnote"><Icon name="shield" size={16} /> A shared board. A little focus. Room to grow.</p>
          </div>
          <aside className="classroom-device-note">
            <span className="classroom-welcome-piece" aria-hidden="true">♞</span>
            <span className="eyebrow">One good idea at a time</span>
            <h3>A focused space<br />to learn.</h3>
            <p>Look at the position. Explain your thinking. Discover what your next move makes possible.</p>
            <dl><div><dt>Your coach</dt><dd>{lesson.coach}</dd></div><div><dt>Lesson format</dt><dd>{lesson.format}</dd></div></dl>
          </aside>
        </div>
      ) : (
        <>
          <div className="classroom-stage">
            <div className="classroom-board-column">
              <div className="classroom-board-heading"><div><span className="eyebrow">Explore. Think. Play.</span><h2>Your next good move.</h2></div><span className="classroom-turn-pill">{gameState}</span></div>
              <div className="classroom-board-frame">
                <div className="classroom-board-label"><span className={`classroom-piece-dot ${orientation === "white" ? "is-black" : ""}`} /><strong>{orientation === "white" ? "Black" : "White"} pieces</strong><small>Shared lesson board</small></div>
                <div className="classroom-board-live" aria-label={`Chessboard at revision ${workspace.board.revision}`}>
                  <Chessboard options={{
                    id: `demo-board-${sessionId}`,
                    position: workspace.board.fen,
                    boardOrientation: orientation,
                    allowDragging: canMove,
                    allowDrawingArrows: teacher,
                    darkSquareStyle: { backgroundColor: "var(--board-dark, #B08C5C)" },
                    lightSquareStyle: { backgroundColor: "var(--board-light, #EDDEC1)" },
                    boardStyle: { borderRadius: 7, overflow: "hidden" },
                    squareStyles: selectedSquare ? { [selectedSquare]: { backgroundColor: "var(--highlight, #D9C59A)", boxShadow: "inset 0 0 0 3px var(--primary, #8C652D)" } } : {},
                    onPieceDrop: ({ sourceSquare, targetSquare }) => { if (targetSquare) play(sourceSquare, targetSquare); return false; },
                    onSquareClick: ({ square }) => {
                      if (!canMove) return;
                      if (selectedSquare && selectedSquare !== square) {
                        if (play(selectedSquare, square)) setSelectedSquare(null);
                        else setSelectedSquare(square);
                      } else setSelectedSquare(square);
                    },
                  }} />
                </div>
                <div className="classroom-board-label"><span className={`classroom-piece-dot ${orientation === "black" ? "is-black" : ""}`} /><strong>{orientation === "white" ? "White" : "Black"} pieces</strong><small>Revision {workspace.board.revision}</small></div>
              </div>
              <div className="classroom-board-actions">
                <button type="button" onClick={() => setOrientation((current) => current === "white" ? "black" : "white")}><Icon name="settings" size={17} /> Flip board</button>
                <button type="button" onClick={() => setFocused((current) => !current)} aria-pressed={focused}><Icon name="expand" size={17} />{focused ? "Show lesson tools" : "Focus board"}</button>
              </div>
              <p className="demo-board-instruction"><Icon name="spark" size={18} /><span>{canMove ? "Drag a piece, click its starting and destination squares, or use move notation in the lesson tools." : "Watch the position. Your coach controls when students can move."}</span></p>
              {focused && error && <p className="classroom-error" role="alert">{error}</p>}
              {focused && notice && <p role="status">{notice}</p>}
            </div>
            <aside className="classroom-side-panel" aria-label="Lesson tools">
              <div className="classroom-coach-card">
                <span className="classroom-coach-avatar" aria-hidden="true">{lesson.coach.replace(/^Coach\s+/i, "").split(" ").map((part) => part[0]).slice(0, 2).join("")}</span>
                <div><small>Your chess guide</small><strong>{lesson.coach}</strong><span>Board-only lesson</span></div>
                <Icon name="book" size={22} />
              </div>
              <section className="classroom-state-card">
                <span className="eyebrow">At the board</span><h3>{gameState}</h3>
                <p>{teacher ? "Guide the lesson, then give the student a turn." : workspace.board.studentControl ? "You have board control. Take your time." : "Your coach is demonstrating. Watch for checks, captures and threats."}</p>
                {teacher && <label className="demo-classroom-control"><input type="checkbox" checked={workspace.board.studentControl} disabled={busy} onChange={async (event) => {
                  setBusy(true); setError("");
                  try { setWorkspace(await demoAction("setStudentControl", { classId: sessionId, enabled: event.target.checked })); }
                  catch (cause) { setError(cause instanceof Error ? cause.message : "Board control could not be updated."); }
                  finally { setBusy(false); }
                }} /> Allow student moves</label>}
                {error && <p className="classroom-error" role="alert">{error}</p>}
                {notice && <p role="status">{notice}</p>}
              </section>
              <section className="classroom-controls-card">
                <span className="eyebrow">Your move</span>
                <form className="demo-classroom-form" onSubmit={submitMove}>
                  <div className="classroom-move-entry"><label>Move notation<input name="move" placeholder="e4, Nf3 or e2e4" required disabled={!canMove} autoComplete="off" /></label><button className="button button-dark" disabled={!canMove}>Play move</button></div>
                  <label className="classroom-promotion">Promote a pawn to<select value={promotion} onChange={(event) => setPromotion(event.target.value)} disabled={!canMove}><option value="q">Queen</option><option value="r">Rook</option><option value="b">Bishop</option><option value="n">Knight</option></select></label>
                </form>
                <div className="demo-classroom-tools">
                  {teacher && <><button disabled={busy || moves.length === 0} onClick={() => { game.undo(); void save(game, "Last move taken back."); }}>Undo move</button><button disabled={busy} onClick={() => void save(new Chess(), "Board reset to the starting position.")}>Reset board</button></>}
                  <button onClick={downloadPGN} disabled={moves.length === 0}><Icon name="download" size={14} /> Download PGN</button>
                </div>
                {teacher && <details className="workspace-details"><summary>Load a position or game</summary><form className="demo-classroom-form" onSubmit={async (event) => {
                  event.preventDefault();
                  const values = new FormData(event.currentTarget);
                  try {
                    const nextGame = new Chess();
                    const input = String(values.get("position") || "").trim();
                    if (input.includes("/") && !input.startsWith("[")) nextGame.load(input);
                    else nextGame.loadPgn(input);
                    await save(nextGame, "Lesson position loaded.");
                  } catch { setError("That FEN or PGN could not be read. Check the notation and try again."); }
                }}><label>FEN or PGN<textarea name="position" required rows={4} /></label><button className="button button-dark" disabled={busy}>Load position</button></form></details>}
              </section>
              <section className="classroom-moves-card">
                <div><span className="eyebrow">Move record</span><strong>{moves.length} {moves.length === 1 ? "move" : "moves"}</strong></div>
                {moves.length ? <ol>{moves.map((move, index) => <li className={index === moves.length - 1 ? "is-latest" : ""} key={`${index}-${move}`}><span>{Math.floor(index / 2) + 1}{index % 2 ? "…" : "."}</span>{move}</li>)}</ol> : <p>Your lesson moves will appear here.</p>}
              </section>
            </aside>
          </div>
          <footer className="classroom-room-footer">
            <span><Icon name="shield" size={16} /> Camera, microphone and recording are off.</span>
            <div className="classroom-session-actions"><button onClick={() => setClassroomJoined(false)}>Leave board</button><Link href={landingPathForRoles(user.roles)}>Back to workspace <Icon name="arrow" size={16} /></Link></div>
          </footer>
        </>
      )}
    </section>
  );
}
