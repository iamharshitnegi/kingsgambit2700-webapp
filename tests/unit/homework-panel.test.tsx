import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Chess } from "chess.js";
import HomeworkPanel from "@/app/components/portal/workspace/HomeworkPanel";
import AssignmentEditor from "@/app/components/portal/workspace/homework/AssignmentEditor";
import PracticeDialog from "@/app/components/portal/workspace/homework/PracticeDialog";
import { demoAcademyWorkspace } from "@/app/lib/demo/academy";
import { applyDemoLearningAction, demoLearningWorkspace } from "@/app/lib/demo/learning";
import { sampleHomework } from "@/app/lib/demo/homework-samples";
import { demoUser, DEMO_STUDENT_ID } from "@/app/lib/demo/data";
import { loginDemo, readDemo, writeDemo } from "@/app/lib/demo/store";

vi.mock("@/app/components/chess/AccessibleChessboard", () => ({ default: () => <div aria-label="Chessboard" /> }));

beforeEach(() => {
  vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "true");
  vi.stubEnv("NODE_ENV", "test");
  localStorage.clear(); sessionStorage.clear();
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute("open", ""); };
  HTMLDialogElement.prototype.close = function () { this.removeAttribute("open"); };
  vi.stubGlobal("matchMedia", () => ({ matches: false }));
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); vi.unstubAllEnvs(); });

function coachWorkspace() {
  loginDemo("coach");
  const state = readDemo();
  const data = demoAcademyWorkspace(state);
  const learning = demoLearningWorkspace(state);
  return { data, learning };
}

describe("homework access and authoring", () => {
  it("shows a student only their published homework without learner or staff controls", async () => {
    const { data, learning } = coachWorkspace();
    const foreign = { ...data.students[0], id: "another-profile", userId: "another-user", name: "Another learner" };
    data.students.push(foreign);
    learning.students.push({ ...learning.students[0], id: foreign.id, name: foreign.name });
    learning.assignments.push({ ...learning.assignments[0], id: "foreign-assignment", studentId: foreign.id, title: "Another learner’s work" });
    render(<HomeworkPanel data={data} learning={learning} user={demoUser("student")} kind="student" act={vi.fn()} onRefresh={vi.fn()} />);
    await userEvent.setup().click(screen.getByRole("button", { name: /^All homework/ }));
    expect(screen.getByRole("button", { name: "View The art of the checkmate" })).toBeVisible();
    expect(screen.queryByRole("button", { name: /Another learner’s work|Build a better thinking routine|Create homework|Edit draft/ })).not.toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /Learner|Batch/ })).not.toBeInTheDocument();
  });

  it("keeps the guardian view read-only", () => {
    loginDemo("guardian"); const state = readDemo();
    render(<HomeworkPanel data={demoAcademyWorkspace(state)} learning={demoLearningWorkspace(state)} user={demoUser("guardian")} kind="guardian" act={vi.fn()} onRefresh={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /Start homework|Continue practice|Improve my answer|Create homework|Submit practice/ })).not.toBeInTheDocument();
  });

  it("validates an audience, preserves content going back, and writes only after final review", async () => {
    const user = userEvent.setup(); const act = vi.fn().mockResolvedValue(true); const saved = vi.fn();
    render(<AssignmentEditor {...coachWorkspace()} act={act} close={vi.fn()} saved={saved} />);
    const dialog = within(screen.getByRole("dialog"));
    await user.type(dialog.getByLabelText("Homework title"), "Calculate before moving");
    await user.type(dialog.getByLabelText("Instructions"), "Describe two candidate moves and compare them.");
    await user.click(dialog.getByRole("button", { name: "Continue" }));
    await user.click(dialog.getByRole("button", { name: "Continue" }));
    expect(dialog.getByRole("alert")).toHaveTextContent("Select at least one learner.");
    expect(act).not.toHaveBeenCalled();
    await user.click(dialog.getByRole("checkbox", { name: "Aarav" }));
    await user.click(dialog.getByRole("radio", { name: /Assign to learners/ }));
    await user.click(dialog.getByRole("button", { name: "Back" }));
    expect(dialog.getByLabelText("Homework title")).toHaveValue("Calculate before moving");
    await user.click(dialog.getByRole("button", { name: "Continue" }));
    expect(dialog.getByRole("checkbox", { name: "Aarav" })).toBeChecked();
    await user.click(dialog.getByRole("button", { name: "Continue" }));
    expect(dialog.getByText(/Each learner sees only their own homework/)).toBeVisible();
    expect(act).not.toHaveBeenCalled();
    await user.click(dialog.getByRole("button", { name: "Assign homework" }));
    expect(act).toHaveBeenCalledWith("createAssignment", expect.objectContaining({ title: "Calculate before moving", studentIds: [DEMO_STUDENT_ID], status: "assigned", kind: "study" }), expect.any(String));
    expect(saved).toHaveBeenCalledWith("assigned");
  });

  it("preserves an existing draft and its version when saving fails", async () => {
    const user = userEvent.setup(); const workspace = coachWorkspace(); const saved = vi.fn();
    const item = workspace.learning.assignments.find(item => item.status === "draft")!;
    const act = vi.fn().mockResolvedValue(false);
    render(<AssignmentEditor {...workspace} item={item} act={act} close={vi.fn()} saved={saved} />);
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await user.click(screen.getByRole("button", { name: "Save changes" }));
    expect(act).toHaveBeenCalledWith("editAssignment", expect.objectContaining({ id: item.id, version: item.version, title: item.title, studentIds: [item.studentId], status: "draft" }), expect.any(String));
    expect(screen.getByRole("alert")).toHaveTextContent("wasn’t saved");
    expect(saved).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Back" }));
    await user.click(screen.getByRole("button", { name: "Back" }));
    expect(screen.getByLabelText("Homework title")).toHaveValue(item.title);
  });

  it("starts a fresh puzzle attempt after coach return without replaying old solved moves", async () => {
    const user = userEvent.setup(); const assignment = sampleHomework(DEMO_STUDENT_ID)[0]; const act = vi.fn().mockResolvedValue(true);
    assignment.attempts[0].status = "returned";
    render(<PracticeDialog assignment={assignment} act={act} close={vi.fn()} submitted={vi.fn()} keepDraft={vi.fn()} />);
    expect(screen.getByLabelText("Puzzle")).toHaveValue("0");
    await user.type(screen.getByLabelText("Your move"), "Re8#");
    await user.click(screen.getByRole("button", { name: "Check move" }));
    expect(act).toHaveBeenCalledWith("puzzleMove", expect.objectContaining({ id: assignment.id, puzzleIndex: 0, move: "e1e8", version: assignment.version }), expect.any(String));
  });
});

