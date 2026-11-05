import { describe, expect, it } from "vitest";
import {
  addDays,
  calendarDates,
  canJoinClass,
  dayInZone,
  daySegments,
  eventLanes,
  safeMeetingUrl,
} from "@/app/lib/calendar";
import type { AcademyClass } from "@/app/lib/academy-types";
const lesson: AcademyClass = {
  id: "lesson",
  batchId: "batch",
  coachId: "coach",
  coach: "Maya",
  title: "Candidate moves",
  format: "group",
  studentIds: ["learner"],
  startsAt: "2026-09-23T12:00:00Z",
  endsAt: "2026-09-23T13:00:00Z",
  timeZone: "UTC",
  status: "scheduled",
  revision: 0,
  notes: "",
  meetingUrl: "",
  attendance: {},
};

describe("calendar periods and placement", () => {
  it("uses Monday weeks across month/year boundaries without local DST arithmetic", () => {
    expect(calendarDates("2027-01-01", "week")).toEqual([
      "2026-12-28",
      "2026-12-29",
      "2026-12-30",
      "2026-12-31",
      "2027-01-01",
      "2027-01-02",
      "2027-01-03",
    ]);
    expect(calendarDates("2026-03-08", "agenda")).toHaveLength(7);
    expect(addDays("2026-03-08", 1)).toBe("2026-03-09");
    expect(calendarDates("2026-02-28", "month")).toHaveLength(42);
  });
  it("splits classes crossing local midnight without adding a zero-length next day", () => {
    const item = {
      ...lesson,
      startsAt: "2026-09-23T18:00:00Z",
      endsAt: "2026-09-23T19:00:00Z",
    };
    expect(daySegments([item], "2026-09-23", "Asia/Kolkata")[0]).toMatchObject({
      start: 1410,
      end: 1440,
    });
    expect(daySegments([item], "2026-09-24", "Asia/Kolkata")[0]).toMatchObject({
      start: 0,
      end: 30,
    });
    expect(
      daySegments(
        [{ ...item, endsAt: "2026-09-23T18:30:00Z" }],
        "2026-09-24",
        "Asia/Kolkata",
      ),
    ).toEqual([]);
  });
  it("displays saved instants in the selected timezone across DST", () => {
    expect(dayInZone("2026-03-08T04:30:00Z", "America/New_York")).toBe(
      "2026-03-07",
    );
    const spring = {
      ...lesson,
      startsAt: "2026-03-08T06:30:00Z",
      endsAt: "2026-03-08T07:30:00Z",
    };
    expect(
      daySegments([spring], "2026-03-08", "America/New_York")[0],
    ).toMatchObject({ start: 90, end: 210 });
    const autumn = {
      ...lesson,
      startsAt: "2026-11-01T05:30:00Z",
      endsAt: "2026-11-01T06:30:00Z",
    };
    const [segment] = daySegments([autumn], "2026-11-01", "America/New_York");
    expect(segment.end).toBeGreaterThan(segment.start);
  });
  it("gives overlaps distinct columns and reuses space for adjacent classes", () => {
    const result = eventLanes([
      { lesson, start: 600, end: 660 },
      { lesson: { ...lesson, id: "second" }, start: 630, end: 690 },
      { lesson: { ...lesson, id: "third" }, start: 690, end: 720 },
    ]);
    expect(result.map(({ lane, lanes }) => [lane, lanes])).toEqual([
      [0, 2],
      [1, 2],
      [0, 1],
    ]);
  });
});
describe("classroom access and meeting links", () => {
  it("keeps finite classroom access and excludes cancelled/completed classes", () => {
    expect(canJoinClass(lesson, Date.parse(lesson.startsAt) - 900001)).toBe(
      false,
    );
    expect(canJoinClass(lesson, Date.parse(lesson.startsAt) - 900000)).toBe(
      true,
    );
    expect(canJoinClass(lesson, Date.parse(lesson.endsAt) + 900000)).toBe(
      false,
    );
    for (const status of [
      "completed",
      "cancelled_family",
      "cancelled_academy",
      "rescheduled",
    ])
      expect(
        canJoinClass({ ...lesson, status }, Date.parse(lesson.startsAt)),
      ).toBe(false);
  });
  it("rejects executable URLs while accepting saved Zoom/Meet links", () => {
    expect(safeMeetingUrl("javascript:alert(1)")).toBeNull();
    expect(safeMeetingUrl("data:text/html,<h1>test</h1>")).toBeNull();
    expect(safeMeetingUrl("")).toBeNull();
    expect(safeMeetingUrl("https://meet.google.com/abc-defg-hij")).toBe(
      "https://meet.google.com/abc-defg-hij",
    );
  });
});
