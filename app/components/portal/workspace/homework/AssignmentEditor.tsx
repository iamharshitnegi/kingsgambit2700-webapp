"use client";

import { useRef, useState } from "react";
import { Chess } from "chess.js";
import { Icon } from "@/app/components/Icon";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import type { AcademyWorkspaceData, Assignment, LearningAction, LearningWorkspace, Puzzle } from "@/app/lib/academy-types";
import { HomeworkDialog } from "./HomeworkUI";
import { activities, activityFor, dueLabel, localDateInput } from "./model";
import s from "./homework.module.css";

const starters = [
  { title: "A plan for your first 10 moves", kind: "study", description: "List three goals for the opening. Choose which pieces you would develop first. Explain when you would castle and why." },
  { title: "Find the turning point", kind: "game_review", description: "Think back to a recent game. Describe one decision that changed the position, explain your opponent’s threat, and suggest a better plan." },
  { title: "The art of the checkmate", kind: "puzzle_set", description: "Find the forcing move in each position. Check every escape square before you play. When you finish, share what you noticed." },
];
const starterPuzzle: Puzzle = { title: "A back-rank finish", fen: "6k1/5ppp/8/8/8/8/5PPP/4R1K1 w - - 0 1", solutions: [["e1e8"]] };

function PuzzleComposer({ puzzles, change }: { puzzles: Puzzle[]; change: (puzzles: Puzzle[]) => void }) {
  const [open, setOpen] = useState(!puzzles.length);
  const [title, setTitle] = useState("");
  const [fen, setFen] = useState(starterPuzzle.fen);
  const [lines, setLines] = useState("");
  const [error, setError] = useState("");
  let validFen = false;
  try { new Chess(fen); validFen = true; } catch { /* Validate again with useful feedback when adding. */ }
  function add() {
    setError("");
    try {
      if (!title.trim()) throw new Error("Give this puzzle a title.");
      if (puzzles.length >= 100) throw new Error("Use up to 100 puzzles per assignment.");
      new Chess(fen);
      const solutions = lines.trim().split("\n").filter(line => line.trim()).map(line => {
        const game = new Chess(fen);
        const tokens = line.replace(/\d+\.(\.\.)?/g, " ").trim().split(/\s+/).filter(Boolean);
        if (!tokens.length || tokens.length > 64) throw new Error("Use 1–64 moves per solution line.");
        return tokens.map(token => {
          const move = /^[a-h][1-8][a-h][1-8][qrbn]?$/.test(token) ? game.move({ from: token.slice(0, 2), to: token.slice(2, 4), promotion: token[4] }) : game.move(token);
          return move.from + move.to + (move.promotion || "");
        });
      });
      if (!solutions.length || solutions.length > 20) throw new Error("Add 1–20 complete solution lines, one per line.");
      change([...puzzles, { title: title.trim(), fen, solutions }]);
      setTitle(""); setLines(""); setOpen(false);
    } catch (cause) { setError(cause instanceof Error ? cause.message : "Check the position and solution moves."); }
  }
  return <section className={s.composer}><div className={s.sectionHeading}><h3>Puzzle set</h3><span>{puzzles.length}/100 positions</span></div>
    {puzzles.map((puzzle, index) => <div className={s.puzzleRow} key={`${index}-${puzzle.title}`}><span className={s.stepNumber}>{index + 1}</span><div><strong>{puzzle.title}</strong><small>{puzzle.solutions?.length || 0} solution line(s)</small></div><button className={s.iconButton} type="button" aria-label={`Remove ${puzzle.title}`} onClick={() => change(puzzles.filter((_, i) => i !== index))}><Icon name="close" size={16} /></button></div>)}
    {open && <div className={s.puzzleEditor}><div className={s.puzzleFields}><label>Puzzle title<input value={title} onChange={event => setTitle(event.target.value)} maxLength={200} placeholder="e.g. A back-rank finish" /></label><label>Starting position (FEN)<textarea rows={2} value={fen} onChange={event => setFen(event.target.value)} /></label><label>Solution lines<textarea rows={3} value={lines} onChange={event => setLines(event.target.value)} placeholder="Re8#" /><small>Use chess notation or square coordinates. Include the opponent’s replies. One complete alternative per line.</small></label></div><div className={s.composerBoard} aria-label="Puzzle position preview">{validFen ? <Chessboard options={{ id: "new-homework-position", position: fen, allowDragging: false, boardOrientation: fen.split(" ")[1] === "b" ? "black" : "white", darkSquareStyle: { backgroundColor: "#a98b64" }, lightSquareStyle: { backgroundColor: "#f2e6d3" } }} /> : <p>Enter a valid FEN to preview the position.</p>}</div>{error && <p className={s.error} role="alert">{error}</p>}<div className={s.inlineActions}><button className={s.secondaryButton} type="button" onClick={() => setOpen(false)}>Cancel position</button><button className={s.primaryButton} type="button" onClick={add}>Add position <Icon name="plus" size={15} /></button></div></div>}
    {!open && <button className={s.textButton} type="button" disabled={puzzles.length >= 100} onClick={() => setOpen(true)}><Icon name="plus" size={16} />Add a puzzle</button>}
  </section>;
}

