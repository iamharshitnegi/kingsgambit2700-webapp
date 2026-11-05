import { Chess } from "chess.js";
import { importStudy } from "../chess/study";
import { academyLevels } from "../academy-types";
import type { Assessment, AssessmentContent, Assignment, LearningAttempt, LearningFile, LearningMetrics, LearningResource, LearningWorkspace, Puzzle, PuzzleProgress } from "../academy-types";
import { createUUIDv7 } from "../realtime/classroom-protocol";
import { DEMO_STUDENT_ID, demoAccounts, type DemoWorkspace } from "./data";
import { sampleHomework } from "./homework-samples";
import { DemoError, getDemoUser, requireRole, writeDemo } from "./store";
import { demoAcademyWorkspace } from "./academy";
import { getDemoFile, putDemoFile } from "./learning-file-store";

type Notice = LearningWorkspace["notifications"][number] & { recipientId: string; studentId: string };
type StoredFile = LearningFile & { ownerId: string; assignmentId?: string; certificateId?: string };
export type DemoLearningState = {
  homeworkSamplesVersion?: number;
  assignments: Assignment[];
  resources: LearningResource[];
  resourceVersions?: Record<string, LearningResource>;
  games: (LearningWorkspace["games"][number] & { ownerId: string })[];
  threads: LearningWorkspace["threads"];
  notifications: Notice[];
  rubrics: LearningWorkspace["rubrics"];
  assessments: Assessment[];
  certificates: LearningWorkspace["certificates"];
  placements: LearningWorkspace["placements"];
  threadReads: Record<string, Record<string, number>>;
  files: StoredFile[];
};

const now = () => new Date().toISOString();
function text(input: Record<string, unknown>, key: string, max = 10000) {
  const value = typeof input[key] === "string" ? input[key].trim() : "";
  if (!value || value.length > max) throw new DemoError(`Enter ${key} (up to ${max} characters).`, 422);
  return value;
}
function find<T extends { id: string }>(items: T[], id: unknown): T {
  const item = items.find((value) => value.id === id);
  if (!item) throw new DemoError("That learning record is unavailable.", 404);
  return item;
}
function conflict(): never { throw new DemoError("The record changed or this action is unavailable. Refresh and try again.", 409); }
function denied(): never { throw new DemoError("This action is unavailable for this account or learner.", 403); }
function version(item: { version: number }, input: Record<string, unknown>) { if (item.version !== input.version) conflict(); }
function requiredRole(...roles: string[]) { const user = getDemoUser(); if (!user) throw new DemoError("Sign in to continue.", 401); if (!user.roles.some((role) => roles.includes(role))) denied(); return user; }
function publisher() { return requiredRole("academic_manager", "super_admin"); }
function staff() { return requiredRole("coach", "academic_manager", "super_admin"); }
function metrics(progress: PuzzleProgress[], total: number, finalized = false): LearningMetrics {
  const attempted = progress.filter((item) => !!item.lastActionAt && !item.lastActionAt.startsWith("0001-"));
  const solved = attempted.filter((item) => item.solved && !item.revealed);
  const notSolved = attempted.filter((item) => (!item.solved || item.revealed) && (finalized || item.revealed)).length;
  const inProgress = attempted.length - solved.length - notSolved;
  const seconds = attempted.reduce((sum, item) => sum + item.seconds, 0);
  const assessed = solved.length + notSolved;
  return { completed: solved.length, notSolved, inProgress, notAttempted: total - attempted.length, firstTry: solved.filter((item) => !item.incorrect && !item.hints).length, incorrect: attempted.reduce((sum, item) => sum + item.incorrect, 0), hints: attempted.reduce((sum, item) => sum + item.hints, 0), reveals: attempted.filter((item) => item.revealed).length, seconds, accuracy: assessed ? 100 * solved.length / assessed : null, averageSeconds: attempted.length ? seconds / attempted.length : null };
}
function emptyProgress(): PuzzleProgress { return { moves: [], incorrect: 0, hints: 0, seconds: 0, solved: false, revealed: false, lastActionAt: "" }; }
function newAttempt(assignment: Assignment): LearningAttempt {
  const progress = assignment.puzzles.map(emptyProgress);
  return { id: createUUIDv7(), number: (assignment.attempts.at(-1)?.number ?? 0) + 1, status: "started", answer: "", feedback: "", puzzles: progress, metrics: metrics(progress, assignment.puzzles.length), files: [] };
}

