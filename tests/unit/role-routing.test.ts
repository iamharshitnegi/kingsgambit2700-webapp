import { describe, expect, it } from "vitest";
import { authorizedReturnPath, landingPathForRoles, safeReturnPath, userCanAccess } from "@/app/lib/role-routing";
import type { SessionUser } from "@/app/lib/api/types";

describe("role routing", () => {
  it("uses deterministic precedence for multi-role accounts", () => {
    expect(landingPathForRoles(["student", "guardian"])).toBe("/guardian");
    expect(landingPathForRoles(["coach", "operations_admin"])).toBe("/admin");
    expect(landingPathForRoles(["guardian", "super_admin"])).toBe("/admin");
  });

  it("does not accept cross-origin return paths", () => {
    expect(safeReturnPath("https://example.com/steal")).toBe("/portal");
    expect(safeReturnPath("//example.com/steal")).toBe("/portal");
    expect(safeReturnPath("/\\example.com/steal")).toBe("/portal");
    expect(safeReturnPath("/portal\nhttps://example.com/steal")).toBe("/portal");
    expect(safeReturnPath("/guardian?tab=schedule")).toBe("/guardian?tab=schedule");
  });

  it("checks allowed roles without treating the browser as final authorization", () => {
    const user: SessionUser = { id: "user-1", roles: ["student"] };
    expect(userCanAccess(user, ["student"])).toBe(true);
    expect(userCanAccess(user, ["guardian"])).toBe(false);
  });
});

describe("authorized sign-in destinations", () => {
  it.each([
    ["guardian", "/admin/settings", "/guardian"],
    ["student", "/admin", "/student"],
    ["coach", "/admin?tab=finance", "/coach"],
    ["operations_admin", "/guardian", "/admin"],
    ["finance_admin", "/coach", "/admin"],
    ["academic_manager", "/student", "/admin"],
    ["super_admin", "/guardian", "/admin"],
    ["guardian", "/guardian/students?tab=consent", "/guardian/students?tab=consent"],
    ["guardian", "/%61dmin/settings", "/guardian"],
    ["guardian", "/student/../admin", "/guardian"],
    ["guardian", "/forbidden", "/guardian"],
    ["student", "/classroom/lesson-1", "/classroom/lesson-1"],
    ["student", "//outside.example", "/student"],
    ["student", "/%2f%2foutside.example", "/student"],
  ] as const)("routes %s from %s to %s", (role, requested, expected) => {
    expect(authorizedReturnPath({ id: "account", roles: [role] }, requested)).toBe(expected);
  });
  it("retains an authorized secondary-role destination", () => {
    expect(authorizedReturnPath({ id: "account", roles: ["coach", "guardian"] }, "/guardian/students")).toBe("/guardian/students");
  });
  it("does not send accounts without roles into a workspace", () => {
    expect(authorizedReturnPath({ id: "account", roles: [] }, "/admin")).toBe("/forbidden");
  });
});
