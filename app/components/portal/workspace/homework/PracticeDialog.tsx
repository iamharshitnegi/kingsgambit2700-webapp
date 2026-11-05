"use client";

import { useEffect, useRef, useState } from "react";
import { Chess } from "chess.js";
import { Icon } from "@/app/components/Icon";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import type { Assignment, LearningAction, LearningWorkspace } from "@/app/lib/academy-types";
import { FileDownload, FileUpload } from "../LearningFiles";
import { AssignedMaterials, HomeworkDialog } from "./HomeworkUI";
import { currentAttempt } from "./model";
import s from "./homework.module.css";

function PuzzlePractice({ assignment, act, busy, setBusy }: { assignment: Assignment; act: LearningAction; busy: boolean; setBusy: (busy: boolean) => void }) {
  const attempt = currentAttempt(assignment);
  const [index, setIndex] = useState(() => Math.max(0, assignment.puzzles.findIndex((_, i) => !attempt?.puzzles[i]?.solved && !attempt?.puzzles[i]?.revealed)));
  const [selected, setSelected] = useState<string | null>(null);
  const [notation, setNotation] = useState("");
  const [promotion, setPromotion] = useState("q");
  const [error, setError] = useState("");
  const elapsed = useRef(0);
  const active = useRef(0);
  const pending = useRef(false);
  const puzzle = assignment.puzzles[index];
  const progress = attempt?.puzzles[index];
  const done = !!progress?.solved || !!progress?.revealed;
  useEffect(() => {
    elapsed.current = 0; active.current = Date.now();
    if (done) return;
    const timer = window.setInterval(() => { if (!document.hidden && Date.now() - active.current < 60000) elapsed.current++; }, 1000);
    return () => window.clearInterval(timer);
  }, [assignment.id, index, done]);
  if (!puzzle) return <p>No puzzle positions are available.</p>;
  const game = new Chess(puzzle.fen);
  for (const move of progress?.moves || []) game.move({ from: move.slice(0, 2), to: move.slice(2, 4), promotion: move[4] });
  async function send(action: string, move?: string) {
    if (busy || done || pending.current) return;
    pending.current = true; setBusy(true); setError("");
    const seconds = Math.min(60, elapsed.current);
    try {
      if (await act(action, { id: assignment.id, version: assignment.version, puzzleIndex: index, elapsedSeconds: seconds, ...(move ? { move } : {}) }, action === "puzzleMove" ? "Move checked." : "Practice updated.")) { elapsed.current = Math.max(0, elapsed.current - seconds); setNotation(""); }
    } finally { pending.current = false; setBusy(false); setSelected(null); }
  }
  function play(from: string, to: string) {
    if (busy || done) return false;
    const piece = game.get(from as Parameters<Chess["get"]>[0]);
    void send("puzzleMove", from + to + (piece?.type === "p" && /[18]$/.test(to) ? promotion : ""));
    return true;
  }
  function choose(value: number) { setIndex(value); setSelected(null); setNotation(""); setError(""); }
  return <section className={s.puzzlePractice} onPointerDown={() => { active.current = Date.now(); }} onKeyDown={() => { active.current = Date.now(); }}>
    <div><div className={s.boardHeading}><span>{game.turn() === "w" ? "White" : "Black"} to move</span><span>{index + 1} of {assignment.puzzles.length}</span></div><div className={s.practiceBoard}><Chessboard options={{ id: `homework-${assignment.id}-${index}`, position: game.fen(), boardOrientation: puzzle.fen.split(" ")[1] === "b" ? "black" : "white", allowDragging: !busy && !done, darkSquareStyle: { backgroundColor: "#a98b64" }, lightSquareStyle: { backgroundColor: "#f2e6d3" }, onPieceDrop: ({ sourceSquare, targetSquare }) => !!targetSquare && play(sourceSquare, targetSquare), onSquareClick: ({ square }) => { if (busy || done) return; if (selected && selected !== square) play(selected, square); else setSelected(square); }, squareStyles: selected ? { [selected]: { boxShadow: "inset 0 0 0 3px #82623f" } } : {} }} /></div><div className={s.puzzleNavigation}><button type="button" className={s.iconButton} aria-label="Previous puzzle" disabled={!index || busy} onClick={() => choose(index - 1)}><Icon name="left" size={17} /></button><label className="sr-only" htmlFor="homework-puzzle">Puzzle</label><select id="homework-puzzle" value={index} disabled={busy} onChange={event => choose(Number(event.target.value))}>{assignment.puzzles.map((puzzle, number) => <option key={number} value={number}>{number + 1}. {puzzle.title}{attempt?.puzzles[number]?.solved ? " · Solved" : attempt?.puzzles[number]?.revealed ? " · Revealed" : ""}</option>)}</select><button className={s.iconButton} type="button" aria-label="Next puzzle" disabled={index === assignment.puzzles.length - 1 || busy} onClick={() => choose(index + 1)}><Icon name="right" size={17} /></button></div></div>
    <div className={s.puzzleGuidance}><span className={s.eyebrow}>FIND YOUR NEXT GOOD MOVE</span><h3>{puzzle.title}</h3><p>Take your time. Look for forcing moves and check your opponent’s replies.</p><div className={s.progressTrack}><span style={{ width: `${(attempt?.puzzles.filter(item => item.solved || item.revealed).length || 0) / assignment.puzzles.length * 100}%` }} /></div><small>{attempt?.metrics.completed || 0} of {assignment.puzzles.length} solved{attempt?.metrics.reveals ? ` · ${attempt.metrics.reveals} revealed` : ""}</small>
      <div className={done ? s.successMessage : s.moveFeedback} role="status">{progress?.solved ? "Solved — well done." : progress?.revealed ? "Solution revealed. Review the line, then try another position." : progress?.hint || "Move a piece on the board, or enter your move below."}</div>
      {progress?.revealed && progress.hint && <p className={s.hint}>{progress.hint}</p>}
      {!done && <><form className={s.moveForm} onSubmit={event => { event.preventDefault(); setError(""); try { const test = new Chess(game.fen()); const token = notation.trim(); const move = /^[a-h][1-8][a-h][1-8][qrbn]?$/.test(token) ? test.move({ from: token.slice(0, 2), to: token.slice(2, 4), promotion: token[4] || promotion }) : test.move(token); void send("puzzleMove", move.from + move.to + (move.promotion || "")); } catch { setError("That move isn’t legal in this position. Try chess notation such as e4 or square coordinates such as e2e4."); } }}><label htmlFor="homework-move">Your move</label><div className={s.moveInput}><input id="homework-move" value={notation} onChange={event => setNotation(event.target.value)} placeholder="e4 or e2e4" required disabled={busy} autoComplete="off" /><button className={s.primaryButton} disabled={busy}>Check move</button></div><label className={s.promotion}>Promote to<select value={promotion} disabled={busy} onChange={event => setPromotion(event.target.value)}><option value="q">Queen</option><option value="r">Rook</option><option value="b">Bishop</option><option value="n">Knight</option></select></label></form><div className={s.inlineActions}><button className={s.textButton} type="button" disabled={busy} onClick={() => void send("puzzleHint")}><Icon name="help" size={15} />Get a hint</button><button className={s.textButton} type="button" disabled={busy} onClick={() => void send("puzzleReveal")}>Reveal solution</button></div></>}
      {done && index < assignment.puzzles.length - 1 && <button className={s.primaryButton} type="button" disabled={busy} onClick={() => choose(index + 1)}>Next position <Icon name="arrow" size={16} /></button>}
      {error && <p className={s.error} role="alert">{error}</p>}
    </div>
  </section>;
}