function initialized(state: DemoWorkspace): DemoLearningState {
  if (state.learning) {
    state.learning.placements ??= [];
    if (addHomeworkSamples(state)) writeDemo(state);
    return state.learning;
  }
  const dimensions = ["Piece safety", "Tactical patterns", "Planning"];
  const rubricId = "01990000-0000-7000-8000-000000000013";
  state.learning = {
    assignments: state.assignments.map((item) => ({ id: item.id, studentId: item.studentId, title: item.title, description: item.description, kind: "study", status: "assigned", version: 1, dueAt: item.dueAt, puzzles: [], resources: [], attempts: item.status === "assigned" ? [] : [{ id: createUUIDv7(), number: 1, status: item.status, answer: item.answer || "", feedback: item.feedback || "", puzzles: [], files: [], metrics: metrics([], 0) }] })),
    resources: [{ id: createUUIDv7(), ownerId: "demo-coach", title: "Ask before every move", kind: "study", visibility: "shared", status: "published", levelSlug: "foundation-1", tags: ["thinking routine"], content: "Look for checks, captures and threats. Before you move, explain what your opponent is threatening.", version: 1 }],
    games: [], threads: [], notifications: [], certificates: [], placements: [], threadReads: {}, files: [],
    rubrics: [{ id: rubricId, levelSlug: "foundation-1", title: "Board foundations", version: 1, dimensions }],
    assessments: state.reports.map((report) => ({ id: report.id, reportId: report.id, studentId: report.studentId, rubricId, title: report.title, period: now().slice(0, 7), status: report.status, version: 1, publishedAt: report.status === "published" ? now() : null, acknowledgedAt: report.acknowledgedAt || null, history: [], content: { learned: report.summary, strengths: "Growing confidence in piece safety and tactical patterns.", focusNext: "Check the opponent’s threats before each move.", practicePlan: "Practise a few positions each day and explain your choice.", coachComment: "Keep building a thoughtful routine.", eligible: false, ratings: dimensions.map((dimension) => ({ dimension, rating: "developing", evidence: "Observed in recent practice and lessons." })) } })),
  };
  addHomeworkSamples(state);
  writeDemo(state);
  return state.learning;
}

function addHomeworkSamples(state: DemoWorkspace) {
  const learning = state.learning;
  if (!learning || learning.homeworkSamplesVersion === 1) return false;
  if (state.students.some(student => student.id === DEMO_STUDENT_ID)) {
    const ids = new Set(learning.assignments.map(item => item.id));
    learning.assignments.push(...sampleHomework(DEMO_STUDENT_ID).filter(item => !ids.has(item.id)));
  }
  learning.homeworkSamplesVersion = 1;
  return true;
}

function context(state: DemoWorkspace) {
  const user = requireRole(...demoAccounts.map((item) => item.role));
  const academy = demoAcademyWorkspace(state);
  const manage = user.roles.some((role) => ["coach", "academic_manager", "super_admin"].includes(role));
  const publish = user.roles.some((role) => ["academic_manager", "super_admin"].includes(role));
  const read = !user.roles.every((role) => ["operations_admin", "finance_admin"].includes(role));
  const communicate = !user.roles.every((role) => role === "finance_admin");
  return { user, academy, manage, publish, read, communicate };
}
function studentAccess(state: DemoWorkspace, studentId: string, kind: "view" | "manage" | "submit" | "communicate") {
  const ctx = context(state);
  const student = ctx.academy.students.find((item) => item.id === studentId && item.status === "active");
  if (!student || (kind === "view" && !ctx.read) || (kind === "manage" && !ctx.manage) || (kind === "submit" && (!ctx.user.roles.includes("student") || student.userId !== ctx.user.id)) || (kind === "communicate" && !ctx.communicate)) denied();
  return ctx;
}
function currentBatchStudentIds(state: DemoWorkspace, batchId: string) {
  const at = Date.now();
  return [...new Set((state.academy?.enrolments ?? []).filter((item) =>
    item.batchId === batchId && item.status === "active" && Date.parse(item.startsAt) <= at &&
    (!item.endsAt || Date.parse(item.endsAt) > at),
  ).map((item) => item.studentId))];
}
function threadAccess(state: DemoWorkspace, thread: { studentId?: string; batchId?: string }, send: boolean) {
  if (thread.studentId) return studentAccess(state, thread.studentId, "communicate");
  const ctx = context(state);
  const batch = ctx.academy.batches.find((item) => item.id === thread.batchId);
  if (!batch || !ctx.communicate) denied();
  const staff = ctx.user.roles.some((role) => ["academic_manager", "operations_admin", "super_admin"].includes(role));
  const assignedCoach = ctx.user.roles.includes("coach") && batch.coachId === ctx.user.id;
  const members = currentBatchStudentIds(state, batch.id);
  if (send ? !staff && !assignedCoach : !staff && !members.some((id) => ctx.academy.students.some((student) => student.id === id))) denied();
  return ctx;
}
function notify(state: DemoWorkspace, studentId: string, title: string, href = "/portal/learning", includeCoach = false) {
  const learning = initialized(state);
  const ctx = context(state);
  const student = ctx.academy.students.find((item) => item.id === studentId);
  const guardian = demoAccounts.find((account) => account.role === "guardian" && account.email === state.academy?.guardianEmails[studentId]);
  const coachIDs = includeCoach ? ctx.academy.batches.filter((batch) => currentBatchStudentIds(state, batch.id).includes(studentId) && state.academy?.staff?.find((person) => person.id === batch.coachId)?.status !== "suspended").map((batch) => batch.coachId) : [];
  const recipients = [student?.userId, ...(guardian ? ["demo-guardian"] : []), ...coachIDs].filter((item): item is string => !!item && item !== ctx.user.id);
  for (const recipientId of new Set(recipients)) learning.notifications.unshift({ id: createUUIDv7(), recipientId, studentId, title, href, createdAt: now(), readAt: null });
}

