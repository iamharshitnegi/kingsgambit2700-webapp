import { Blob as NodeBlob, File as NodeFile } from "node:buffer";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { Chess } from "chess.js";
import { demoFetch } from "@/app/lib/demo/transport";
import { loginDemo, readDemo, writeDemo } from "@/app/lib/demo/store";
import { DEMO_STUDENT_ID } from "@/app/lib/demo/data";
import { academyLevels, type AssessmentContent, type LearningWorkspace } from "@/app/lib/academy-types";

beforeEach(() => { vi.stubEnv("NEXT_PUBLIC_DEMO_MODE", "true"); vi.stubEnv("NODE_ENV", "test"); localStorage.clear(); sessionStorage.clear(); vi.stubGlobal("File", NodeFile); vi.stubGlobal("Blob", NodeBlob); });
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); localStorage.clear(); sessionStorage.clear(); });
async function request(path = "/learning/workspace", method = "GET", body?: Record<string, unknown>) {
  return demoFetch(`http://localhost/api/v1${path}`, { method, ...(body ? { headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) } : {}) });
}
async function workspace() { const response = await request(); expect(response.status).toBe(200); return await response.json() as LearningWorkspace; }
async function action(input: Record<string, unknown>, expected = 200) {
  const response = await request("/learning/actions", "POST", input); const data = await response.json(); expect(response.status, JSON.stringify(data)).toBe(expected); return data as LearningWorkspace;
}
async function puzzleAssignment() {
  loginDemo("coach");
  const result = await action({ action: "createAssignment", studentId: DEMO_STUDENT_ID, title: "Opening ideas", description: "Develop your central pawn and knight.", kind: "puzzle_set", status: "assigned", puzzles: [{ title: "Develop pieces", fen: new Chess().fen(), solutions: [["e2e4", "e7e5", "g1f3"]] }] });
  return result.assignments[0];
}
function assessmentContent(): AssessmentContent {
  return { learned: "Central control", strengths: "Piece safety", focusNext: "Planning", practicePlan: "Three puzzles daily", coachComment: "Growing confidence", eligible: true, ratings: ["Piece safety", "Tactical patterns", "Planning"].map((dimension) => ({ dimension, rating: "secure", evidence: "Observed in class and practice" })) };
}

describe("learning demo contract and privacy", () => {
  it("requires sign-in and scopes all role capabilities", async () => {
    expect((await request()).status).toBe(401);
    for (const role of ["student", "guardian", "coach", "academic_manager", "operations_admin", "finance_admin", "super_admin"] as const) {
      loginDemo(role); const view = await workspace();
      expect(view.canCreateResources).toBe(["coach", "academic_manager", "super_admin"].includes(role));
      expect(view.canPublish).toBe(["academic_manager", "super_admin"].includes(role));
      expect(view.students.every((student) => student.canSubmit === (role === "student"))).toBe(true);
      if (["operations_admin", "finance_admin"].includes(role)) { expect(view.assignments).toEqual([]); expect(view.assessments).toEqual([]); }
    }
  });
  it("does not expose draft homework or solutions through either demo endpoint", async () => {
    const assignment = await puzzleAssignment();
    await action({ action: "createAssignment", studentId: DEMO_STUDENT_ID, title: "Private draft", description: "Not ready to share", kind: "study", status: "draft" });
    for (const role of ["student", "guardian"] as const) {
      loginDemo(role); const view = await workspace();
      expect(view.assignments.find((item) => item.id === assignment.id)?.puzzles[0].solutions).toBeUndefined();
      expect(view.assignments.some((item) => item.title === "Private draft")).toBe(false);
      const legacy = await (await request("/demo/workspace")).json();
      expect(legacy.learning).toBeUndefined(); expect(legacy.academy).toBeUndefined();
    }
  });
  it("enforces assigned coach scope and preserves state after rejected creation", async () => {
    loginDemo("coach"); await workspace();
    const state = readDemo(); state.academy!.students.push({ ...state.academy!.students[0], id: "other-profile", userId: "other-user", name: "Other learner" }); writeDemo(state);
    await action({ action: "createAssignment", studentId: "other-profile", title: "Unauthorized", description: "Cannot assign this learner", kind: "study", status: "assigned" }, 403);
    expect(readDemo()).toEqual(state);
  });
});

