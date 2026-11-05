import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  demoAcademyRequest,
  demoAcademyWorkspace,
} from "@/app/lib/demo/academy";
import { createDemoWorkspace, DEMO_STUDENT_ID } from "@/app/lib/demo/data";
import { getDemoUser, loginDemo, readDemo } from "@/app/lib/demo/store";
import type { AcademyRole } from "@/app/lib/api/types";
vi.mock("@/app/lib/demo/mode", () => ({ isDemoMode: () => true }));
const request = new Request("http://local-demo.invalid/api/v1/academy/classes");
beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});
describe("shared demo academy contracts", () => {
  it("keeps finance schedules private and shows coaches only their assigned classes", () => {
    loginDemo("coach");
    const state = createDemoWorkspace();
    const workspace = demoAcademyWorkspace(state);
    state.academy!.classes.push({ ...workspace.classes[0], id: "other-coach-class", coachId: "another-coach" });
    expect(demoAcademyWorkspace(state).classes.map((lesson) => lesson.id)).not.toContain("other-coach-class");
    loginDemo("finance_admin");
    const finance = demoAcademyWorkspace(state);
    expect(finance.students).toHaveLength(1);
    expect(finance.enrolments).toHaveLength(1);
    expect(finance.classes).toEqual([]);
    expect(finance.workload.completedClasses).toBe(0);
  });
  it.each<AcademyRole>(["guardian", "student"])("redacts unrelated classmates and coach discovery for %s", (role) => {
    loginDemo(role);
    const state = createDemoWorkspace();
    demoAcademyWorkspace(state);
    const data = state.academy!;
    data.students.push({ ...data.students[0], id: "unrelated-learner", userId: "unrelated-user", familyId: "unrelated-family", name: "Another learner" });
    data.guardianEmails["unrelated-learner"] = "other-family@example.test";
    data.coaches.push({ id: "unrelated-coach", name: "Another coach" });
    data.classes[0].studentIds.push("unrelated-learner");
    data.classes[0].attendance = { [DEMO_STUDENT_ID]: "on_time", "unrelated-learner": "unexcused" };
    data.batches[0].studentIds.push("unrelated-learner");
    const view = demoAcademyWorkspace(state);
    expect(view.students.map((student) => student.id)).toEqual([DEMO_STUDENT_ID]);
    expect(view.classes[0].studentIds).toEqual([DEMO_STUDENT_ID]);
    expect(view.classes[0].attendance).toEqual({ [DEMO_STUDENT_ID]: "on_time" });
    expect(view.batches[0].studentIds).toEqual([DEMO_STUDENT_ID]);
    expect(view.coaches.map((coach) => coach.id)).toEqual(["demo-coach"]);
    expect(data.classes[0].studentIds).toContain("unrelated-learner");
    expect(data.classes[0].attendance["unrelated-learner"]).toBe("unexcused");
  });
  it("revokes a suspended demo staff account and preserves roles when editing", async () => {
    loginDemo("super_admin");
    const state = createDemoWorkspace();
    const directory = await demoAcademyRequest("/academy/staff", "GET", {}, state, request)!.json();
    const coach = directory.staff.find((person: { id: string }) => person.id === "demo-coach");
    const updated = await demoAcademyRequest("/academy/staff/demo-coach", "PATCH", { displayName: "Coach Maya Updated", status: "suspended", expectedUpdatedAt: coach.updatedAt }, readDemo(), request)!.json();
    expect(updated.roles).toEqual(["coach"]);
    expect(() => loginDemo("coach")).toThrow(/suspended/);
    expect(getDemoUser()).toBeUndefined();
  });
  it("links an academy cancellation to a replacement without granting another credit", () => {
    loginDemo("coach");
    const state = createDemoWorkspace();
    const workspace = demoAcademyWorkspace(state);
    const lesson = workspace.classes.find((item) => item.id === "demo-class-next")!;
    demoAcademyRequest(`/academy/classes/${lesson.id}/cancel`, "POST", { revision: lesson.revision, reason: "Coach is unavailable", idempotencyKey: "makeup-cancel" }, state, request);
    const obligation = state.academy!.makeUps[0];
    expect(obligation.status).toBe("open");
    demoAcademyRequest("/academy/classes", "POST", { batchId: workspace.batches[0].id, title: "Replacement lesson", localStart: "2027-04-01T10:00", timeZone: "Asia/Kolkata", durationMinutes: 60, occurrences: 1, idempotencyKey: "replacement" }, state, request);
    const replacement = state.academy!.classes.find((item) => item.title === "Replacement lesson")!;
    const available = state.academy!.enrolments[0].available;
    const body = { replacementClassId: replacement.id, idempotencyKey: "fulfill" };
    demoAcademyRequest(`/academy/make-ups/${obligation.id}/fulfill`, "POST", body, state, request);
    demoAcademyRequest(`/academy/make-ups/${obligation.id}/fulfill`, "POST", body, state, request);
    expect(obligation.status).toBe("fulfilled");
    expect(state.academy!.enrolments[0].available).toBe(available);
  });
  it("schedules idempotently and rolls back a whole series with insufficient credits", () => {
    loginDemo("coach");
    const state = createDemoWorkspace();
    const workspace = demoAcademyWorkspace(state);
    const input = {
      batchId: workspace.batches[0].id,
      title: "Practice forks",
      localStart: "2027-04-01T10:00",
      timeZone: "Asia/Kolkata",
      durationMinutes: 60,
      occurrences: 2,
      recurringUntil: "2027-04-08",
      meetingUrl: "",
      idempotencyKey: "schedule-1",
    };
    demoAcademyRequest("/academy/classes", "POST", input, state, request);
    expect(state.academy?.classes).toHaveLength(5);
    expect(state.academy?.enrolments[0].available).toBe(6);
    demoAcademyRequest("/academy/classes", "POST", input, state, request);
    expect(state.academy?.classes).toHaveLength(5);
    expect(() =>
      demoAcademyRequest(
        "/academy/classes",
        "POST",
        {
          ...input,
          localStart: "2027-06-01T10:00",
          recurringUntil: "2027-08-01",
          occurrences: 7,
          idempotencyKey: "schedule-2",
        },
        state,
        request,
      ),
    ).toThrow(/credits/);
    expect(state.academy?.classes).toHaveLength(5);
    expect(state.academy?.enrolments[0].available).toBe(6);
  });
  it.each<AcademyRole>([
    "guardian",
    "student",
    "academic_manager",
    "finance_admin",
  ])("prevents %s from scheduling", (role) => {
    loginDemo(role);
    const state = createDemoWorkspace();
    const workspace = demoAcademyWorkspace(state);
    expect(() =>
      demoAcademyRequest(
        "/academy/classes/preview",
        "POST",
        { batchId: workspace.batches[0].id },
        state,
        request,
      ),
    ).toThrow(/assigned coach/);
  });
  it("applies group absence to one learner without restoring their credit or cancelling peers", async () => {
    loginDemo("guardian");
    const state = createDemoWorkspace();
    const workspace = demoAcademyWorkspace(state);
    const lesson = workspace.classes.find(
      (item) => item.id === "demo-class-next",
    )!;
    const response = demoAcademyRequest(
      `/academy/classes/${lesson.id}/cancel`,
      "POST",
      {
        revision: lesson.revision,
        studentId: DEMO_STUDENT_ID,
        reason: "Family unavailable",
        idempotencyKey: "cancel-1",
      },
      state,
      request,
    )!;
    const result = await response.json();
    expect(result.outcomes[0]).toMatchObject({
      action: "consume",
      makeUp: false,
      reason: "group_family_absence",
    });
    expect(
      state.academy?.classes.find((item) => item.id === lesson.id)?.status,
    ).toBe("scheduled");
    expect(state.academy?.enrolments[0].available).toBe(8);
  });
});