export default function PracticeDialog({ assignment, act, close, submitted, onUploaded, draft, keepDraft }: { assignment: Assignment; act: LearningAction; close: () => void; submitted: () => void; onUploaded?: (workspace: LearningWorkspace) => void; draft?: string; keepDraft: (answer: string) => void }) {
  const [answer, setAnswer] = useState(draft ?? (assignment.attempts.at(-1)?.status === "returned" ? assignment.attempts.at(-1)?.answer : "") ?? "");
  const [busy, setBusy] = useState(false);
  const [uploading, setUploading] = useState(false);
  const attempt = currentAttempt(assignment);
  const previousFeedback = [...assignment.attempts].reverse().find(item => item.feedback)?.feedback;
  const required = !["puzzle_set", "file_upload"].includes(assignment.kind);
  const remaining = assignment.puzzles.length - (attempt?.puzzles.filter(item => item.solved || item.revealed).length || 0);
  const disabled = busy || uploading;
  function dismiss() { if (!disabled) { keepDraft(answer); close(); } }
  return <HomeworkDialog title={assignment.title} subtitle="YOUR PRACTICE SPACE" close={dismiss} wide={assignment.kind === "puzzle_set"} busy={disabled}>
    <div className={s.practiceContent}>{previousFeedback && <div className={s.feedback}><Icon name="message" size={18} /><div><strong>A note from your coach</strong><p>{previousFeedback}</p></div></div>}
      <p className={s.prose}>{assignment.description}</p><AssignedMaterials assignment={assignment} />
      {assignment.kind === "puzzle_set" && <PuzzlePractice assignment={assignment} act={act} busy={disabled} setBusy={setBusy} />}
      {!!attempt?.files?.length && <div className={s.fileList}>{attempt.files.map(file => <FileDownload key={file.id} id={file.id} filename={file.filename} />)}</div>}
      <details className={s.disclosure} open={assignment.kind === "file_upload" || undefined}><summary>Attach practice evidence</summary><FileUpload assignmentId={assignment.id} onUploaded={onUploaded} onBusyChange={setUploading} /></details>
      <form className={s.responseForm} onSubmit={async event => { event.preventDefault(); if (disabled) return; setBusy(true); try { if (await act("submitAssignment", { id: assignment.id, version: assignment.version, answer: answer.trim() }, "Practice submitted for review.")) { keepDraft(""); submitted(); } } finally { setBusy(false); } }}>
        <label>Your thinking{!required && <span className={s.optional}> (optional)</span>}<textarea value={answer} onChange={event => setAnswer(event.target.value)} required={required} minLength={required ? 5 : undefined} maxLength={10000} rows={assignment.kind === "puzzle_set" ? 3 : 5} disabled={disabled} placeholder="Explain the idea and what you learned…" /></label>
        {assignment.kind === "puzzle_set" && remaining > 0 && <p className={s.fieldHint}>{remaining} {remaining === 1 ? "position is" : "positions are"} unfinished. You can continue practising or submit your current attempt.</p>}
        {assignment.kind === "file_upload" && !attempt?.files?.length && <p className={s.fieldHint}>Upload your practice evidence before submitting.</p>}
        <footer className={s.dialogFooter}><button className={s.secondaryButton} type="button" disabled={disabled} onClick={dismiss}>Continue later</button><button className={s.primaryButton} disabled={disabled || (required && answer.trim().length < 5) || (assignment.kind === "file_upload" && !attempt?.files?.length)}>Submit practice <Icon name="arrow" size={16} /></button></footer>
      </form>
    </div><div className={s.dialogNote}>{assignment.kind === "puzzle_set" ? "Puzzle progress saves automatically. " : ""}Your unsubmitted reflection stays on this page until you leave or reload.</div>
  </HomeworkDialog>;
}
