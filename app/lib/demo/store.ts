import { clearDemoFiles } from "./learning-file-store";
import { Chess } from "chess.js";
import type { AcademyRole, SessionUser } from "../api/types";
import { createDemoWorkspace, DEMO_CLASS_ID, DEMO_SESSION_KEY, DEMO_STORAGE_KEY, demoUser, type DemoWorkspace } from "./data";
import { isDemoMode } from "./mode";

export class DemoError extends Error {
  constructor(message: string, public status = 400) { super(message); }
}

function requireDemo() {
  if (!isDemoMode()) throw new DemoError("Local demo mode is disabled.", 404);
}

export function readDemo(): DemoWorkspace {
  requireDemo();
  if (typeof window === "undefined") return createDemoWorkspace();
  const raw = localStorage.getItem(DEMO_STORAGE_KEY);
  if (raw) {
    try { return JSON.parse(raw) as DemoWorkspace; }
    catch { throw new DemoError("Demo data could not be read. Reset the sample academy from the demo page.", 409); }
  }
  const state = createDemoWorkspace();
  writeDemo(state);
  return state;
}

export function writeDemo(state: DemoWorkspace) {
  requireDemo();
  if (typeof window !== "undefined") {
    localStorage.setItem(DEMO_STORAGE_KEY, JSON.stringify(state));
    window.dispatchEvent(new Event("kingsgambit:demo-change"));
  }
}

export function getDemoUser(): SessionUser | undefined {
  requireDemo();
  if (typeof window === "undefined") return undefined;
  const role = sessionStorage.getItem(DEMO_SESSION_KEY) as AcademyRole | null;
  if (!role) return undefined;
  const user = demoUser(role);
  const staff = readDemo().academy?.staff?.find((person) => person.id === user.id);
  if (staff?.status === "suspended") { sessionStorage.removeItem(DEMO_SESSION_KEY); return undefined; }
  if (staff) user.displayName = staff.displayName;
  return user;
}

export function loginDemo(role: AcademyRole) {
  requireDemo();
  const user = demoUser(role);
  sessionStorage.setItem(DEMO_SESSION_KEY, role);
  const staff = readDemo().academy?.staff?.find((person) => person.id === user.id);
  if (staff?.status === "suspended") { sessionStorage.removeItem(DEMO_SESSION_KEY); throw new DemoError("This account is suspended.", 403); }
  return { authenticated: true as const, user, csrfToken: "local-demo-csrf" };
}

export function logoutDemo() {
  requireDemo();
  sessionStorage.removeItem(DEMO_SESSION_KEY);
}

export function resetDemo() {
  requireDemo();
  clearDemoFiles();
  writeDemo(createDemoWorkspace());
}

export function requireRole(...roles: AcademyRole[]) {
  const user = getDemoUser();
  if (!user) throw new DemoError("Sign in to a demo account to continue.", 401);
  if (!user.roles.some((role) => roles.includes(role))) throw new DemoError("This account cannot perform that action.", 403);
  return user;
}

function visibleLegacyStudents(state: DemoWorkspace, user: SessionUser) {
  if (user.roles.some((role) => ["academic_manager", "operations_admin", "finance_admin", "super_admin"].includes(role))) return new Set(state.students.map((student) => student.id));
  const academy = state.academy;
  if (!academy) {
    if (user.roles.includes("student")) return new Set([user.id]);
    if (user.roles.includes("guardian")) return new Set(state.students.map((student) => student.id));
    return new Set(state.classes.filter((lesson) => lesson.coach === "Coach Maya").flatMap((lesson) => lesson.studentIds));
  }
  const coachBatches = new Set(academy.batches.filter((batch) => batch.coachId === user.id).map((batch) => batch.id));
  return new Set(academy.students.filter((student) =>
    (user.roles.includes("student") && student.userId === user.id) ||
    (user.roles.includes("guardian") && academy.guardianEmails[student.id] === user.email) ||
    (user.roles.includes("coach") && academy.enrolments.some((enrolment) => enrolment.studentId === student.id && enrolment.status === "active" && coachBatches.has(enrolment.batchId)))
  ).map((student) => student.id));
}

function emptyDemoBoard(): DemoWorkspace["board"] {
  return { fen: new Chess().fen(), pgn: "", revision: 0, studentControl: false };
}
function boardForClass(state: DemoWorkspace, classId: string) {
  return classId === DEMO_CLASS_ID ? state.board : state.boards?.[classId] || emptyDemoBoard();
}
function saveClassBoard(state: DemoWorkspace, classId: string, board: DemoWorkspace["board"]) {
  if (classId === DEMO_CLASS_ID) state.board = board;
  else { state.boards ||= {}; state.boards[classId] = board; }
}

