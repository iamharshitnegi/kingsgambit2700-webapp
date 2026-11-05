import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import LearnersPanel from "@/app/components/portal/workspace/LearnersPanel";
import type { AcademyWorkspaceData } from "@/app/lib/academy-types";

beforeEach(() => {
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute("open", ""); };
  HTMLDialogElement.prototype.close = function () { this.removeAttribute("open"); };
});
afterEach(cleanup);

const data: AcademyWorkspaceData = {
  pagination: { limit: 500, hasMore: false, nextLimit: 1000 },
  capabilities: { manageAcademy: true, manageScheduling: true, manageTeaching: false, manageStaff: false, managePolicies: false },
  students: [],
  coaches: [{ id: "coach-1", name: "Coach Maya" }],
  batches: [{ id: "legacy-batch", name: "Original batch", coachId: "coach-1", format: "group", level: "beginner", timeZone: "Asia/Kolkata", capacity: 4, studentIds: [] }],
  enrolments: [],
  classes: [],
  policies: [],
  settings: { name: "Academy", legalName: "Academy", email: "academy@example.test" },
  profile: { displayName: "Operations", timeZone: "Asia/Kolkata", savedViews: [] },
  availability: [],
  makeUps: [],
  trialSlots: [],
  workload: { months: [], scheduledClasses: 0, pendingCompletion: 0, completedClasses: 0, completedMinutes: 0 },
};

describe("batch level preservation", () => {
  it("keeps a legacy level when operations edits only the batch name", async () => {
    const user = userEvent.setup();
    const mutate = vi.fn().mockResolvedValue(true);
    render(<LearnersPanel data={data} user={{ id: "operations", roles: ["operations_admin"] }} mutate={mutate} />);

    await user.click(screen.getByRole("button", { name: "Edit batch" }));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByLabelText("Curriculum level")).toHaveValue("beginner");
    await user.clear(dialog.getByLabelText("Batch name"));
    await user.type(dialog.getByLabelText("Batch name"), "Renamed legacy batch");
    await user.click(dialog.getByRole("button", { name: "Save batch" }));

    expect(mutate).toHaveBeenCalledWith("/academy/batches/legacy-batch", {
      name: "Renamed legacy batch", coachId: "coach-1", format: "group", level: "beginner", timeZone: "Asia/Kolkata", capacity: 4, status: "active",
    }, "Batch updated.", "PATCH");
  });
});

const consentPolicies = [
  { id: "privacy-v1", slug: "privacy", title: "Privacy", version: "1", content: "Current privacy policy" },
  { id: "safety-v1", slug: "child-safety", title: "Child safety", version: "1", content: "Current child-safety policy" },
];
const learner = { id: "learner-1", userId: "user-1", familyId: "family-1", name: "Maya", ageBand: "8_12", country: "IN", timeZone: "Asia/Kolkata", status: "prospect", consent: false };

describe("enrollment prerequisites", () => {
  it("explains missing academy setup before sending an enrollment request", async () => {
    const user = userEvent.setup();
    const mutate = vi.fn();
    render(<LearnersPanel data={{ ...data, students: [learner], settings: { name: "Academy", legalName: "", email: "" }, capabilities: { ...data.capabilities, managePolicies: true } }} user={{ id: "admin", roles: ["super_admin"] }} mutate={mutate} />);
    await user.click(screen.getByRole("button", { name: "Enroll learner" }));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByText("Save the academy legal name and contact email.")).toBeVisible();
    expect(dialog.getByText("Publish the privacy policy.")).toBeVisible();
    expect(dialog.getByText("Publish the child-safety policy.")).toBeVisible();
    expect(dialog.getByRole("link", { name: "Complete academy setup" })).toHaveAttribute("href", "/admin/settings");
    await user.click(dialog.getByRole("button", { name: "Save enrollment" }));
    expect(mutate).not.toHaveBeenCalled();
  });

  it("checks the selected learner's consent and permits a ready learner", async () => {
    const user = userEvent.setup();
    const mutate = vi.fn().mockResolvedValue(true);
    render(<LearnersPanel data={{ ...data, policies: consentPolicies, students: [learner, { ...learner, id: "learner-2", name: "Arun", consent: true }] }} user={{ id: "operations", roles: ["operations_admin"] }} mutate={mutate} />);
    await user.click(screen.getByRole("button", { name: "Enroll learner" }));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByText(/Maya’s guardian must sign in/)).toBeVisible();
    expect(dialog.getByRole("button", { name: "Save enrollment" })).toBeDisabled();
    await user.selectOptions(dialog.getByLabelText("Learner"), "learner-2");
    expect(dialog.getByRole("button", { name: "Save enrollment" })).toBeEnabled();
    await user.click(dialog.getByRole("button", { name: "Save enrollment" }));
    expect(mutate).toHaveBeenCalledWith("/academy/enrolments", expect.objectContaining({ studentId: "learner-2", batchId: "legacy-batch" }), "Enrollment created.", "POST");
  });

  it("does not offer guardian acceptance while academy details are missing", async () => {
    const user = userEvent.setup();
    render(<LearnersPanel data={{ ...data, policies: consentPolicies, students: [learner], settings: { name: "Academy", legalName: "", email: "" } }} user={{ id: "guardian", roles: ["guardian"] }} mutate={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "Review consent" }));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByText(/Contact academy staff to finish setup/)).toBeVisible();
    expect(dialog.getByRole("button", { name: "Save consent" })).toBeDisabled();
  });
});