export default function AssignmentEditor({ data, learning, item, act, close, saved }: { data: AcademyWorkspaceData; learning: LearningWorkspace; item?: Assignment; act: LearningAction; close: () => void; saved: (status: string) => void }) {
  const [step, setStep] = useState(0);
  const [kind, setKind] = useState(item?.kind || "study");
  const [title, setTitle] = useState(item?.title || "");
  const [description, setDescription] = useState(item?.description || "");
  const [batch, setBatch] = useState("");
  const [students, setStudents] = useState<string[]>(item ? [item.studentId] : []);
  const [due, setDue] = useState(localDateInput(item?.dueAt));
  const [classId, setClass] = useState(item?.classId || "");
  const [publication, setPublication] = useState("draft");
  const [resources, setResources] = useState(item?.resources?.map(resource => resource.id) || []);
  const [puzzles, setPuzzles] = useState<Puzzle[]>(item?.puzzles || []);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const headingRef = useRef<HTMLHeadingElement>(null);
  const managed = learning.students.filter(student => student.canManage);
  const attached = [...learning.resources.filter(resource => resource.status === "published" || resources.includes(resource.id)), ...(item?.resources || []).filter(resource => !learning.resources.some(candidate => candidate.id === resource.id))];
  const classes = data.classes.filter(lesson => students.length > 0 && students.every(id => lesson.studentIds.includes(id)));
  const selectedActivity = activityFor(kind);
  function go(next: number) { setError(""); setStep(next); window.requestAnimationFrame(() => headingRef.current?.focus()); }
  function next() {
    setError("");
    if (!formRef.current?.reportValidity()) return;
    if (step === 0 && (title.trim().length < 1 || description.trim().length < 5)) { setError("Add a title and at least five characters of instructions."); return; }
    if (step === 0 && kind === "puzzle_set" && !puzzles.length) { setError("Add at least one puzzle before continuing."); return; }
    if (step === 1 && !students.length) { setError("Select at least one learner."); return; }
    if (step === 1 && classId && !classes.some(lesson => lesson.id === classId)) { setError("Choose a class that includes every selected learner, or choose independent practice."); return; }
    go(step + 1);
  }
  async function save() {
    if (busy) return;
    setBusy(true); setError("");
    try {
      const success = await act(item ? "editAssignment" : "createAssignment", {
        ...(item ? { id: item.id, version: item.version } : {}), title: title.trim(), description: description.trim(), kind,
        studentIds: students, classId, resourceIds: resources, puzzles: kind === "puzzle_set" ? puzzles : [],
        dueAt: due ? new Date(due).toISOString() : null, status: item ? "draft" : publication,
      }, item ? "Homework draft updated." : publication === "assigned" ? "Homework assigned to the selected learners." : "Homework saved as a draft.");
      if (success) saved(item ? "draft" : publication);
      else setError("Your homework wasn’t saved. Check the message above and try again.");
    } catch { setError("Your homework wasn’t saved. Try again without leaving this draft."); }
    finally { setBusy(false); }
  }
  return <HomeworkDialog title={item ? "Edit homework draft" : "Create homework"} subtitle="A LITTLE PLANNING. PURPOSEFUL PRACTICE." close={close} wide busy={busy}>
    <div className={s.editorSteps} aria-label="Create homework steps">{["Activity & content", "Learners & due date", "Review & assign"].map((label, index) => <span key={label} className={step === index ? s.currentStep : ""} aria-current={step === index ? "step" : undefined}><b>{index < step ? <Icon name="check" size={13} /> : index + 1}</b>{label}</span>)}</div>
    <form ref={formRef} className={s.editorForm} onSubmit={event => { event.preventDefault(); if (step < 2) next(); else void save(); }}>
      <fieldset disabled={busy}><h3 ref={headingRef} tabIndex={-1} className={s.editorTitle}>{step === 0 ? "What would you like them to practise?" : step === 1 ? "Give this practice a home." : "Ready for the next move."}</h3>
        {step === 0 && <>
          {!item && !title && <div className={s.starters}><span>Start with an example</span>{starters.map(starter => <button key={starter.kind} className={s.starterButton} type="button" onClick={() => { setTitle(starter.title); setDescription(starter.description); setKind(starter.kind); if (starter.kind === "puzzle_set") setPuzzles([structuredClone(starterPuzzle)]); }}><Icon name={activityFor(starter.kind).icon} size={15} />{activityFor(starter.kind).label}</button>)}</div>}
          <fieldset className={s.activityChoices}><legend>Activity</legend>{activities.map(activity => <label key={activity.kind} className={kind === activity.kind ? s.chosenActivity : ""}><input type="radio" name="activity" value={activity.kind} checked={kind === activity.kind} onChange={() => setKind(activity.kind)} /><Icon name={activity.icon} size={21} /><strong>{activity.label}</strong><small>{activity.description}</small></label>)}</fieldset>
          <label>Homework title<input value={title} onChange={event => setTitle(event.target.value)} required maxLength={200} placeholder="e.g. The art of the checkmate" /></label>
          <label>Instructions<textarea value={description} onChange={event => setDescription(event.target.value)} required minLength={5} maxLength={5000} rows={4} placeholder="Describe the challenge, what to practise, and what to submit…" /></label>
          {kind === "puzzle_set" && <PuzzleComposer puzzles={puzzles} change={setPuzzles} />}
          <details className={s.disclosure} open={resources.length > 0 || undefined}><summary>Attach learning material <span>({resources.length}/10)</span></summary><div className={s.resourceChoices}>{attached.length ? attached.map(resource => <label className={s.checkbox} key={resource.id}><input type="checkbox" checked={resources.includes(resource.id)} disabled={!resources.includes(resource.id) && resources.length >= 10} onChange={event => setResources(event.target.checked ? [...resources, resource.id] : resources.filter(id => id !== resource.id))} /><span>{resource.title}<small>Version {resource.version} · {resource.kind}</small></span></label>) : <p>Publish a resource in the library to attach it here. You can also give the full task in the instructions above.</p>}</div><small>Students receive the version you attach, even if the library changes later.</small></details>
        </>}
        {step === 1 && <>
          {item ? <div className={s.assignmentAudience}><Icon name="users" size={20} /><span>Assigned learner<strong>{managed.find(student => student.id === item.studentId)?.name || "Learner"}</strong></span></div> : <><label>Select a batch<select value={batch} onChange={event => { const id = event.target.value; setBatch(id); setStudents(data.batches.find(group => group.id === id)?.studentIds.filter(id => managed.some(student => student.id === id)) || []); setClass(""); }}><option value="">Choose learners individually</option>{data.batches.filter(group => group.studentIds.some(id => managed.some(student => student.id === id))).map(group => <option value={group.id} key={group.id}>{group.name}</option>)}</select></label><fieldset className={s.learnerChoices}><legend>Assign to learners <span>{students.length} selected</span></legend>{managed.map(student => <label className={s.checkbox} key={student.id}><input type="checkbox" checked={students.includes(student.id)} onChange={event => { setStudents(event.target.checked ? [...students, student.id] : students.filter(id => id !== student.id)); setBatch(""); setClass(""); }} /><span>{student.name}</span></label>)}{!managed.length && <p>No learners are available for you to assign homework.</p>}</fieldset></>}
          <div className={s.formColumns}><label>Due date<input type="datetime-local" value={due} onChange={event => setDue(event.target.value)} /><small>Optional · your device’s local time</small></label><label>Related class<select value={classId} onChange={event => setClass(event.target.value)}><option value="">Independent practice</option>{classes.map(lesson => <option key={lesson.id} value={lesson.id}>{lesson.title}</option>)}</select><small>Only classes shared by the selected learners.</small></label></div>
          {!item && <fieldset className={s.publicationChoices}><legend>Publication</legend><label className={s.checkbox}><input type="radio" name="publication" value="draft" checked={publication === "draft"} onChange={() => setPublication("draft")} /><span>Save as a draft<small>Keep it private while you prepare.</small></span></label><label className={s.checkbox}><input type="radio" name="publication" value="assigned" checked={publication === "assigned"} onChange={() => setPublication("assigned")} /><span>Assign to learners<small>Make it available in their homework list.</small></span></label></fieldset>}
        </>}
        {step === 2 && <div className={s.editorReview}><div className={s.reviewTitle}><span className={s.activityIcon}><Icon name={selectedActivity.icon} size={25} /></span><div><span className={s.eyebrow}>{selectedActivity.label}</span><h3>{title}</h3></div></div><p className={s.prose}>{description}</p><dl className={s.reviewFacts}><div><dt>Learners</dt><dd>{students.map(id => managed.find(student => student.id === id)?.name || "Learner").join(", ")}</dd></div><div><dt>Due</dt><dd>{due ? dueLabel(new Date(due).toISOString()) : "No due date"}</dd></div><div><dt>Class</dt><dd>{data.classes.find(lesson => lesson.id === classId)?.title || "Independent practice"}</dd></div><div><dt>Material</dt><dd>{resources.length ? attached.filter(resource => resources.includes(resource.id)).map(resource => resource.title).join(", ") : "Instructions only"}</dd></div>{kind === "puzzle_set" && <div><dt>Puzzles</dt><dd>{puzzles.length} positions · solutions checked</dd></div>}</dl><div className={s.reviewNotice}><Icon name={publication === "assigned" && !item ? "users" : "edit"} size={20} /><p>{publication === "assigned" && !item ? `This creates ${students.length} ${students.length === 1 ? "assignment" : "individual assignments"}. Each learner sees only their own homework and progress.` : "This draft is only visible to authorized teaching staff. Assign it when you’re ready."}</p></div></div>}
        {error && <p className={s.error} role="alert">{error}</p>}
        <footer className={s.dialogFooter}><button className={s.secondaryButton} type="button" onClick={() => step ? go(step - 1) : close()}>{step ? "Back" : "Cancel"}</button><button className={s.primaryButton} type="submit">{busy ? "Saving…" : step < 2 ? "Continue" : item ? "Save changes" : publication === "assigned" ? "Assign homework" : "Save draft"}<Icon name={step < 2 ? "arrow" : "check"} size={16} /></button></footer>
      </fieldset>
    </form>
  </HomeworkDialog>;
}