export function workspaceForUser(state: DemoWorkspace, classId = DEMO_CLASS_ID): DemoWorkspace {
  const user = requireRole("guardian", "student", "coach", "academic_manager", "operations_admin", "finance_admin", "super_admin");
  const role = user.roles[0];
  const result = structuredClone(state);
  delete result.learning;
  delete result.academy;
  delete result.boards;
  const ids = visibleLegacyStudents(state, user);
  result.students = result.students.filter((student) => ids.has(student.id));
  result.classes = result.classes.filter((lesson) => {
    if (["operations_admin", "academic_manager", "super_admin"].includes(role)) return true;
    if (role === "finance_admin") return false;
    if (role === "coach") return state.academy ? state.academy.classes.some((item) => item.id === lesson.id && item.coachId === user.id) : lesson.coach === "Coach Maya";
    return lesson.studentIds.some((id) => ids.has(id));
  });
  if (["guardian", "student"].includes(role)) result.classes = result.classes.map((lesson) => ({ ...lesson, studentIds: lesson.studentIds.filter((id) => ids.has(id)), attendance: Object.fromEntries(Object.entries(lesson.attendance).filter(([id]) => ids.has(id))) }));
  const canReadLearning = ["guardian", "student", "coach", "academic_manager", "super_admin"].includes(role);
  result.assignments = canReadLearning ? result.assignments.filter((assignment) => ids.has(assignment.studentId)) : [];
  result.reports = canReadLearning ? result.reports.filter((report) => ids.has(report.studentId) && (!["guardian", "student"].includes(role) || report.status === "published")) : [];
  if (!["guardian", "finance_admin", "super_admin"].includes(role)) { result.invoices = []; result.family.credits = 0; }
  if (!["operations_admin", "super_admin"].includes(role)) { result.leads = []; result.bookings = []; result.audit = []; }
  if (!["guardian", "operations_admin", "super_admin"].includes(role)) result.supportCases = [];
  else if (role === "guardian") result.supportCases = result.supportCases.filter((item) => item.familyId === state.family.id).map((item) => ({ ...item, events: item.events.filter((event) => event.visibility === "customer_visible") }));
  result.board = result.classes.some((lesson) => lesson.id === classId) && ["coach", "student", "academic_manager", "operations_admin", "super_admin"].includes(role) ? structuredClone(boardForClass(state, classId)) : emptyDemoBoard();
  return result;
}

function requiredText(input: Record<string, unknown>, key: string, limit = 5000) {
  const value = typeof input[key] === "string" ? input[key].trim() : "";
  if (!value || value.length > limit) throw new DemoError(`Enter ${key.replace(/([A-Z])/g, " $1").toLowerCase()} (up to ${limit} characters).`, 422);
  return value;
}

function findItem<T extends { id: string }>(items: T[], id: unknown): T {
  const item = items.find((entry) => entry.id === id);
  if (!item) throw new DemoError("That record was not found.", 404);
  return item;
}