export function demoLearningWorkspace(state: DemoWorkspace): LearningWorkspace {
  const ctx = context(state);
  const learning = initialized(state);
  const visibleStudents = ctx.read ? ctx.academy.students.filter((item) => item.status === "active") : [];
  const ids = new Set(visibleStudents.map((item) => item.id));
  const assignments = structuredClone(learning.assignments.filter((item) => ids.has(item.studentId) && (ctx.manage || item.status !== "draft")));
  for (const assignment of assignments) for (const attempt of assignment.attempts) attempt.metrics = metrics(attempt.puzzles, assignment.puzzles.length, attempt.status !== "started");
  if (!ctx.manage) for (const assignment of assignments) for (const puzzle of assignment.puzzles) delete puzzle.solutions;
  const threads = structuredClone(learning.threads.filter((thread) => {
    try { threadAccess(state, thread, false); return true; } catch { return false; }
  }));
  for (const thread of threads) {
    try { threadAccess(state, thread, true); thread.canSend = true; } catch { thread.canSend = false; }
    const read = learning.threadReads[thread.id]?.[ctx.user.id] ?? 0;
    thread.unread = thread.messages.slice(read).filter((item) => item.actorId !== ctx.user.id).length;
  }
  return {
    students: visibleStudents.map((item) => ({ id: item.id, name: item.name, levelSlug: learning.placements.find((placement) => placement.studentId === item.id)?.levelSlug || ctx.academy.batches.find((batch) => batch.studentIds.includes(item.id))?.level || "", canManage: ctx.manage, canSubmit: ctx.user.roles.includes("student") && item.userId === ctx.user.id, canCommunicate: ctx.communicate })),
    assignments,
    resources: ctx.manage ? structuredClone(learning.resources.filter((item) => item.ownerId === ctx.user.id || (item.visibility === "shared" && (item.status === "published" || ctx.publish)))) : [],
    games: learning.games.filter((item) => ids.has(item.studentId) && (item.shared || item.ownerId === ctx.user.id || ctx.manage)).map((item) => ({ id: item.id, studentId: item.studentId, title: item.title, source: item.source, pgn: item.pgn, playedAt: item.playedAt, shared: item.shared, version: item.version, review: item.shared || ctx.manage ? item.review : "" })),
    threads,
    notifications: learning.notifications.filter((item) => item.recipientId === ctx.user.id && ctx.academy.students.some((student) => student.id === item.studentId)).map((item) => ({ id: item.id, title: item.title, href: item.href, createdAt: item.createdAt, readAt: item.readAt })),
    rubrics: structuredClone(learning.rubrics),
    assessments: structuredClone(learning.assessments.filter((item) => ids.has(item.studentId) && (ctx.manage || item.status === "published"))).map((item) => ({ ...item, history: ctx.manage ? item.history : [] })),
    certificates: structuredClone(learning.certificates.filter((item) => ids.has(item.studentId))),
    placements: structuredClone(learning.placements.filter((item) => ids.has(item.studentId))), canPlace: ctx.publish,
    pagination: { limit: 500, hasMore: false, nextLimit: 1000 },
    canPublish: ctx.publish, canCreateResources: ctx.manage,
  };
}

