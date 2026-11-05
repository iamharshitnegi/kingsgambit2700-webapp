import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import CalendarView from "@/app/components/portal/workspace/CalendarView";
import type { AcademyWorkspaceData } from "@/app/lib/academy-types";
vi.mock("next/navigation", () => ({ usePathname: () => "/student/calendar" }));
beforeEach(() => {
  vi.stubGlobal(
    "matchMedia",
    vi.fn(() => ({
      matches: false,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })),
  );
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function workspace(): AcademyWorkspaceData {
  return {
    pagination: { limit: 500, hasMore: false, nextLimit: 1000 },
    capabilities: {
      manageAcademy: false,
      manageScheduling: false,
      manageTeaching: false,
      manageStaff: false,
      managePolicies: false,
    },
    students: [
      {
        id: "learner",
        name: "Arun",
        userId: "user",
        familyId: "family",
        status: "active",
        ageBand: "8_12",
        timeZone: "UTC",
        country: "IN",
        consent: true,
      },
    ],
    coaches: [{ id: "coach", name: "Coach Maya" }],
    batches: [],
    enrolments: [],
    classes: [
      {
        id: "lesson",
        batchId: "batch",
        coachId: "coach",
        coach: "Coach Maya",
        title: "Planning ahead",
        format: "individual",
        studentIds: ["learner"],
        startsAt: new Date(Date.now() + 3600000).toISOString(),
        endsAt: new Date(Date.now() + 7200000).toISOString(),
        timeZone: "UTC",
        status: "scheduled",
        revision: 0,
        notes: "",
        meetingUrl: "https://meet.google.com/abc-defg-hij",
        attendance: {},
      },
    ],
    makeUps: [],
    trialSlots: [],
    policies: [],
    settings: {
      name: "Academy",
      legalName: "Academy",
      email: "academy@example.test",
    },
    profile: { displayName: "Arun", timeZone: "UTC", savedViews: [] },
    availability: [],
    workload: {
      months: [],
      scheduledClasses: 1,
      pendingCompletion: 0,
      completedClasses: 0,
      completedMinutes: 0,
    },
  };
}
function show(data = workspace(), compact = false) {
  return render(
    <CalendarView
      data={data}
      page="classes"
      compact={compact}
      timeZone="UTC"
      setTimeZone={vi.fn()}
      canJoin
      canCancel={false}
      onCreate={vi.fn()}
      onEdit={vi.fn()}
      onCancel={vi.fn()}
      onComplete={vi.fn()}
    />,
  );
}
it("shows a future meeting link while classroom access stays closed for a learner", () => {
  show();
  expect(screen.getByRole("link", { name: "Open meeting" })).toHaveAttribute(
    "href",
    "https://meet.google.com/abc-defg-hij",
  );
  expect(screen.getByRole("button", { name: "Join classroom" })).toBeDisabled();
  expect(screen.queryByRole("button", { name: "Edit class" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Schedule class" })).toBeNull();
  expect(
    screen.queryByRole("button", { name: "Cancellation options" }),
  ).toBeNull();
});
it("removes stale class details when search excludes the selected class", async () => {
  show();
  await userEvent.type(
    screen.getByRole("searchbox", { name: "Search classes" }),
    "unmatched",
  );
  expect(screen.queryByRole("link", { name: "Open meeting" })).toBeNull();
  expect(screen.getByText("No classes match these filters.")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "Clear filters" }));
  expect(screen.getByRole("link", { name: "Open meeting" })).toBeVisible();
});
it("explains a missing or unsafe meeting link without offering learner edits", () => {
  const data = workspace();
  data.classes[0].meetingUrl = "javascript:alert(1)";
  show(data);
  expect(
    screen.getByText("The coach has not added a meeting link yet."),
  ).toBeVisible();
  expect(screen.queryByRole("link", { name: "Open meeting" })).toBeNull();
  expect(screen.queryByRole("button", { name: "Add meeting link" })).toBeNull();
});
it("keeps a past-only home schedule useful with an empty state", () => {
  const data = workspace();
  data.classes[0].status = "completed";
  show(data, true);
  expect(
    screen.getByText("No classes scheduled in this view yet."),
  ).toBeVisible();
  expect(
    screen.queryByRole("button", { name: "View Planning ahead" }),
  ).toBeNull();
});