export function applyDemoAction(input: Record<string, unknown>): DemoWorkspace {
  const state = readDemo();
  const action = String(input.action);
  if (state.academy) requireRole("guardian", "student", "coach", "academic_manager", "operations_admin", "finance_admin", "super_admin");
  if (state.academy && !["saveBoard", "setStudentControl", "payInvoice"].includes(action)) {
    throw new DemoError("Use the academy workspace form to update this record.", 409);
  }
  let user: SessionUser;
  switch (action) {
    case "addStudent": {
      user = requireRole("guardian", "super_admin");
      const name = requiredText(input, "name", 100);
      if (!["4_7", "8_12", "13_17", "18_plus"].includes(String(input.ageBand))) throw new DemoError("Choose an age band.", 422);
      state.students.push({ id: crypto.randomUUID(), name, ageBand: String(input.ageBand), level: "Awaiting assessment", consent: false });
      break;
    }
    case "acceptConsent": {
      user = requireRole("guardian", "super_admin");
      const student = findItem(state.students, input.studentId);
      student.consent = true; student.consentVersion = "demo-1"; student.consentAt = new Date().toISOString();
      break;
    }
    case "payInvoice": {
      user = requireRole("guardian", "finance_admin", "super_admin");
      if (input.outcome !== "success" && input.outcome !== "failure") throw new DemoError("Choose a simulated payment outcome.", 422);
      const invoice = findItem(state.invoices, input.invoiceId);
      if (invoice.status === "paid") return workspaceForUser(state);
      if (input.outcome === "failure") throw new DemoError("The simulated payment was declined. Your invoice is still due; try again.", 402);
      invoice.status = "paid"; invoice.paidAt = new Date().toISOString(); state.family.credits += invoice.credits;
      const account = state.academy?.enrolments.find((item) => item.status === "active" && state.academy?.students.some((student) => student.id === item.studentId && student.familyId === state.family.id));
      if (account) account.available += invoice.credits;
      break;
    }
    case "submitAssignment": {
      user = requireRole("student", "super_admin");
      const assignment = findItem(state.assignments, input.assignmentId);
      if (user.roles[0] === "student" && assignment.studentId !== user.id) throw new DemoError("This assignment belongs to another student.", 403);
      if (assignment.status === "reviewed") throw new DemoError("This assignment has already been reviewed.", 409);
      assignment.answer = requiredText(input, "answer"); assignment.status = "submitted";
      break;
    }
    case "reviewAssignment": {
      user = requireRole("coach", "academic_manager", "super_admin");
      const assignment = findItem(state.assignments, input.assignmentId);
      if (assignment.status !== "submitted") throw new DemoError("Wait for the student to submit their work.", 409);
      assignment.feedback = requiredText(input, "feedback"); assignment.status = "reviewed";
      break;
    }
    case "completeClass": {
      user = requireRole("coach", "super_admin");
      const lesson = findItem(state.classes, input.classId);
      if (lesson.status !== "scheduled") throw new DemoError("This class has already been resolved.", 409);
      if (Date.parse(lesson.startsAt) > Date.now()) throw new DemoError("The class has not started yet.", 409);
      const attendance = input.attendance as DemoWorkspace["classes"][number]["attendance"] | undefined;
      if (!attendance || lesson.studentIds.some((id) => !["present", "absent", "excused"].includes(attendance[id]))) throw new DemoError("Resolve attendance for every student before completing the class.", 422);
      const notes = requiredText(input, "notes");
      if (state.family.credits < 1) throw new DemoError("No class credits remain.", 409);
      lesson.notes = notes; lesson.attendance = Object.fromEntries(lesson.studentIds.map((id) => [id, attendance[id]])); lesson.status = "completed";
      state.family.credits -= 1;
      break;
    }
    case "cancelClass": {
      user = requireRole("guardian", "operations_admin", "super_admin");
      const lesson = findItem(state.classes, input.classId);
      if (lesson.status !== "scheduled") throw new DemoError("Only scheduled classes can be cancelled.", 409);
      if (new Date(lesson.startsAt).getTime() - Date.now() < 24 * 3_600_000) throw new DemoError("The 24-hour cancellation window has closed. Contact academy support.", 409);
      lesson.status = "cancelled";
      break;
    }
    case "updateLead": {
      user = requireRole("operations_admin", "super_admin");
      if (!["new", "contacted", "trial_booked", "enrolled", "closed"].includes(String(input.status))) throw new DemoError("Choose a valid lead stage.", 422);
      findItem(state.leads, input.leadId).status = String(input.status);
      break;
    }
    case "acknowledgeReport": {
      user = requireRole("guardian", "super_admin");
      const report = findItem(state.reports, input.reportId);
      if (report.status !== "published") throw new DemoError("This report has not been published.", 409);
      report.acknowledgedAt ||= new Date().toISOString();
      break;
    }
    case "publishReport": {
      user = requireRole("academic_manager", "super_admin");
      findItem(state.reports, input.reportId).status = "published";
      break;
    }
    case "saveBoard": {
      user = requireRole("coach", "student", "super_admin");
      const classId = String(input.classId || "");
      if (!workspaceForUser(state, classId).classes.some((lesson) => lesson.id === classId)) throw new DemoError("This classroom is not assigned to you.", 403);
      const lesson = findItem(state.classes, classId);
      const board = boardForClass(state, classId);
      const now = Date.now();
      if (lesson.status !== "scheduled" || now < Date.parse(lesson.startsAt) - 15 * 60_000 || now > Date.parse(lesson.endsAt) + 15 * 60_000) throw new DemoError("This classroom is outside its join window.", 403);
      if (user.roles[0] === "student") {
        if (!lesson.studentIds.includes(user.id) || !findItem(state.students, user.id).consent) throw new DemoError("Classroom access requires assignment and guardian consent.", 403);
        if (!board.studentControl) throw new DemoError("Wait for your coach to give you board control.", 403);
      }
      if (input.revision !== board.revision) throw new DemoError("The board changed in another tab. Refresh the position and try again.", 409);
      const fen = requiredText(input, "fen", 200);
      const pgn = typeof input.pgn === "string" ? input.pgn : "";
      try {
        new Chess(fen);
        if (pgn) { const replay = new Chess(); replay.loadPgn(pgn); if (replay.fen() !== fen) throw new Error("Position mismatch"); }
        if (user.roles[0] === "student") {
          const current = new Chess(board.fen);
          const legal = current.moves({ verbose: true }).some((move) => { const candidate = new Chess(board.fen); candidate.move(move); return candidate.fen() === fen; });
          if (!legal) throw new Error("Only one legal move is allowed");
        }
      } catch { throw new DemoError("Provide a legal move or a valid matching FEN and PGN.", 422); }
      saveClassBoard(state, classId, { ...board, fen, pgn, revision: board.revision + 1 });
      break;
    }
    case "setStudentControl": {
      user = requireRole("coach", "super_admin");
      const classId = String(input.classId || DEMO_CLASS_ID);
      if (!workspaceForUser(state, classId).classes.some((lesson) => lesson.id === classId)) throw new DemoError("This classroom is not assigned to you.", 403);
      const board = boardForClass(state, classId);
      saveClassBoard(state, classId, { ...board, studentControl: input.enabled === true, revision: board.revision + 1 });
      break;
    }
    default: throw new DemoError("That demo action is not available.", 404);
  }
  state.audit.unshift({ id: crypto.randomUUID(), actor: user.displayName || user.id, action, at: new Date().toISOString() });
  writeDemo(state);
  return workspaceForUser(state, typeof input.classId === "string" ? input.classId : undefined);
}