describe("homework demo lifecycle", () => {
  it("counts unfinished puzzle work as in progress until submission", async () => {
    const assignment = await puzzleAssignment(); loginDemo("student");
    let result = await action({ action: "puzzleMove", id: assignment.id, version: assignment.version, puzzleIndex: 0, move: "d2d4", elapsedSeconds: 0 });
    let homework = result.assignments[0];
    expect(homework.attempts[0].metrics).toMatchObject({ completed: 0, notSolved: 0, inProgress: 1, notAttempted: 0, accuracy: null });
    result = await action({ action: "submitAssignment", id: homework.id, version: homework.version }); homework = result.assignments[0];
    expect(homework.attempts[0].metrics).toMatchObject({ completed: 0, notSolved: 1, inProgress: 0, accuracy: 0 });
    expect((await workspace()).assignments[0].attempts[0].metrics).toEqual(homework.attempts[0].metrics);
  });
  it("checks puzzle answers, auto-replies, records hints and keeps immutable attempts", async () => {
    const assignment = await puzzleAssignment(); loginDemo("student");
    let result = await action({ action: "puzzleMove", id: assignment.id, version: assignment.version, puzzleIndex: 0, move: "d2d4", elapsedSeconds: 0 });
    let homework = result.assignments[0]; expect(homework.attempts[0].puzzles[0].incorrect).toBe(1);
    result = await action({ action: "puzzleHint", id: homework.id, version: homework.version, puzzleIndex: 0, elapsedSeconds: 0 }); homework = result.assignments[0]; expect(homework.attempts[0].puzzles[0].hint).toContain("e2");
    result = await action({ action: "puzzleMove", id: homework.id, version: homework.version, puzzleIndex: 0, move: "e2e4", elapsedSeconds: 0 }); homework = result.assignments[0]; expect(homework.attempts[0].puzzles[0].moves).toEqual(["e2e4", "e7e5"]);
    result = await action({ action: "puzzleMove", id: homework.id, version: homework.version, puzzleIndex: 0, move: "g1f3", elapsedSeconds: 0 }); homework = result.assignments[0]; expect(homework.attempts[0].metrics).toMatchObject({ completed: 1, firstTry: 0, incorrect: 1, hints: 1, accuracy: 100 });
    result = await action({ action: "submitAssignment", id: homework.id, version: homework.version }); homework = result.assignments[0];
    loginDemo("coach"); result = await action({ action: "returnAssignment", id: homework.id, version: homework.version, feedback: "Try again and explain your plan." }); homework = result.assignments[0];
    loginDemo("student"); result = await action({ action: "puzzleReveal", id: homework.id, version: homework.version, puzzleIndex: 0, elapsedSeconds: 0 }); homework = result.assignments[0];
    expect(homework.attempts).toHaveLength(2); expect(homework.attempts[0].status).toBe("returned"); expect(homework.attempts[1].metrics).toMatchObject({ completed: 0, notSolved: 1, reveals: 1 });
  });
  it("rejects stale updates and role misuse without changing saved homework", async () => {
    const assignment = await puzzleAssignment(); loginDemo("student");
    await action({ action: "puzzleHint", id: assignment.id, version: assignment.version, puzzleIndex: 0, elapsedSeconds: 0 }); const saved = readDemo();
    await action({ action: "submitAssignment", id: assignment.id, version: assignment.version }, 409);
    await action({ action: "closeAssignment", id: assignment.id, version: assignment.version + 1 }, 403);
    expect(readDemo()).toEqual(saved);
  });
  it("snapshots published resources and preserves attachments after archival", async () => {
    loginDemo("coach"); let result = await action({ action: "createResource", title: "Coach notes", kind: "study", content: "Look at forcing moves", visibility: "personal", status: "published" }); const resource = result.resources[0];
    result = await action({ action: "createAssignment", studentId: DEMO_STUDENT_ID, title: "Study the note", description: "Explain the idea", kind: "study", status: "assigned", resourceIds: [resource.id] }); const homework = result.assignments[0];
    await action({ action: "archiveResource", id: resource.id, version: resource.version }); loginDemo("student"); result = await workspace();
    expect(result.resources).toEqual([]); expect(result.assignments.find((item) => item.id === homework.id)?.resources?.[0].content).toBe("Look at forcing moves");
  });
  it("revises library material without rewriting assigned content", async () => {
    loginDemo("coach"); let result = await action({ action: "createResource", title: "Original notes", kind: "study", content: "Check forcing moves", visibility: "personal", status: "published" });
    let resource = result.resources[0];
    await action({ action: "createAssignment", studentId: DEMO_STUDENT_ID, title: "Original study", description: "Explain the idea", kind: "study", status: "assigned", resourceIds: [resource.id] });
    result = await action({ action: "reviseResource", id: resource.id, version: resource.version }); resource = result.resources[0];
    result = await action({ action: "editResource", id: resource.id, version: resource.version, title: "Revised notes", kind: "study", content: "Check threats before forcing moves" }); resource = result.resources[0];
    await action({ action: "publishResource", id: resource.id, version: resource.version });
    loginDemo("student"); result = await workspace();
    expect(result.assignments[0].resources[0].content).toBe("Check forcing moves");
  });
});