describe("sample homework migration", () => {
  it("adds samples to an existing demo once without replacing edited records", () => {
    coachWorkspace(); let state = readDemo();
    const original = state.learning!.assignments[0]; original.title = "My existing edited homework";
    state.learning!.assignments = [original]; delete state.learning!.homeworkSamplesVersion; writeDemo(state);
    demoLearningWorkspace(readDemo());
    state = readDemo(); const sample = state.learning!.assignments.find(item => item.id === "demo-homework-opening")!;
    sample.title = "An edited sample"; writeDemo(state);
    const result = demoLearningWorkspace(readDemo());
    expect(result.assignments).toHaveLength(9);
    expect(result.assignments[0].title).toBe("My existing edited homework");
    expect(result.assignments.find(item => item.id === sample.id)?.title).toBe("An edited sample");
    loginDemo("student"); const student = demoLearningWorkspace(readDemo());
    expect(student.assignments).toHaveLength(8);
    expect(student.assignments.every(item => item.status !== "draft" && item.puzzles.every(puzzle => !puzzle.solutions))).toBe(true);
  });

  it("ships legal mating lines and counts unplayed sample puzzles correctly after a move", () => {
    for (const puzzle of sampleHomework(DEMO_STUDENT_ID)[0].puzzles) {
      const chess = new Chess(puzzle.fen);
      for (const move of puzzle.solutions![0]) chess.move({ from: move.slice(0, 2), to: move.slice(2, 4) });
      expect(chess.isCheckmate(), puzzle.title).toBe(true);
    }
    coachWorkspace(); loginDemo("student");
    const result = applyDemoLearningAction(readDemo(), { action: "puzzleMove", id: "demo-homework-tactics", version: 1, puzzleIndex: 2, move: "a1a8", elapsedSeconds: 0 });
    expect(result.assignments.find(item => item.id === "demo-homework-tactics")?.attempts[0].metrics).toMatchObject({ completed: 3, notAttempted: 2, inProgress: 0 });
  });
});
