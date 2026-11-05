"use client";

import { useContext, useRef, useState } from "react";
import { Icon } from "@/app/components/Icon";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import type { SessionUser } from "@/app/lib/api/types";
import type { AcademyWorkspaceData, Assignment, LearningAction, LearningWorkspace } from "@/app/lib/academy-types";
import { FileDownload } from "./LearningFiles";
import { WorkspaceFeedback } from "./WorkspaceUI";
import AssignmentEditor from "./homework/AssignmentEditor";
import PracticeDialog from "./homework/PracticeDialog";
import { AssignedMaterials, HomeworkDialog, Metrics, Status } from "./homework/HomeworkUI";
import { activities, activityFor, assignmentStatus, canPractice, currentAttempt, dueLabel, isTodo, matchesFilter, statusLabels, type HomeworkFilter } from "./homework/model";
import s from "./homework/homework.module.css";

function AttemptHistory({ assignment }: { assignment: Assignment }) {
  if (!assignment.attempts.length) return null;
  return <details className={s.disclosure}><summary>Attempt history ({assignment.attempts.length})</summary>{[...assignment.attempts].reverse().map(attempt => <section key={attempt.id} className={s.historyEntry}><h4>Attempt {attempt.number} · {statusLabels[attempt.status] || attempt.status}</h4>{attempt.answer && <p className={s.prose}>{attempt.answer}</p>}{attempt.feedback && <div className={s.feedback}><div><strong>Coach feedback</strong><p>{attempt.feedback}</p></div></div>}{!!attempt.files?.length && <div className={s.fileList}>{attempt.files.map(file => <FileDownload key={file.id} id={file.id} filename={file.filename} />)}</div>}{assignment.kind === "puzzle_set" && <><Metrics metrics={attempt.metrics} /><div className={s.tableScroll} role="region" tabIndex={0} aria-label={`Puzzle outcomes for attempt ${attempt.number}`}><table><thead><tr><th>Puzzle</th><th>Outcome</th><th>Incorrect tries</th><th>Hints</th><th>Active seconds</th></tr></thead><tbody>{attempt.puzzles.map((puzzle, index) => <tr key={index}><td>{assignment.puzzles[index]?.title}</td><td>{puzzle.solved ? "Solved" : puzzle.revealed ? "Revealed" : !puzzle.lastActionAt || puzzle.lastActionAt.startsWith("0001-") ? "Not attempted" : attempt.status === "started" ? "In progress" : "Not solved"}</td><td>{puzzle.incorrect}</td><td>{puzzle.hints}</td><td>{puzzle.seconds}</td></tr>)}</tbody></table></div></>}</section>)}</details>;
}

function CoachReview({ assignment, act }: { assignment: Assignment; act: LearningAction }) {
  const [feedback, setFeedback] = useState("");
  const [busy, setBusy] = useState(false);
  const { busy: workspaceBusy } = useContext(WorkspaceFeedback);
  return <form className={s.coachReview} onSubmit={async event => {
    event.preventDefault(); if (busy || workspaceBusy) return;
    const submitter = (event.nativeEvent as SubmitEvent).submitter as HTMLButtonElement | null;
    setBusy(true);
    try { await act(submitter?.value === "return" ? "returnAssignment" : "reviewAssignment", { id: assignment.id, version: assignment.version, feedback: feedback.trim() }, "Feedback shared with the learner."); }
    finally { setBusy(false); }
  }}><label>Coach feedback<textarea value={feedback} onChange={event => setFeedback(event.target.value)} required minLength={5} maxLength={5000} rows={4} placeholder="Recognise what went well and give one useful next step…" disabled={busy || workspaceBusy} /></label><div className={s.inlineActions}><button className={s.primaryButton} value="review" disabled={busy || workspaceBusy || feedback.trim().length < 5}>Share feedback <Icon name="check" size={15} /></button><button className={s.secondaryButton} value="return" disabled={busy || workspaceBusy || feedback.trim().length < 5}>Return for another attempt</button></div></form>;
}

