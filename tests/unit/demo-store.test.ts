import { Chess } from "chess.js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { DEMO_CLASS_ID, DEMO_STUDENT_ID, DEMO_STORAGE_KEY } from "@/app/lib/demo/data";
import { isDemoMode } from "@/app/lib/demo/mode";
import { applyDemoAction, getDemoUser, loginDemo, logoutDemo, readDemo, workspaceForUser, writeDemo } from "@/app/lib/demo/store";

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "true");
  vi.stubEnv("NODE_ENV", "test");
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date("2026-09-08T08:00:00Z"));
  localStorage.clear();
  sessionStorage.clear();
});

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  vi.useRealTimers();
  vi.unstubAllEnvs();
});

function expectRejection(input: Record<string, unknown>, status: number) {
  expect(() => applyDemoAction(input)).toThrowError(expect.objectContaining({ status }));
}

function boardInput(game: Chess, revision: number, classId = DEMO_CLASS_ID) {
  return { action: "saveBoard", classId, fen: game.fen(), pgn: game.pgn(), revision };
}

describe("local demo isolation and access", () => {
  it("requires explicit opt in and never enables demo authentication in production", () => {
    vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "false");
    expect(isDemoMode()).toBe(false);
    expect(() => loginDemo("super_admin")).toThrowError(expect.objectContaining({ status: 404 }));
    vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "true");
    vi.stubEnv("NODE_ENV", "production");
    expect(isDemoMode()).toBe(false);
    expect(() => loginDemo("guardian")).toThrowError(expect.objectContaining({ status: 404 }));
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });

  it("keeps account sessions separate from persisted academy data", () => {
    loginDemo("guardian");
    applyDemoAction({ action: "addStudent", name: "Mira", ageBand: "4_7" });
    logoutDemo();
    expect(getDemoUser()).toBeUndefined();
    loginDemo("coach");
    expect(getDemoUser()?.roles).toEqual(["coach"]);
    expect(readDemo().students.some((student) => student.name === "Mira")).toBe(true);
  });

  it("denies unauthenticated actions and student access to guardian or staff actions", () => {
    const initial = readDemo();
    expectRejection({ action: "payInvoice", invoiceId: initial.invoices[0].id, outcome: "success" }, 401);
    loginDemo("student");
    expectRejection({ action: "payInvoice", invoiceId: initial.invoices[0].id, outcome: "success" }, 403);
    expectRejection({ action: "setStudentControl", enabled: true }, 403);
    expectRejection({ action: "addStudent", name: "Unauthorized child", ageBand: "8_12" }, 403);
    expect(readDemo()).toEqual(initial);
  });

  it("does not silently overwrite corrupt saved demo data", () => {
    localStorage.setItem(DEMO_STORAGE_KEY, "not valid JSON");
    expect(() => readDemo()).toThrowError(expect.objectContaining({ status: 409 }));
    expect(localStorage.getItem(DEMO_STORAGE_KEY)).toBe("not valid JSON");
  });

  it("limits student data to the assigned learner and hides billing and draft reports", () => {
    loginDemo("guardian");
    const state = readDemo();
    state.students.push({ id: "other-student", name: "Mira", ageBand: "4_7", level: "Beginner", consent: false });
    state.assignments.push({ ...state.assignments[0], id: "other-assignment", studentId: "other-student" });
    state.reports.push({ ...state.reports[0], id: "other-report", studentId: "other-student" }, { ...state.reports[0], id: "draft-report", status: "draft" });
    state.classes.push({ ...state.classes[0], id: "other-class", studentIds: ["other-student"] });
    writeDemo(state);
    loginDemo("student");
    const visible = workspaceForUser(readDemo());
    expect(visible.students.map((student) => student.id)).toEqual([DEMO_STUDENT_ID]);
    expect(visible.assignments.every((assignment) => assignment.studentId === DEMO_STUDENT_ID)).toBe(true);
    expect(visible.classes.every((lesson) => lesson.studentIds.includes(DEMO_STUDENT_ID))).toBe(true);
    expect(visible.reports.every((report) => report.studentId === DEMO_STUDENT_ID && report.status === "published")).toBe(true);
    expect(visible.invoices).toEqual([]);
    expect(visible.leads).toEqual([]);
    expect(visible.family.credits).toBe(0);
    expect(readDemo().invoices).toHaveLength(1);
  });
});

