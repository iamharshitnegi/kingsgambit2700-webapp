import { describe, expect, it } from "vitest";
import { localTeachingInstant } from "@/app/lib/scheduling-time";
describe("teaching wall-clock times", () => {
  it("preserves the local teaching hour through a daylight-saving change", () => {
    expect(localTeachingInstant("2026-03-01T10:00", "America/New_York")).toBe(
      "2026-03-01T15:00:00.000Z",
    );
    expect(localTeachingInstant("2026-03-15T10:00", "America/New_York")).toBe(
      "2026-03-15T14:00:00.000Z",
    );
  });
  it("rejects nonexistent and ambiguous local teaching times", () => {
    expect(() =>
      localTeachingInstant("2026-03-08T02:30", "America/New_York"),
    ).toThrow(/does not exist/);
    expect(() =>
      localTeachingInstant("2026-11-01T01:30", "America/New_York"),
    ).toThrow(/twice/);
    expect(localTeachingInstant("2026-11-01T09:00", "Asia/Kolkata")).toBe(
      "2026-11-01T03:30:00.000Z",
    );
  });
});