function validatePGN(pgn: string) { if (!pgn.trim() || pgn.length > 512 * 1024) throw new DemoError("Enter a PGN smaller than 512 KB.", 422); try { for (const game of pgn.split(/\n\s*\n(?=\s*\[Event\s)/)) importStudy(game); } catch { throw new DemoError("The PGN contains an invalid game or move.", 422); } }
function validatePuzzles(input: unknown): Puzzle[] {
  if (!Array.isArray(input) || input.length > 100) throw new DemoError("Use up to 100 puzzles.", 422);
  const puzzles = structuredClone(input) as Puzzle[];
  for (const puzzle of puzzles) {
    if (!puzzle.title?.trim() || !Array.isArray(puzzle.solutions) || !puzzle.solutions.length || puzzle.solutions.length > 20) throw new DemoError("Each puzzle needs a title and solution lines.", 422);
    for (const line of puzzle.solutions) {
      if (!Array.isArray(line) || !line.length || line.length > 64) throw new DemoError("Use 1–64 moves per solution.", 422);
      try { const game = new Chess(puzzle.fen); for (const move of line) game.move({ from: move.slice(0, 2), to: move.slice(2, 4), promotion: move.slice(4) || undefined }); }
      catch { throw new DemoError("A puzzle solution has an illegal move.", 422); }
    }
  }
  return puzzles;
}
function assignmentFields(input: Record<string, unknown>) {
  const kind = String(input.kind || "");
  if (!["puzzle_set", "game_review", "study", "file_upload", "other"].includes(kind)) throw new DemoError("Choose a homework activity.", 422);
  const puzzles = validatePuzzles(input.puzzles || []);
  if (kind === "puzzle_set" && !puzzles.length) throw new DemoError("Add at least one puzzle.", 422);
  if (input.dueAt && !Number.isFinite(Date.parse(String(input.dueAt)))) throw new DemoError("Choose a valid due date.", 422);
  return { title: text(input, "title", 200), description: text(input, "description"), kind, puzzles, dueAt: input.dueAt ? String(input.dueAt) : null };
}
function resourcesForAssignment(state: DemoWorkspace, input: Record<string, unknown>) {
  const ids = Array.isArray(input.resourceIds) ? input.resourceIds : [];
  if (ids.length > 10) throw new DemoError("Attach up to 10 resources.", 422);
  const available = demoLearningWorkspace(state).resources;
  return [...new Set(ids)].map((id) => { const resource = find(available, id); if (resource.status !== "published") conflict(); const { id: resourceID, version, title, kind, content, fileId, filename } = resource; return { id: resourceID, version, title, kind, content, fileId, filename }; });
}
function editableAttempt(assignment: Assignment) {
  let attempt = assignment.attempts.at(-1);
  if (!attempt || attempt.status === "returned") { attempt = newAttempt(assignment); assignment.attempts.push(attempt); }
  if (attempt.status !== "started") conflict();
  return attempt;
}
function puzzleAction(assignment: Assignment, attempt: LearningAttempt, input: Record<string, unknown>) {
  const index = Number(input.puzzleIndex), elapsed = Number(input.elapsedSeconds ?? 0);
  if (!Number.isInteger(index) || index < 0 || index >= assignment.puzzles.length || !Number.isInteger(elapsed) || elapsed < 0 || elapsed > 60) throw new DemoError("Choose an available puzzle and valid practice duration.", 422);
  const puzzle = assignment.puzzles[index];
  const progress = attempt.puzzles[index];
  if (progress.solved || progress.revealed) conflict();
  progress.seconds += progress.lastActionAt && !progress.lastActionAt.startsWith("0001-") ? Math.min(elapsed, Math.max(0, Math.floor((Date.now() - Date.parse(progress.lastActionAt)) / 1000))) : 0;
  progress.lastActionAt = now();
  const candidates = (puzzle.solutions ?? []).filter((line) => line.length > progress.moves.length && progress.moves.every((move, i) => line[i] === move));
  if (!candidates.length) conflict();
  if (input.action === "puzzleHint") { progress.hints++; progress.hint = `Look at the piece on ${candidates[0][progress.moves.length].slice(0, 2)}.`; }
  else if (input.action === "puzzleReveal") { progress.revealed = true; progress.hint = `Solution: ${candidates[0].join(" ")}`; }
  else {
    const move = String(input.move || "").trim().toLowerCase();
    const line = candidates.find((item) => item[progress.moves.length] === move);
    if (!line) { progress.incorrect++; progress.hint = "Try another move."; }
    else { progress.moves.push(move); progress.hint = ""; if (progress.moves.length < line.length) progress.moves.push(line[progress.moves.length]); progress.solved = progress.moves.length === line.length; }
  }
  attempt.metrics = metrics(attempt.puzzles, assignment.puzzles.length);
}

function assignmentAction(state: DemoWorkspace, input: Record<string, unknown>) {
  const learning = initialized(state), action = String(input.action);
  if (action === "createAssignment") {
    const fields = assignmentFields(input), resources = resourcesForAssignment(state, input);
    const studentIds = Array.isArray(input.studentIds) && input.studentIds.length ? input.studentIds.map(String) : input.studentId ? [String(input.studentId)] : [];
    if (!studentIds.length || !["draft", "assigned"].includes(String(input.status || "draft"))) throw new DemoError("Select learners and draft or assigned status.", 422);
    for (const studentId of new Set(studentIds)) {
      const ctx = studentAccess(state, studentId, "manage");
      if (input.classId && !ctx.academy.classes.some((item) => item.id === input.classId && item.studentIds.includes(studentId))) throw new DemoError("The learner is not assigned to this class.", 422);
      const assignment: Assignment = { id: createUUIDv7(), studentId, ...(input.classId ? { classId: String(input.classId) } : {}), ...fields, resources, status: String(input.status || "draft"), version: 1, attempts: [] };
      learning.assignments.unshift(assignment);
      if (assignment.status === "assigned") notify(state, studentId, `New practice: ${assignment.title}`);
    }
    return;
  }
  const assignment = find(learning.assignments, input.id);
  const studentAction = ["submitAssignment", "puzzleMove", "puzzleHint", "puzzleReveal"].includes(action);
  studentAccess(state, assignment.studentId, studentAction ? "submit" : "manage"); version(assignment, input);
  if (action === "editAssignment") { if (assignment.status !== "draft") conflict(); Object.assign(assignment, assignmentFields(input), { resources: resourcesForAssignment(state, input) }); }
  else if (action === "assignAssignment") { if (assignment.status !== "draft") conflict(); assignment.status = "assigned"; notify(state, assignment.studentId, `New practice: ${assignment.title}`); }
  else if (action === "closeAssignment") { if (assignment.status !== "assigned") conflict(); assignment.status = "closed"; }
  else {
    if (assignment.status !== "assigned") conflict();
    if (studentAction) {
      const attempt = editableAttempt(assignment);
      if (action === "submitAssignment") {
        if (assignment.kind === "file_upload" && !attempt.files?.length) throw new DemoError("Upload your work before submitting.", 422);
        attempt.answer = ["puzzle_set", "file_upload"].includes(assignment.kind) ? String(input.answer || "") : text(input, "answer");
        attempt.status = "submitted";
        attempt.metrics = metrics(attempt.puzzles, assignment.puzzles.length, true);
      } else puzzleAction(assignment, attempt, input);
    } else {
      const attempt = assignment.attempts.at(-1); if (!attempt || attempt.status !== "submitted") conflict();
      attempt.feedback = text(input, "feedback"); attempt.status = action === "returnAssignment" ? "returned" : "reviewed";
      notify(state, assignment.studentId, `Coach feedback: ${assignment.title}`);
    }
  }
  assignment.version++;
}

function resourceAction(state: DemoWorkspace, input: Record<string, unknown>) {
  const user = staff(), learning = initialized(state), action = String(input.action);
  const fields = () => {
    const kind = String(input.kind), content = text(input, "content", 512 * 1024);
    if (kind === "pgn") validatePGN(content);
    else if (kind === "fen") { try { new Chess(content); } catch { throw new DemoError("Enter a valid FEN position.", 422); } }
    else if (kind !== "study" || content.length > 20000) throw new DemoError("Choose PGN, FEN, or study notes up to 20 KB.", 422);
    const tags = Array.isArray(input.tags) ? input.tags.map(String) : [];
    if (tags.length > 20 || tags.some((tag) => !tag.trim() || tag.length > 80)) throw new DemoError("Use up to 20 short tags.", 422);
    return { title: text(input, "title", 200), kind, content, levelSlug: String(input.levelSlug || ""), tags };
  };
  if (action === "createResource") {
    const visibility = String(input.visibility || "personal"), status = String(input.status || "draft");
    if (!["personal", "shared"].includes(visibility) || !["draft", "published"].includes(status)) throw new DemoError("Choose valid resource visibility and status.", 422);
    if (visibility === "shared" && status === "published") publisher();
    learning.resources.unshift({ id: createUUIDv7(), ownerId: user.id, ...fields(), visibility, status, version: 1 }); return;
  }
  const resource = find(learning.resources, input.id);
  if (resource.ownerId !== user.id && !(context(state).publish && resource.visibility === "shared")) denied();
  version(resource, input);
  if (action === "editResource") {
    if (resource.status !== "draft") conflict();
    if (resource.fileId) {
      if (input.kind !== resource.kind || input.content !== resource.content) throw new DemoError("Uploaded file content is immutable; upload a new file to replace it.", 422);
      const tags = Array.isArray(input.tags) ? input.tags.map(String) : [];
      if (tags.length > 20 || tags.some((tag) => !tag.trim() || tag.length > 80) || String(input.levelSlug || "").length > 100) throw new DemoError("Use a level and up to 20 short tags.", 422);
      Object.assign(resource, { title: text(input, "title", 200), levelSlug: String(input.levelSlug || ""), tags });
    } else Object.assign(resource, fields());
  }
  else if (action === "reviseResource") { if (resource.status !== "published") conflict(); (learning.resourceVersions ??= {})[`${resource.id}:${resource.version}`] = structuredClone(resource); resource.status = "draft"; }
  else if (action === "publishResource") { if (resource.status !== "draft") conflict(); if (resource.visibility === "shared") publisher(); resource.status = "published"; }
  else { if (resource.status === "archived") conflict(); resource.status = "archived"; }
  resource.version++;
  if (action === "publishResource") (learning.resourceVersions ??= {})[`${resource.id}:${resource.version}`] = structuredClone(resource);
}

function gameAction(state: DemoWorkspace, input: Record<string, unknown>) {
  const learning = initialized(state);
  if (input.action === "saveGame") {
    const studentId = String(input.studentId || "");
    try { studentAccess(state, studentId, "manage"); } catch { studentAccess(state, studentId, "submit"); }
    validatePGN(String(input.pgn || ""));
    const source = String(input.source || "imported"); if (!["imported", "lesson", "homework"].includes(source)) throw new DemoError("Choose a game source.", 422);
    if (input.classId && !context(state).academy.classes.some((item) => item.id === input.classId && item.studentIds.includes(studentId))) throw new DemoError("The game class does not belong to this learner.", 422);
    learning.games.unshift({ id: createUUIDv7(), ownerId: getDemoUser()!.id, studentId, title: text(input, "title", 200), pgn: String(input.pgn), source, shared: input.shared === true, playedAt: input.playedAt ? new Date(String(input.playedAt)).toISOString() : now(), review: "", version: 1 }); return;
  }
  const game = find(learning.games, input.id); studentAccess(state, game.studentId, "manage"); version(game, input);
  game.review = text(input, "feedback"); if (input.pgn) { validatePGN(String(input.pgn)); game.pgn = String(input.pgn); }
  game.shared = input.shared === true; game.version++;
  if (game.shared) notify(state, game.studentId, "Your coach shared a game review");
}

function threadAction(state: DemoWorkspace, input: Record<string, unknown>) {
  const learning = initialized(state), action = String(input.action);
  let thread;
  if (action === "createThread") {
    const studentId = String(input.studentId || ""), batchId = String(input.batchId || "");
    if (!!studentId === !!batchId) throw new DemoError("Choose one learner or one batch.", 422);
    threadAccess(state, { studentId, batchId }, true);
    thread = { id: createUUIDv7(), ...(studentId ? { studentId } : { batchId }), title: text(input, "title", 200), unread: 0, messages: [], canSend: true } as LearningWorkspace["threads"][number];
    learning.threads.unshift(thread);
  } else { thread = find(learning.threads, input.id); threadAccess(state, thread, action === "sendMessage"); }
  const { user } = context(state);
  if (action !== "readThread") {
    thread.messages.push({ id: createUUIDv7(), actorId: user.id, actorName: user.displayName || "Academy", body: text(input, "body"), createdAt: now() });
    const studentIds = thread.studentId ? [thread.studentId] : currentBatchStudentIds(state, thread.batchId || "");
    for (const id of studentIds) notify(state, id, "New academy message", "/portal/messages", true);
  }
  (learning.threadReads[thread.id] ??= {})[user.id] = thread.messages.length;
}

function assessmentFields(learning: DemoLearningState, input: Record<string, unknown>, fallbackRubric?: string) {
  const rubricId = String(input.rubricId || fallbackRubric || ""), rubric = find(learning.rubrics, rubricId);
  const period = text(input, "period", 7); if (!/^\d{4}-(0[1-9]|1[0-2])$/.test(period)) throw new DemoError("Choose a reporting month.", 422);
  const content = input.assessmentContent as AssessmentContent | undefined;
  if (!content || [content.learned, content.strengths, content.focusNext, content.practicePlan, content.coachComment].some((item) => typeof item !== "string" || !item.trim() || item.length > 5000)) throw new DemoError("Complete all five feedback sections.", 422);
  if (!Array.isArray(content.ratings) || content.ratings.length !== rubric.dimensions.length || new Set(content.ratings.map((item) => item.dimension)).size !== rubric.dimensions.length || content.ratings.some((item) => !rubric.dimensions.includes(item.dimension) || !["not_assessed", "emerging", "developing", "secure", "extending"].includes(item.rating) || !item.evidence?.trim())) throw new DemoError("Assess every rubric dimension and add evidence.", 422);
  return { rubricId, period, title: text(input, "title", 200), content: structuredClone(content) };
}
function assessmentAction(state: DemoWorkspace, input: Record<string, unknown>) {
  const learning = initialized(state), action = String(input.action), user = getDemoUser()!;
  if (action === "revokeCertificate") { publisher(); const certificate = find(learning.certificates, input.id); studentAccess(state, certificate.studentId, "manage"); if (certificate.status !== "issued") conflict(); certificate.reason = text(input, "feedback", 3000); certificate.status = "revoked"; return; }
  if (action === "createAssessment") {
    const studentId = String(input.studentId || ""); studentAccess(state, studentId, "manage");
    const id = createUUIDv7(); learning.assessments.unshift({ id, reportId: id, studentId, ...assessmentFields(learning, input), status: "draft", version: 1, publishedAt: null, acknowledgedAt: null, history: [{ action, actorId: user.id, at: now(), content: {} }] }); return;
  }
  const item = find(learning.assessments, input.id); version(item, input);
  if (action === "acknowledgeAssessment") { requiredRole("guardian"); studentAccess(state, item.studentId, "view"); if (item.status !== "published") conflict(); item.acknowledgedAt ??= now(); return; }
  studentAccess(state, item.studentId, "manage");
  if (["publishAssessment", "returnAssessment", "issueCertificate"].includes(action)) publisher();
  if (action === "correctAssessment") {
    if (item.status !== "published" || learning.assessments.some((other) => other.sourceVersionId === item.id && other.status !== "published")) conflict();
    const fields = assessmentFields(learning, input, item.rubricId); if (fields.period !== item.period) throw new DemoError("Corrections must keep the original report period.", 422);
    learning.assessments.unshift({ ...structuredClone(item), ...fields, id: createUUIDv7(), sourceVersionId: item.id, status: "draft", publishedAt: null, acknowledgedAt: null, version: 1, history: [{ action, actorId: user.id, at: now(), content: { sourceVersionId: item.id } }] }); return;
  }
  if (action === "editAssessment") { if (!["draft", "returned"].includes(item.status)) conflict(); const fields = assessmentFields(learning, input, item.rubricId); if (fields.period !== item.period) throw new DemoError("Keep the original reporting month.", 422); Object.assign(item, fields); item.status = "draft"; }
  else if (action === "submitAssessment") { if (!["draft", "returned"].includes(item.status)) conflict(); item.status = "submitted"; }
  else if (action === "returnAssessment") { if (item.status !== "submitted") conflict(); text(input, "feedback", 3000); item.status = "returned"; }
  else if (action === "publishAssessment") { if (item.status !== "submitted") conflict(); item.status = "published"; item.publishedAt = now(); notify(state, item.studentId, `New learning update: ${item.title}`); }
  else if (action === "issueCertificate") {
    if (item.status !== "published" || !item.content.eligible || learning.certificates.some((certificate) => certificate.assessmentId === item.id)) conflict();
    const rubric = find(learning.rubrics, item.rubricId); learning.certificates.unshift({ id: createUUIDv7(), studentId: item.studentId, assessmentId: item.id, levelSlug: rubric.levelSlug, status: "issued", issuedAt: now(), reason: "" });
    notify(state, item.studentId, `Level completion recorded: ${rubric.title}`); return;
  }
  item.version++; (item.history ??= []).push({ action, actorId: user.id, at: now(), content: input.feedback ? { feedback: String(input.feedback) } : {} });
}

function mirrorLegacy(state: DemoWorkspace) {
  if (!state.learning) return;
  for (const item of state.learning.assignments) {
    if (item.status === "draft") continue;
    const attempt = item.attempts.at(-1);
    const legacy = { id: item.id, studentId: item.studentId, title: item.title, description: item.description, dueAt: item.dueAt || now(), status: (attempt?.status === "reviewed" ? "reviewed" : attempt?.status === "submitted" ? "submitted" : "assigned") as "reviewed" | "submitted" | "assigned", answer: attempt?.answer, feedback: attempt?.feedback };
    const index = state.assignments.findIndex((entry) => entry.id === item.id); if (index < 0) state.assignments.push(legacy); else state.assignments[index] = legacy;
  }
  for (const item of state.learning.assessments) {
    const legacy = { id: item.id, studentId: item.studentId, title: item.title, summary: item.content.learned, skills: item.content.ratings.map((rating) => ({ name: rating.dimension, score: ({ not_assessed: 0, emerging: 25, developing: 50, secure: 75, extending: 100 } as Record<string, number>)[rating.rating] })), status: (item.status === "published" ? "published" : "draft") as "published" | "draft", ...(item.acknowledgedAt ? { acknowledgedAt: item.acknowledgedAt } : {}) };
    const index = state.reports.findIndex((entry) => entry.id === item.id); if (index < 0) state.reports.push(legacy); else state.reports[index] = legacy;
  }
}

export function applyDemoLearningAction(state: DemoWorkspace, input: Record<string, unknown>): LearningWorkspace {
  context(state); const learning = initialized(state), action = String(input.action);
  if (["createAssignment", "editAssignment", "assignAssignment", "closeAssignment", "submitAssignment", "reviewAssignment", "returnAssignment", "puzzleMove", "puzzleHint", "puzzleReveal"].includes(action)) assignmentAction(state, input);
  else if (["createResource", "editResource", "reviseResource", "publishResource", "archiveResource"].includes(action)) resourceAction(state, input);
  else if (["saveGame", "reviewGame"].includes(action)) gameAction(state, input);
  else if (["createThread", "sendMessage", "readThread"].includes(action)) threadAction(state, input);
  else if (["createAssessment", "editAssessment", "submitAssessment", "returnAssessment", "publishAssessment", "acknowledgeAssessment", "correctAssessment", "issueCertificate", "revokeCertificate"].includes(action)) assessmentAction(state, input);
  else if (action === "placeStudent") {
    const user = publisher(), studentId = String(input.studentId || ""), levelSlug = String(input.levelSlug || ""); studentAccess(state, studentId, "manage");
    const report = find(learning.assessments, input.assessmentId);
    if (report.studentId !== studentId || report.status !== "published" || !academyLevels.some((level) => level.slug === levelSlug)) throw new DemoError("Choose a published learner report and a current curriculum level.", 422);
    const reason = text(input, "feedback", 3000); if (reason.length < 5) throw new DemoError("Explain the placement decision.", 422);
    const previousLevelSlug = demoLearningWorkspace(state).students.find((student) => student.id === studentId)?.levelSlug || "";
    if (previousLevelSlug === levelSlug) throw new DemoError("The learner already has this placement.", 422);
    learning.placements.unshift({ id: createUUIDv7(), studentId, curriculumVersionId: "01990000-0000-7000-8000-000000000017", curriculumVersion: "demo-1", levelSlug, previousLevelSlug, assessmentId: report.id, reason, placedBy: user.id, createdAt: now() });
    notify(state, studentId, "Your curriculum placement has been updated", "/portal/progress");
  }
  else if (action === "readNotification" || action === "readAllNotifications") {
    const user = getDemoUser()!;
    const notices = action === "readNotification" ? [find(learning.notifications, input.id)] : learning.notifications.filter((item) => item.recipientId === user.id);
    for (const item of notices) { if (item.recipientId !== user.id) denied(); item.readAt ??= now(); }
  } else throw new DemoError("Unknown learning action.", 422);
  mirrorLegacy(state); writeDemo(state); return demoLearningWorkspace(state);
}

function response(body: unknown, status = 200) { return new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json", "Cache-Control": "no-store" } }); }
export async function demoLearningRequest(path: string, method: string, input: Record<string, unknown>, state: DemoWorkspace, request: Request): Promise<Response | null> {
  context(state);
  if (path === "/learning/workspace" && method === "GET") return response(demoLearningWorkspace(state));
  if (path === "/learning/actions" && method === "POST") return response(applyDemoLearningAction(state, input));
  if (path === "/learning/files" && method === "POST") return uploadFile(state, request);
  const download = path.match(/^\/learning\/files\/([^/]+)\/download$/);
  if (download && method === "GET") {
    const learning = initialized(state), file = find(learning.files, download[1]); const workspace = demoLearningWorkspace(state);
    const allowed = workspace.resources.some((item) => item.fileId === file.id)
      || workspace.assignments.some((item) => item.resources?.some((resource) => resource.fileId === file.id) || item.attempts.some((attempt) => attempt.files?.some((entry) => entry.id === file.id)))
      || workspace.certificates.some((item) => item.fileId === file.id && item.status === "issued");
    if (!allowed) denied();
    const blob = await getDemoFile(file.id); if (!blob) throw new DemoError("This local demo file is no longer stored in this browser. Upload it again.", 404);
    return new Response(blob, { headers: { "Content-Type": file.mediaType, "Content-Disposition": `attachment; filename*=UTF-8''${encodeURIComponent(file.filename)}`, "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
  }
  return null;
}

async function uploadFile(state: DemoWorkspace, request: Request) {
  const user = getDemoUser(); if (!user) throw new DemoError("Sign in to upload files.", 401);
  const form = await request.formData(), file = form.get("file");
  if (!file || typeof file === "string" || typeof file.arrayBuffer !== "function" || !file.size || file.size > 20 * 1024 * 1024) throw new DemoError("Choose a supported file smaller than 20 MB.", 422);
  const assignmentId = String(form.get("assignmentId") || ""), certificateId = String(form.get("certificateId") || "");
  if (assignmentId && certificateId) throw new DemoError("Choose one upload destination.", 422);
  const learning = initialized(state);
  const assignment = assignmentId ? find(learning.assignments, assignmentId) : undefined;
  const certificate = certificateId ? find(learning.certificates, certificateId) : undefined;
  if (assignment) { studentAccess(state, assignment.studentId, "submit"); if (assignment.status !== "assigned") conflict(); }
  else if (certificate) { publisher(); studentAccess(state, certificate.studentId, "manage"); if (certificate.status !== "issued" || certificate.fileId) conflict(); }
  else staff();
  const filename = file.name.replaceAll("\\", "/").split("/").at(-1) || "";
  if (!filename || filename.length > 180 || /[\r\n\x00]/.test(filename)) throw new DemoError("Choose a valid filename.", 422);
  const data = new Uint8Array(await file.arrayBuffer()), extension = filename.split(".").at(-1)?.toLowerCase();
  let mediaType: string, kind: string, content = "";
  if (extension === "pgn" || extension === "fen") {
    if (data.length > 512 * 1024) throw new DemoError("Use PGN/FEN files smaller than 512 KB.", 422);
    try { content = new TextDecoder("utf-8", { fatal: true }).decode(data).trim(); if (extension === "pgn") validatePGN(content); else new Chess(content); } catch { throw new DemoError("The chess file is invalid.", 422); }
    kind = extension; mediaType = extension === "pgn" ? "application/x-chess-pgn" : "text/plain";
  } else if (extension === "pdf") {
    const raw = new TextDecoder("latin1").decode(data);
    if (!raw.startsWith("%PDF-") || !raw.includes("%%EOF") || /\/(JavaScript|JS|Launch|EmbeddedFile|RichMedia)\b/.test(raw)) throw new DemoError("Choose a valid PDF without scripts or embedded files.", 422);
    kind = "pdf"; mediaType = "application/pdf";
  } else if (["png", "jpg", "jpeg"].includes(extension || "")) {
    const png = data[0] === 137 && data[1] === 80 && data[2] === 78 && data[3] === 71;
    const jpeg = data[0] === 255 && data[1] === 216 && data[2] === 255;
    if (data.length > 10 * 1024 * 1024 || (!png && !jpeg)) throw new DemoError("Choose a PNG or JPEG smaller than 10 MB.", 422);
    kind = "image"; mediaType = png ? "image/png" : "image/jpeg";
  } else throw new DemoError("Supported files are PGN, FEN, PDF, PNG and JPEG.", 422);
  if (certificate && kind !== "pdf") throw new DemoError("Certificate attachments must be PDF files.", 422);
  const info: StoredFile = { id: createUUIDv7(), filename, mediaType, sizeBytes: data.length, status: "clean", ownerId: user.id, ...(assignmentId ? { assignmentId } : {}), ...(certificateId ? { certificateId } : {}) };
  const publicFile: LearningFile = { id: info.id, filename, mediaType, sizeBytes: info.sizeBytes, status: info.status };
  if (assignment) { const attempt = editableAttempt(assignment); (attempt.files ??= []).push(publicFile); assignment.version++; }
  else if (certificate) certificate.fileId = info.id;
  else {
    const title = String(form.get("title") || "").trim(), visibility = String(form.get("visibility") || "personal");
    if (!title || title.length > 200 || !["personal", "shared"].includes(visibility)) throw new DemoError("Add a resource title and visibility.", 422);
    learning.resources.unshift({ id: createUUIDv7(), ownerId: user.id, title, kind, visibility, status: "draft", levelSlug: String(form.get("levelSlug") || ""), tags: String(form.get("tags") || "").split(",").map((tag) => tag.trim()).filter(Boolean), content, version: 1, fileId: info.id, filename, mediaType });
  }
  await putDemoFile(info.id, new Blob([data], { type: mediaType }));
  learning.files.push(info); writeDemo(state);
  return response({ file: publicFile, workspace: demoLearningWorkspace(state) }, 201);
}
