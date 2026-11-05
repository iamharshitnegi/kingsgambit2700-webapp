import { Chess } from "chess.js";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { demoAcademyWorkspace } from "@/app/lib/demo/academy";
import { applyDemoAction, loginDemo, readDemo, workspaceForUser, writeDemo } from "@/app/lib/demo/store";
import { DEMO_CLASS_ID, DEMO_STUDENT_ID } from "@/app/lib/demo/data";

beforeEach(() => { vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "true"); vi.stubEnv("NODE_ENV", "test"); localStorage.clear(); sessionStorage.clear(); });
afterEach(() => { vi.unstubAllEnvs(); localStorage.clear(); sessionStorage.clear(); });

function multiFamilyState() {
  loginDemo("super_admin");
  const state = readDemo(); demoAcademyWorkspace(state);
  const academy = state.academy!;
  academy.students.push({ ...academy.students[0], id: "foreign-student", userId: "foreign-user", name: "Foreign learner", familyId: "foreign-family" });
  academy.guardianEmails["foreign-student"] = "other-family@example.test";
  state.students.push({ ...state.students[0], id: "foreign-student", name: "Foreign learner" });
  state.assignments.push({ ...state.assignments[0], id: "foreign-assignment", studentId: "foreign-student" });
  state.reports.push({ ...state.reports[0], id: "foreign-report", studentId: "foreign-student", status: "published" });
  state.classes[0].studentIds.push("foreign-student"); state.classes[0].attendance["foreign-student"] = "absent";
  academy.classes[0].studentIds.push("foreign-student"); academy.classes[0].attendance["foreign-student"] = "absent";
  state.classes.push({ ...state.classes[0], id: "other-coach-class", coach: "Other coach" });
  academy.classes.push({ ...academy.classes[0], id: "other-coach-class", coachId: "other-coach" });
  writeDemo(state); return state;
}

describe("legacy demo compatibility boundaries", () => {
  it("redacts unrelated learner data and family rosters through the legacy workspace", () => {
    multiFamilyState();
    for (const role of ["guardian", "student"] as const) {
      loginDemo(role); const view = workspaceForUser(readDemo());
      expect(view.students.map((student) => student.id)).toEqual([DEMO_STUDENT_ID]);
      expect(view.assignments.some((item) => item.studentId === "foreign-student")).toBe(false);
      expect(view.reports.some((item) => item.studentId === "foreign-student")).toBe(false);
      expect(view.classes.every((item) => !item.studentIds.includes("foreign-student") && !Object.hasOwn(item.attendance, "foreign-student"))).toBe(true);
      expect(view.academy).toBeUndefined(); expect(view.learning).toBeUndefined(); expect(view.boards).toBeUndefined();
    }
    loginDemo("coach"); expect(workspaceForUser(readDemo()).classes.some((lesson) => lesson.id === "other-coach-class")).toBe(false);
    for (const role of ["operations_admin", "finance_admin"] as const) {
      loginDemo(role); const view = workspaceForUser(readDemo()); expect(view.assignments).toEqual([]); expect(view.reports).toEqual([]);
      if (role === "finance_admin") expect(view.classes).toEqual([]);
    }
  });

  it("keeps boards and move control separate between classes and rejects unassigned coaches", () => {
    const state = multiFamilyState();
    state.classes.push({ ...state.classes[0], id: "second-coach-class" });
    state.academy!.classes.push({ ...state.academy!.classes[0], id: "second-coach-class" }); writeDemo(state);
    loginDemo("coach"); const firstGame = new Chess(); firstGame.move("e4");
    applyDemoAction({ action: "saveBoard", classId: DEMO_CLASS_ID, fen: firstGame.fen(), pgn: firstGame.pgn(), revision: 0 });
    expect(workspaceForUser(readDemo(), "second-coach-class").board).toMatchObject({ pgn: "", revision: 0 });
    const secondGame = new Chess(); secondGame.move("d4");
    applyDemoAction({ action: "saveBoard", classId: "second-coach-class", fen: secondGame.fen(), pgn: secondGame.pgn(), revision: 0 });
    applyDemoAction({ action: "setStudentControl", classId: "second-coach-class", enabled: true });
    expect(workspaceForUser(readDemo(), DEMO_CLASS_ID).board).toMatchObject({ fen: firstGame.fen(), studentControl: false, revision: 1 });
    expect(workspaceForUser(readDemo(), "second-coach-class").board).toMatchObject({ fen: secondGame.fen(), studentControl: true, revision: 2 });
    expect(() => applyDemoAction({ action: "saveBoard", classId: "other-coach-class", fen: firstGame.fen(), pgn: firstGame.pgn(), revision: 0 })).toThrow(/not assigned/);
    expect(() => applyDemoAction({ action: "setStudentControl", classId: "other-coach-class", enabled: true })).toThrow(/not assigned/);
  });

  it("requires modern forms after initialization so legacy writes cannot bypass current policies", () => {
    const state = multiFamilyState(); loginDemo("super_admin");
    expect(() => applyDemoAction({ action: "publishReport", reportId: state.reports[0].id })).toThrow(/academy workspace form/);
    expect(readDemo()).toEqual(state);
  });
});