describe("demo billing and delivery", () => {
  it("does not change an invoice or credits when payment fails", () => {
    loginDemo("guardian");
    const initial = readDemo();
    expectRejection({ action: "payInvoice", invoiceId: initial.invoices[0].id, outcome: "failure" }, 402);
    expect(readDemo()).toEqual(initial);
  });

  it("credits a successful payment only once, including after a retry", () => {
    loginDemo("guardian");
    const initial = readDemo();
    const input = { action: "payInvoice", invoiceId: initial.invoices[0].id, outcome: "success" };
    applyDemoAction(input);
    const paid = readDemo();
    expect(paid.family.credits).toBe(initial.family.credits + initial.invoices[0].credits);
    expect(paid.invoices[0]).toMatchObject({ status: "paid", paidAt: "2026-09-08T08:00:00.000Z" });
    applyDemoAction(input);
    expect(readDemo()).toEqual(paid);
  });

  it("requires an explicit supported payment outcome", () => {
    loginDemo("guardian");
    const initial = readDemo();
    expectRejection({ action: "payInvoice", invoiceId: initial.invoices[0].id, outcome: "unknown" }, 422);
    expectRejection({ action: "payInvoice", invoiceId: initial.invoices[0].id }, 422);
    expect(readDemo()).toEqual(initial);
  });

  it("requires resolved attendance and nonempty notes before completing a lesson", () => {
    loginDemo("coach");
    const initial = readDemo();
    expectRejection({ action: "completeClass", classId: DEMO_CLASS_ID, attendance: {}, notes: "Useful class notes." }, 422);
    expectRejection({ action: "completeClass", classId: DEMO_CLASS_ID, attendance: { [DEMO_STUDENT_ID]: "present" }, notes: "  " }, 422);
    expect(readDemo()).toEqual(initial);
    applyDemoAction({ action: "completeClass", classId: DEMO_CLASS_ID, attendance: { [DEMO_STUDENT_ID]: "present", unrelated: "present" }, notes: "Reviewed checks and knight forks." });
    const completed = readDemo();
    expect(completed.classes[0]).toMatchObject({ status: "completed", notes: "Reviewed checks and knight forks.", attendance: { [DEMO_STUDENT_ID]: "present" } });
    expect(completed.classes[0].attendance).not.toHaveProperty("unrelated");
    expect(completed.family.credits).toBe(initial.family.credits - 1);
    expectRejection({ action: "completeClass", classId: DEMO_CLASS_ID, attendance: { [DEMO_STUDENT_ID]: "present" }, notes: "Duplicate completion attempt." }, 409);
    expect(readDemo()).toEqual(completed);
  });

  it("does not complete a class before it starts", () => {
    loginDemo("coach");
    const initial = readDemo();
    expectRejection({ action: "completeClass", classId: "demo-class-next", attendance: { [DEMO_STUDENT_ID]: "present" }, notes: "This future class has not occurred." }, 409);
    expect(readDemo()).toEqual(initial);
  });

  it("applies the cancellation cutoff without creating or deducting credits", () => {
    loginDemo("guardian");
    const initial = readDemo();
    expectRejection({ action: "cancelClass", classId: DEMO_CLASS_ID }, 409);
    applyDemoAction({ action: "cancelClass", classId: "demo-class-next" });
    const cancelled = readDemo();
    expect(cancelled.classes.find((lesson) => lesson.id === "demo-class-next")?.status).toBe("cancelled");
    expect(cancelled.family.credits).toBe(initial.family.credits);
    expectRejection({ action: "cancelClass", classId: "demo-class-next" }, 409);
    expect(readDemo()).toEqual(cancelled);
  });
});

describe("demo learning workflow", () => {
  it("rejects another student's assignment and preserves their response", () => {
    loginDemo("student");
    const state = readDemo();
    state.assignments.push({ ...state.assignments[0], id: "private-assignment", studentId: "another-child" });
    writeDemo(state);
    expectRejection({ action: "submitAssignment", assignmentId: "private-assignment", answer: "My attempted answer." }, 403);
    expect(readDemo()).toEqual(state);
  });

  it("connects student submission, coach review and visible feedback", () => {
    loginDemo("coach");
    const assignmentId = readDemo().assignments[0].id;
    expectRejection({ action: "reviewAssignment", assignmentId, feedback: "Premature feedback." }, 409);
    loginDemo("student");
    applyDemoAction({ action: "submitAssignment", assignmentId, answer: "The bishop targets f7, defended only by the king." });
    loginDemo("coach");
    applyDemoAction({ action: "reviewAssignment", assignmentId, feedback: "Correct. Always count the defenders." });
    loginDemo("student");
    expect(workspaceForUser(readDemo()).assignments[0]).toMatchObject({ status: "reviewed", feedback: "Correct. Always count the defenders." });
    expectRejection({ action: "submitAssignment", assignmentId, answer: "Overwrite reviewed work." }, 409);
  });
});

describe("demo classroom authority", () => {
  it("requires coach control and accepts exactly one legal student move", () => {
    loginDemo("student");
    const initial = readDemo();
    const game = new Chess();
    game.move("e4");
    expectRejection(boardInput(game, 0), 403);
    expect(readDemo()).toEqual(initial);
    loginDemo("coach");
    applyDemoAction({ action: "setStudentControl", enabled: true });
    loginDemo("student");
    applyDemoAction(boardInput(game, 1));
    expect(readDemo().board).toMatchObject({ fen: game.fen(), pgn: game.pgn(), revision: 2, studentControl: true });
    game.move("e5");
    game.move("Nf3");
    expectRejection(boardInput(game, 2), 422);
    expect(readDemo().board.revision).toBe(2);
  });

  it("rejects stale revisions and mismatched FEN/PGN without overwriting the board", () => {
    loginDemo("coach");
    const game = new Chess();
    game.move("e4");
    applyDemoAction(boardInput(game, 0));
    const saved = readDemo();
    const alternative = new Chess();
    alternative.move("d4");
    expectRejection(boardInput(alternative, 0), 409);
    expectRejection({ ...boardInput(alternative, 1), pgn: game.pgn() }, 422);
    expect(readDemo()).toEqual(saved);
  });

  it.each(["consent", "assignment", "join window"] as const)("denies student moves without %s", (missing) => {
    loginDemo("coach");
    applyDemoAction({ action: "setStudentControl", enabled: true });
    const state = readDemo();
    if (missing === "consent") state.students[0].consent = false;
    if (missing === "assignment") state.classes[0].studentIds = ["another-child"];
    if (missing === "join window") state.classes[0].startsAt = "2026-09-09T08:00:00.000Z";
    writeDemo(state);
    loginDemo("student");
    const game = new Chess();
    game.move("e4");
    expectRejection(boardInput(game, 1), 403);
    expect(readDemo()).toEqual(state);
  });
});