describe("games, communication, and report demo lifecycle", () => {
  it.each(["ended", "future"] as const)("excludes %s enrollments from batch messages and notifications until membership is current", async (membership) => {
    loginDemo("coach"); await workspace();
    const state = readDemo();
    const at = Date.now();
    const enrolment = state.academy!.enrolments[0];
    enrolment.startsAt = new Date(at + (membership === "future" ? 86400000 : -172800000)).toISOString();
    enrolment.endsAt = new Date(at + (membership === "future" ? 172800000 : -86400000)).toISOString();
    writeDemo(state);
    await action({ action: "createThread", batchId: enrolment.batchId, title: "Current batch announcement", body: "Bring one game to the next lesson." });
    const saved = readDemo();
    const thread = saved.learning!.threads[0];
    expect(saved.learning!.notifications).toEqual([]);
    for (const role of ["guardian", "student"] as const) {
      loginDemo(role);
      expect((await workspace()).threads).toEqual([]);
      await action({ action: "readThread", id: thread.id }, 403);
    }
    const current = readDemo();
    current.academy!.enrolments[0].startsAt = new Date(at - 86400000).toISOString();
    current.academy!.enrolments[0].endsAt = new Date(at + 86400000).toISOString();
    writeDemo(current);
    loginDemo("coach");
    await action({ action: "sendMessage", id: thread.id, body: "Your enrollment is current; see you in class." });
    expect(readDemo().learning!.notifications.map((item) => item.recipientId).sort()).toEqual([DEMO_STUDENT_ID, "demo-guardian"].sort());
    for (const role of ["guardian", "student"] as const) {
      loginDemo(role);
      const view = await workspace();
      expect(view.threads[0].id).toBe(thread.id);
      expect(view.threads[0].canSend).toBe(false);
      expect(view.notifications).toHaveLength(1);
    }
  });
  it("does not notify a former batch coach about a family's individual conversation", async () => {
    loginDemo("guardian"); await workspace();
    const state = readDemo();
    state.academy!.enrolments[0].endsAt = new Date(Date.now() - 86400000).toISOString();
    writeDemo(state);
    await action({ action: "createThread", studentId: DEMO_STUDENT_ID, title: "Family question", body: "Please help us choose our next batch." });
    expect(readDemo().learning!.notifications.some((item) => item.recipientId === "demo-coach")).toBe(false);
  });
  it("keeps private coach reviews private until they are shared", async () => {
    loginDemo("student"); const result = await action({ action: "saveGame", studentId: DEMO_STUDENT_ID, title: "My game", source: "imported", pgn: "1. e4 e5 *", shared: false }); const game = result.games[0];
    loginDemo("coach"); await action({ action: "reviewGame", id: game.id, version: game.version, feedback: "Private coach review", shared: false });
    loginDemo("guardian"); expect((await workspace()).games).toEqual([]);
    loginDemo("student"); expect((await workspace()).games[0].review).toBe("");
    loginDemo("coach"); await action({ action: "reviewGame", id: game.id, version: 2, feedback: "Shared review", shared: true }); loginDemo("guardian"); expect((await workspace()).games[0].review).toBe("Shared review");
  });
  it("shows family threads and personal unread state with batch reply restrictions", async () => {
    loginDemo("guardian"); let result = await action({ action: "createThread", studentId: DEMO_STUDENT_ID, title: "Practice question", body: "Which position should we repeat?" }); const thread = result.threads[0];
    loginDemo("coach"); result = await workspace(); expect(result.threads[0].unread).toBe(1);
    await action({ action: "sendMessage", id: thread.id, body: "Repeat the knight fork exercise." });
    loginDemo("guardian"); result = await workspace(); expect(result.threads[0].unread).toBe(1); expect(result.notifications.length).toBeGreaterThan(0);
    await action({ action: "readThread", id: thread.id }); expect((await workspace()).threads[0].unread).toBe(0);
    loginDemo("coach"); result = await action({ action: "createThread", batchId: "demo-batch-foundations", title: "Group lesson", body: "Bring one game for review." }); const batchThread = result.threads[0];
    loginDemo("student"); expect((await workspace()).threads.find((item) => item.id === batchThread.id)?.canSend).toBe(false); await action({ action: "sendMessage", id: batchThread.id, body: "Not permitted" }, 403);
  });
  it("requires academic publication, preserves correction history, and issues/revokes certificates", async () => {
    loginDemo("coach"); const initial = await workspace();
    let result = await action({ action: "createAssessment", studentId: DEMO_STUDENT_ID, rubricId: initial.rubrics[0].id, title: "Monthly update", period: "2026-09", assessmentContent: assessmentContent() }); let report = result.assessments[0];
    result = await action({ action: "submitAssessment", id: report.id, version: report.version }); report = result.assessments[0];
    await action({ action: "publishAssessment", id: report.id, version: report.version }, 403);
    loginDemo("guardian"); expect((await workspace()).assessments.some((item) => item.id === report.id)).toBe(false);
    loginDemo("academic_manager"); result = await action({ action: "publishAssessment", id: report.id, version: report.version }); report = result.assessments.find((item) => item.id === report.id)!;
    const nextLevel = academyLevels.find((level) => level.slug !== result.students.find((student) => student.id === DEMO_STUDENT_ID)?.levelSlug)!;
    result = await action({ action: "placeStudent", studentId: DEMO_STUDENT_ID, assessmentId: report.id, levelSlug: nextLevel.slug, feedback: "Published evidence supports this next learning level." });
    expect(result.placements[0]).toMatchObject({ studentId: DEMO_STUDENT_ID, assessmentId: report.id, levelSlug: nextLevel.slug });
    expect(result.students.find((student) => student.id === DEMO_STUDENT_ID)?.levelSlug).toBe(nextLevel.slug);
    result = await action({ action: "issueCertificate", id: report.id, version: report.version }); const certificate = result.certificates[0];
    loginDemo("guardian"); result = await action({ action: "acknowledgeAssessment", id: report.id, version: report.version }); expect(result.assessments.find((item) => item.id === report.id)?.acknowledgedAt).toBeTruthy();
    loginDemo("coach"); result = await action({ action: "correctAssessment", id: report.id, version: report.version, rubricId: report.rubricId, title: "Corrected monthly update", period: report.period, assessmentContent: assessmentContent() }); expect(result.assessments[0].sourceVersionId).toBe(report.id); expect(result.assessments.find((item) => item.id === report.id)?.status).toBe("published");
    loginDemo("academic_manager"); result = await action({ action: "revokeCertificate", id: certificate.id, feedback: "Issued for the wrong level" }); expect(result.certificates[0].status).toBe("revoked");
  });
});

