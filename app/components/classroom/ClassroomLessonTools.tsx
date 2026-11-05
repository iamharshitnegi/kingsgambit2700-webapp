"use client";

import { Chess } from "chess.js";
import { useEffect, useState, type FormEvent } from "react";
import { apiFetch } from "@/app/lib/api/client";
import type { AcademyStudent } from "@/app/lib/academy-types";
import type { ClassroomBoardState, ClassroomToolCommand } from "@/app/lib/realtime/classroom-protocol";
import { exportLessonPGN, importLessonPGN } from "@/app/lib/realtime/lesson-game";
import "./classroom-tools.css";

type Props = {
  board: ClassroomBoardState;
  canControl: boolean;
  disabled: boolean;
  onCommand: (command: ClassroomToolCommand) => boolean;
  students: AcademyStudent[];
  saveStudents: AcademyStudent[];
  title: string;
};

export default function ClassroomLessonTools({ board, canControl, disabled, onCommand, students, saveStudents, title }: Props) {
  const [clock, setClock] = useState(() => Date.now());
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [saving, setSaving] = useState(false);
  const [pgn, setPGN] = useState("");
  const [fen, setFEN] = useState("");
  const [selectedStudent, setSelectedStudent] = useState("");
  const lesson = board.lesson;
  const deadline = lesson?.countdownEndsAt ? Date.parse(lesson.countdownEndsAt) : undefined;
  const remaining = deadline ? Math.max(0, Math.ceil((deadline - clock) / 1000)) : 0;

  useEffect(() => {
    if (!deadline) return;
    const timer = window.setInterval(() => setClock(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [deadline]);

  function run(command: ClassroomToolCommand) { setError(""); setNotice(""); onCommand(command); }
  function importGame() {
    try { run({ type: "loadPosition", ...importLessonPGN(pgn) }); }
    catch (error) { setError(error instanceof Error ? error.message : "This PGN could not be imported."); }
  }
  function loadPosition() {
    try { const position = new Chess(fen.trim()); run({ type: "loadPosition", fen: position.fen(), initialFen: position.fen(), moves: [] }); }
    catch { setError("Enter a valid FEN with both kings and a legal board position."); }
  }
  function downloadGame() {
    try {
      const content = exportLessonPGN(board, title);
      const url = URL.createObjectURL(new Blob([content], { type: "application/x-chess-pgn" }));
      const link = document.createElement("a"); link.href = url; link.download = "lesson-game.pgn"; link.click();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      setError(""); setNotice("Lesson PGN downloaded.");
    } catch (error) { setError(error instanceof Error ? error.message : "Could not export the lesson."); }
  }
  async function saveGame(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const studentId = String(form.get("studentId") || "");
    if (!studentId) return;
    setSaving(true); setError(""); setNotice("");
    try {
      await apiFetch("/learning/actions", { method: "POST", body: JSON.stringify({ action: "saveGame", studentId, classId: board.sessionId, title: String(form.get("title") || title), pgn: exportLessonPGN(board, title), source: "lesson", shared: true, playedAt: new Date().toISOString() }) });
      setNotice("Lesson game saved to the learner’s game reviews.");
    } catch (error) { setError(error instanceof Error ? error.message : "Could not save this lesson game."); }
    finally { setSaving(false); }
  }

  return <div className="classroom-lesson-tools">
    {deadline && <section className={`classroom-countdown ${remaining === 0 ? "is-finished" : ""}`} role="timer" aria-label={lesson?.countdownLabel || "Lesson countdown"}>
      <span>{lesson?.countdownLabel || "Time to think"}</span><strong>{Math.floor(remaining / 60)}:{String(remaining % 60).padStart(2, "0")}</strong><small>{remaining ? "Take your time. Find your idea." : "Time is up — share your thinking."}</small>
    </section>}
    {canControl && <>
      <section className="classroom-tool-card"><h3>Guide the board</h3>
        <label>Lesson mode<select aria-label="Lesson mode" value={board.mode} disabled={disabled} onChange={(event) => {
          if (event.target.value === "student_turn") { if (students[0]?.userId) run({ type: "setMode", mode: "student_turn", activeActorId: students[0].userId }); }
          else run({ type: "setMode", mode: event.target.value });
        }}>
          <option value="demonstration">Coach demonstration</option><option value="free_analysis">Everyone can move</option><option value="student_turn" disabled={!students.length}>One learner’s turn</option><option value="game_review">Game review</option>
          {!["demonstration", "free_analysis", "student_turn", "game_review"].includes(board.mode) && <option value={board.mode}>{board.mode.replaceAll("_", " ")}</option>}
        </select></label>
        {board.mode === "student_turn" && <label>Active learner<select aria-label="Active learner" value={board.activeActorId || ""} disabled={disabled} onChange={(event) => run({ type: "setMode", mode: "student_turn", activeActorId: event.target.value })}>{students.map((student) => <option key={student.id} value={student.userId}>{student.name}</option>)}</select></label>}
        <div className="classroom-tool-buttons"><button type="button" disabled={disabled || (lesson?.positions.length ?? 0) < 2} onClick={() => run({ type: "undo" })}>Undo last move</button><button type="button" disabled={disabled || !lesson?.annotations.length} onClick={() => run({ type: "annotation", annotations: [] })}>Clear markings</button></div>
      </section>
      <details className="classroom-tool-card"><summary>Checkpoints <span>{lesson?.checkpoints.length ?? 0}/8</span></summary><p>Save a position and its markings, then return to it later in this lesson.</p>
        <form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); run({ type: "checkpoint", title: String(form.get("title") || "") }); }}>
          <label>Checkpoint name<input name="title" required maxLength={100} placeholder="The key position" /></label><button disabled={disabled || (lesson?.checkpoints.length ?? 0) >= 8}>Save checkpoint</button>
        </form>
        {!!lesson?.checkpoints.length && <ul className="classroom-checkpoint-list">{lesson.checkpoints.map((checkpoint) => <li key={checkpoint.id}><span>{checkpoint.title}</span><button disabled={disabled} onClick={() => run({ type: "restoreCheckpoint", checkpointId: checkpoint.id })} aria-label={`Restore ${checkpoint.title}`}>Restore</button><button disabled={disabled} onClick={() => run({ type: "deleteCheckpoint", checkpointId: checkpoint.id })} aria-label={`Delete ${checkpoint.title}`}>×</button></li>)}</ul>}
      </details>
      <details className="classroom-tool-card"><summary>Set a thinking timer</summary><form onSubmit={(event) => { event.preventDefault(); const form = new FormData(event.currentTarget); run({ type: "countdown", seconds: Number(form.get("minutes")) * 60, label: String(form.get("label") || "") }); }}>
        <label>Prompt<input name="label" maxLength={100} placeholder="Find the strongest move" /></label><label>Minutes<input name="minutes" type="number" min={1} max={180} step={1} defaultValue={3} required /></label><div className="classroom-tool-buttons"><button disabled={disabled}>Start timer</button>{deadline && <button type="button" disabled={disabled} onClick={() => run({ type: "countdown", seconds: 0 })}>Stop timer</button>}</div>
      </form></details>
      <details className="classroom-tool-card"><summary>Load a game or position</summary><p>This replaces the shared board. Save a checkpoint first if you want to return here.</p>
        <label>PGN game<textarea aria-label="PGN game" rows={5} value={pgn} onChange={(event) => setPGN(event.target.value)} maxLength={64000} placeholder="1. e4 e5 2. Nf3…" /></label>
        <label>Choose PGN file<input type="file" accept=".pgn,text/plain,application/x-chess-pgn" onChange={async (event) => { const file = event.target.files?.[0]; if (!file) return; if (file.size > 64000) { setError("Choose a PGN of up to 64 KB."); return; } try { setPGN(await file.text()); } catch { setError("The PGN file could not be read."); } }} /></label><button type="button" disabled={disabled || !pgn.trim()} onClick={importGame}>Import game to board</button>
        <label>FEN position<textarea aria-label="FEN position" rows={2} value={fen} onChange={(event) => setFEN(event.target.value)} /></label><div className="classroom-tool-buttons"><button type="button" disabled={disabled || !fen.trim()} onClick={loadPosition}>Load position</button><button type="button" disabled={disabled} onClick={() => { const position = new Chess().fen(); run({ type: "loadPosition", fen: position, initialFen: position, moves: [] }); }}>Starting position</button></div>
      </details>
    </>}
    <details className="classroom-tool-card"><summary>Keep this lesson game</summary><button type="button" onClick={downloadGame}>Download PGN</button>
      {!!saveStudents.length && <form onSubmit={saveGame}><label>Learner<select name="studentId" value={selectedStudent || saveStudents[0]?.id || ""} onChange={(event) => setSelectedStudent(event.target.value)} required>{saveStudents.map((student) => <option key={student.id} value={student.id}>{student.name}</option>)}</select></label><label>Game title<input name="title" defaultValue={title || "Lesson game"} required maxLength={180} /></label><button disabled={saving}>{saving ? "Saving…" : "Save to game reviews"}</button></form>}
    </details>
    {error && <p className="classroom-error" role="alert">{error}</p>}{notice && <p role="status">{notice}</p>}
  </div>;
}