export default function HomeworkPanel({ data, learning, user, kind, act, onUploaded, onRefresh }: { data: AcademyWorkspaceData; learning: LearningWorkspace; user: SessionUser; kind: "student" | "guardian" | "coach" | "admin"; act: LearningAction; onUploaded?: (workspace: LearningWorkspace) => void; onRefresh: () => void }) {
  const studentView = kind === "student";
  const staffView = kind === "coach" || kind === "admin";
  const managed = staffView && learning.students.some(student => student.canManage);
  const ownIds = new Set(data.students.filter(student => student.userId === user.id).map(student => student.id));
  const students = learning.students.filter(student => !studentView || ownIds.has(student.id));
  const studentIds = new Set(students.map(student => student.id));
  const [filter, setFilter] = useState<HomeworkFilter>(staffView ? "all" : "todo");
  const [query, setQuery] = useState("");
  const [activity, setActivity] = useState("");
  const [studentId, setStudent] = useState("");
  const [batchId, setBatch] = useState("");
  const [sort, setSort] = useState("due");
  const [selectedId, setSelected] = useState("");
  const [limit, setLimit] = useState(12);
  const [mobileDetail, setMobileDetail] = useState(false);
  const [editor, setEditor] = useState<Assignment | "new" | null>(null);
  const [practiceId, setPractice] = useState<string | null>(null);
  const [closing, setClosing] = useState<Assignment | null>(null);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [now] = useState(() => Date.now());
  const detailRef = useRef<HTMLElement>(null);
  const listRef = useRef<HTMLElement>(null);
  const { busy } = useContext(WorkspaceFeedback);
  const zone = data.profile.timeZone;
  const scoped = learning.assignments.filter(item => studentIds.has(item.studentId) && (managed || item.status !== "draft") && (!studentId || item.studentId === studentId) && (!batchId || data.batches.find(batch => batch.id === batchId)?.studentIds.includes(item.studentId)));
  const visible = scoped.filter(item => matchesFilter(item, filter) && (!activity || item.kind === activity) && `${item.title} ${item.description} ${studentView ? "" : students.find(student => student.id === item.studentId)?.name || ""}`.toLowerCase().includes(query.trim().toLowerCase())).sort((a, b) => sort === "title" ? a.title.localeCompare(b.title) : sort === "learner" ? (students.find(student => student.id === a.studentId)?.name || "").localeCompare(students.find(student => student.id === b.studentId)?.name || "") : (a.dueAt || "9999").localeCompare(b.dueAt || "9999"));
  const selected = visible.find(item => item.id === selectedId) || (studentView ? visible.find(item => assignmentStatus(item) === "started") : undefined) || visible[0];
  const selectedStudent = students.find(student => student.id === selected?.studentId);
  const attempt = selected?.attempts.at(-1);
  const feedback = selected ? [...selected.attempts].reverse().find(item => item.feedback)?.feedback : undefined;
  const current = selected ? currentAttempt(selected) : undefined;
  const lesson = data.classes.find(lesson => lesson.id === selected?.classId);
  const coach = data.coaches.find(coach => coach.id === lesson?.coachId);
  const practice = learning.assignments.find(item => item.id === practiceId && studentIds.has(item.studentId) && canPractice(item, students.find(student => student.id === item.studentId)));
  const resume = studentView ? scoped.find(item => assignmentStatus(item) === "started") : undefined;
  const tabs: { id: HomeworkFilter; label: string }[] = [{ id: "todo", label: "To do" }, { id: "submitted", label: managed ? "To review" : "In review" }, { id: "reviewed", label: "Completed" }, ...(managed ? [{ id: "draft" as const, label: "Drafts" }] : []), { id: "all", label: "All homework" }];
  const overdue = (item: Assignment) => isTodo(item) && !!item.dueAt && Date.parse(item.dueAt) < now;
  function focusDetails() { window.requestAnimationFrame(() => { detailRef.current?.focus({ preventScroll: true }); if (window.matchMedia("(max-width: 760px)").matches) detailRef.current?.scrollIntoView({ block: "start", behavior: "instant" }); }); }
  function select(item: Assignment) { setSelected(item.id); setMobileDetail(true); if (window.matchMedia("(max-width: 760px)").matches) focusDetails(); }
  function changeFilter(value: HomeworkFilter) { setFilter(value); setLimit(12); setMobileDetail(false); }
  function resetFilters() { setQuery(""); setActivity(""); setStudent(""); setBatch(""); setLimit(12); }
  function showSubmission(id: string) { setPractice(null); setSelected(id); setFilter("submitted"); resetFilters(); setMobileDetail(true); focusDetails(); }

  return <section className={s.homework} aria-label="Homework workspace">
    <header className={s.pageHeading}><div><h1>{studentView ? "My homework" : kind === "guardian" ? "Homework & practice" : "Homework"}<span>.</span></h1><p>{studentView ? "Your next move starts with a little practice." : managed ? "Purposeful practice. Thoughtful feedback. All in one place." : "Follow practice, celebrate progress, and see the coach’s feedback."}</p></div><div className={s.headingActions}><button className={s.iconButton} disabled={busy} aria-label="Refresh homework" onClick={onRefresh}><Icon name="refresh" size={17} /></button>{managed && <button className={s.primaryButton} onClick={() => setEditor("new")}><Icon name="plus" size={17} />Create homework</button>}</div></header>
    <div className={s.overview}><div className={s.summary}><span><b>{scoped.filter(isTodo).length}</b> to do</span><i /><span><b>{scoped.filter(item => assignmentStatus(item) === "submitted").length}</b> {managed ? "to review" : "with the coach"}</span><i /><span><Icon name="checkCircle" size={15} /><b>{scoped.filter(item => assignmentStatus(item) === "reviewed").length}</b> completed</span></div><button className={s.textButton} onClick={() => window.print()}><Icon name="list" size={14} />Print this view</button></div>
    {resume && <section className={s.continueCard} aria-label="Suggested practice"><span className={s.continueIcon}><Icon name={activityFor(resume.kind).icon} size={25} /></span><div><span className={s.eyebrow}>PICK UP WHERE YOU LEFT OFF</span><h2>{resume.title}</h2><p>{resume.kind === "puzzle_set" ? `${currentAttempt(resume)?.metrics.completed || 0} of ${resume.puzzles.length} puzzles solved` : activityFor(resume.kind).label}</p></div><button className={s.primaryButton} onClick={() => setPractice(resume.id)}>Continue practice <Icon name="arrow" size={17} /></button></section>}
    <div className={s.tabs} aria-label="Filter homework by status">{tabs.map(tab => <button key={tab.id} className={filter === tab.id ? s.activeTab : ""} aria-pressed={filter === tab.id} onClick={() => changeFilter(tab.id)}>{tab.label}<span>{scoped.filter(item => matchesFilter(item, tab.id)).length}</span></button>)}</div>
    <div className={s.filters}><label className={s.search}><Icon name="search" size={17} /><input type="search" aria-label="Search homework" placeholder="Search homework…" value={query} onChange={event => { setQuery(event.target.value); setLimit(12); setMobileDetail(false); }} /></label><label className={s.selectFilter}><span className="sr-only">Activity type</span><select value={activity} onChange={event => { setActivity(event.target.value); setLimit(12); setMobileDetail(false); }}><option value="">All activities</option>{activities.map(item => <option key={item.kind} value={item.kind}>{item.label}</option>)}</select></label>
      {managed && <label className={s.selectFilter}><span className="sr-only">Batch</span><select value={batchId} onChange={event => { setBatch(event.target.value); setStudent(""); setLimit(12); setMobileDetail(false); }}><option value="">All batches</option>{data.batches.map(batch => <option key={batch.id} value={batch.id}>{batch.name}</option>)}</select></label>}
      {!studentView && students.length > 1 && <label className={s.selectFilter}><span className="sr-only">Learner</span><select value={studentId} onChange={event => { setStudent(event.target.value); setLimit(12); setMobileDetail(false); }}><option value="">All learners</option>{students.filter(student => !batchId || data.batches.find(batch => batch.id === batchId)?.studentIds.includes(student.id)).map(student => <option key={student.id} value={student.id}>{student.name}</option>)}</select></label>}
      <label className={s.sort}><Icon name="list" size={14} /><span className="sr-only">Sort homework</span><select value={sort} onChange={event => setSort(event.target.value)}><option value="due">Due date</option><option value="title">Title A–Z</option>{!studentView && students.length > 1 && <option value="learner">Learner</option>}</select></label>
    </div>
    <div className={`${s.workspace} ${mobileDetail ? s.showDetail : ""}`}><section ref={listRef} tabIndex={-1} className={s.assignmentList} aria-label="Homework assignments"><header className={s.listHeader}><h2>{filter === "todo" ? "On the practice list" : filter === "submitted" ? "Ready for coach feedback" : filter === "reviewed" ? "Practice pays off" : filter === "draft" ? "A little preparation" : "All assignments"}</h2><span>{visible.length} {visible.length === 1 ? "assignment" : "assignments"}</span></header>
      {visible.slice(0, limit).map(item => { const status = assignmentStatus(item); const type = activityFor(item.kind); return <button key={item.id} className={`${s.assignment} ${selected?.id === item.id ? s.selectedAssignment : ""}`} aria-label={`View ${item.title}${studentView ? "" : ` · ${students.find(student => student.id === item.studentId)?.name || "Learner"}`}`} aria-pressed={selected?.id === item.id} onClick={() => select(item)}><span className={`${s.activityIcon} ${s[item.kind]}`}><Icon name={type.icon} size={20} /></span><span className={s.assignmentContent}><span className={s.assignmentType}>{type.label}{!studentView && <><i />{students.find(student => student.id === item.studentId)?.name}</>}</span><strong>{item.title}</strong><span className={s.assignmentMeta}><Status item={item} /><span className={overdue(item) ? s.overdue : ""}>{overdue(item) ? "Overdue · " : item.dueAt ? "Due " : ""}{dueLabel(item.dueAt, zone)}</span></span>{item.kind === "puzzle_set" && status === "started" && <span className={s.rowProgress}><span><i style={{ width: `${(currentAttempt(item)?.metrics.completed || 0) / Math.max(item.puzzles.length, 1) * 100}%` }} /></span>{currentAttempt(item)?.metrics.completed || 0}/{item.puzzles.length} solved</span>}{status === "returned" && <span className={s.feedbackHint}><Icon name="message" size={12} />A note from the coach</span>}</span><Icon name="right" size={15} /></button>; })}
      {!visible.length && <div className={s.empty}><Icon name={query || activity || studentId || batchId ? "search" : "homework"} size={32} /><h3>{query || activity || studentId || batchId ? "No matching homework" : filter === "todo" ? "All caught up here." : "Nothing here just yet."}</h3><p>{managed ? "Create a new assignment or try another view." : "New homework appears when your coach shares it."}</p>{(query || activity || studentId || batchId) && <button className={s.secondaryButton} onClick={resetFilters}>Clear filters</button>}</div>}
      <footer className={s.listFooter}>{visible.length > limit ? <button className={s.textButton} onClick={() => setLimit(value => value + 12)}>Show more assignments</button> : <><Icon name="info" size={14} /><span>A few focused minutes make a difference.</span></>}</footer>
    </section>
    {selected ? <article ref={detailRef} tabIndex={-1} className={s.detail} aria-label={`Details: ${selected.title}`}><button className={s.backButton} onClick={() => { setMobileDetail(false); window.requestAnimationFrame(() => listRef.current?.focus()); }}><Icon name="left" size={16} />Back to homework</button><header className={s.detailHeading}><span className={s.eyebrow}>{activityFor(selected.kind).label}</span><Status item={selected} /></header><h2>{selected.title}</h2><div className={s.coach}><Icon name={studentView ? "book" : "users"} size={15} /><span>{studentView ? coach?.name || "Practice from your coach" : selectedStudent?.name}</span>{lesson && <><i /><span>{lesson.title}</span></>}</div><div className={s.detailFacts}><span className={overdue(selected) ? s.overdue : ""}><Icon name="calendar" size={15} />{overdue(selected) ? "Overdue · " : selected.dueAt ? "Due " : ""}{dueLabel(selected.dueAt, zone)}</span>{selected.kind === "puzzle_set" && <span><Icon name="board" size={15} />{selected.puzzles.length} positions</span>}</div>
      {feedback && <div className={`${s.feedback} ${assignmentStatus(selected) === "reviewed" ? s.reviewFeedback : ""}`}><Icon name="message" size={17} /><div><strong>Coach feedback</strong><p>{feedback}</p></div></div>}
      <section className={s.detailDescription}><h3>{assignmentStatus(selected) === "reviewed" ? "What you worked on" : "Your challenge"}</h3><p className={s.prose}>{selected.description}</p></section>
      {selected.kind === "puzzle_set" && !!selected.puzzles.length && <div className={s.puzzlePreview}><div className={s.miniBoard} aria-hidden="true" inert><Chessboard options={{ id: `homework-preview-${selected.id}`, position: selected.puzzles[0].fen, allowDragging: false, showNotation: false, darkSquareStyle: { backgroundColor: "#a98b64" }, lightSquareStyle: { backgroundColor: "#f2e6d3" } }} /></div><div><span className={s.eyebrow}>PRACTICE PROGRESS</span><strong>{current?.metrics.completed || 0}<span> / {selected.puzzles.length} solved</span></strong><div className={s.progressTrack}><span style={{ width: `${(current?.metrics.completed || 0) / selected.puzzles.length * 100}%` }} /></div><p>{assignmentStatus(selected) === "returned" ? "Ready for a fresh attempt." : current?.metrics.reveals ? `${current.metrics.reveals} solutions revealed` : current?.metrics.completed === selected.puzzles.length ? "All positions solved." : "One thoughtful move at a time."}</p></div></div>}
      <AssignedMaterials assignment={selected} />
      {attempt?.answer && <section className={s.submission}><h3>{studentView ? "Your reflection" : "Learner response"}</h3><blockquote>{attempt.answer}</blockquote></section>}
      {!!attempt?.files?.length && <div className={s.fileList}>{attempt.files.map(file => <FileDownload key={file.id} id={file.id} filename={file.filename} />)}</div>}
      {assignmentStatus(selected) === "submitted" && !managed && <div className={s.reviewNotice}><Icon name="clock" size={20} /><div><strong>Over to the coach.</strong><p>Practice is submitted. Feedback will appear here once it’s reviewed.</p></div></div>}
      {current && selected.kind === "puzzle_set" && <details className={s.disclosure}><summary>Practice summary</summary><Metrics metrics={current.metrics} /></details>}
      <AttemptHistory assignment={selected} />
      {managed && selectedStudent?.canManage && assignmentStatus(selected) === "submitted" && <CoachReview key={`${selected.id}-${attempt?.number}`} assignment={selected} act={act} />}
      <footer className={s.detailFooter}>{studentView && canPractice(selected, selectedStudent) ? <><button className={s.primaryButton} onClick={() => setPractice(selected.id)}>{assignmentStatus(selected) === "returned" ? "Improve my answer" : assignmentStatus(selected) === "started" ? "Continue practice" : "Start homework"}<Icon name="arrow" size={17} /></button><small>Practise, reflect, and send your work for feedback.</small></> : managed && selectedStudent?.canManage ? <div className={s.inlineActions}>{selected.status === "draft" ? <><button className={s.secondaryButton} disabled={busy} onClick={() => setEditor(selected)}>Edit draft <Icon name="edit" size={15} /></button><button className={s.primaryButton} disabled={busy} onClick={async () => { if (await act("assignAssignment", { id: selected.id, version: selected.version }, "Homework assigned.")) { setFilter("todo"); setSelected(selected.id); } }}>Assign homework <Icon name="arrow" size={16} /></button></> : selected.status === "assigned" && <button className={s.textButton} disabled={busy} onClick={() => setClosing(selected)}>Close assignment</button>}</div> : assignmentStatus(selected) === "reviewed" ? <span className={s.finishedNote}><Icon name="checkCircle" size={16} />Completed and reviewed. Well done!</span> : selected.status === "closed" ? <p>This assignment is closed. Past attempts remain available above.</p> : kind === "guardian" && <small>Learners submit their practice from their own student account.</small>}</footer>
    </article> : <div className={s.detailEmpty}><Icon name="homework" size={32} /><p>A little room for the next challenge.</p></div>}
    </div>
    {editor && <AssignmentEditor data={data} learning={learning} item={editor === "new" ? undefined : editor} act={act} close={() => setEditor(null)} saved={status => { setEditor(null); setFilter(status === "draft" ? "draft" : "todo"); resetFilters(); setSelected(""); setMobileDetail(false); }} />}
    {practice && <PracticeDialog key={practice.id} assignment={practice} act={act} close={() => setPractice(null)} submitted={() => showSubmission(practice.id)} onUploaded={onUploaded} draft={drafts[practice.id]} keepDraft={answer => setDrafts(previous => ({ ...previous, [practice.id]: answer }))} />}
    {closing && <HomeworkDialog title="Close this assignment?" subtitle="HOMEWORK SETTINGS" close={() => setClosing(null)} busy={busy}><div className={s.practiceContent}><h3>{closing.title}</h3><p>The learner will no longer be able to submit or continue this assignment. Existing attempts, files, and feedback remain available.</p><footer className={s.dialogFooter}><button className={s.secondaryButton} disabled={busy} onClick={() => setClosing(null)}>Keep open</button><button className={s.primaryButton} disabled={busy} onClick={async () => { if (await act("closeAssignment", { id: closing.id, version: closing.version }, "Homework closed.")) { setClosing(null); setFilter("all"); } }}>Close assignment</button></footer></div></HomeworkDialog>}
  </section>;
}