async function upload(fields: Record<string, string>, content = "1. e4 e5 *", filename = "lesson.pgn") {
  // Explicit multipart exercises the same Request.formData parser as the UI's FormData.
  const boundary = "kg-demo-test-file";
  const values = Object.entries(fields).map(([key, value]) => `--${boundary}\r\nContent-Disposition: form-data; name="${key}"\r\n\r\n${value}\r\n`).join("");
  const body = `${values}--${boundary}\r\nContent-Disposition: form-data; name="file"; filename="${filename}"\r\nContent-Type: application/octet-stream\r\n\r\n${content}\r\n--${boundary}--\r\n`;
  return demoFetch("http://localhost/api/v1/learning/files", { method: "POST", headers: { "Content-Type": `multipart/form-data; boundary=${boundary}` }, body });
}

describe("local demo learning files", () => {
  it("uploads a personal resource, restricts download, and grants access through an assignment snapshot", async () => {
    loginDemo("coach");
    const response = await upload({ title: "Opening game", visibility: "personal" });
    const uploaded = await response.json(); expect(response.status, JSON.stringify(uploaded)).toBe(201);
    const file = uploaded.file as { id: string; filename: string };
    const resource = (uploaded.workspace as LearningWorkspace).resources[0];
    const downloaded = await request(`/learning/files/${file.id}/download`); expect(downloaded.status).toBe(200); expect(await downloaded.text()).toBe("1. e4 e5 *");
    loginDemo("student"); expect((await request(`/learning/files/${file.id}/download`)).status).toBe(403);
    loginDemo("coach"); await action({ action: "publishResource", id: resource.id, version: resource.version });
    await action({ action: "createAssignment", studentId: DEMO_STUDENT_ID, title: "Review this opening", description: "Explain the center", kind: "game_review", resourceIds: [resource.id], status: "assigned" });
    loginDemo("student"); expect((await request(`/learning/files/${file.id}/download`)).status).toBe(200);
  });
  it("lets a learner attach evidence before submission and rejects staff impersonation", async () => {
    loginDemo("coach"); const created = await action({ action: "createAssignment", studentId: DEMO_STUDENT_ID, title: "Upload practice", description: "Submit your game", kind: "file_upload", status: "assigned" }); const assignment = created.assignments[0];
    expect((await upload({ assignmentId: assignment.id })).status).toBe(403);
    loginDemo("student"); await action({ action: "submitAssignment", id: assignment.id, version: assignment.version }, 422);
    const response = await upload({ assignmentId: assignment.id }); const uploaded = await response.json(); expect(response.status, JSON.stringify(uploaded)).toBe(201);
    const homework = (uploaded.workspace as LearningWorkspace).assignments[0]; expect(homework.attempts[0].files).toHaveLength(1);
    await action({ action: "submitAssignment", id: homework.id, version: homework.version }); expect((await upload({ assignmentId: assignment.id })).status).toBe(409);
  });
  it("rejects malformed chess data and active PDF content without a new resource", async () => {
    loginDemo("coach"); await workspace(); const before = readDemo();
    expect((await upload({ title: "Bad PGN" }, "1. e5")).status).toBe(422);
    expect((await upload({ title: "Active PDF" }, "%PDF-1.4\n/JavaScript (bad)\n%%EOF", "unsafe.pdf")).status).toBe(422);
    expect(readDemo()).toEqual(before);
  });
});
