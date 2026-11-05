import type { AcademyRole, SessionUser } from "./api/types";

const adminRoles: AcademyRole[] = [
  "academic_manager",
  "operations_admin",
  "finance_admin",
  "super_admin",
];

export const roleLabels: Record<AcademyRole, string> = {
  guardian: "Guardian",
  student: "Student",
  coach: "Coach",
  academic_manager: "Academic manager",
  operations_admin: "Operations admin",
  finance_admin: "Finance admin",
  super_admin: "Super admin",
};

export function landingPathForRole(role: AcademyRole) {
  if (adminRoles.includes(role)) return "/admin";
  if (role === "coach") return "/coach";
  if (role === "guardian") return "/guardian";
  if (role === "student") return "/student";
  return "/forbidden";
}

export function landingPathForRoles(roles: AcademyRole[]) {
  if (roles.includes("super_admin")) return "/admin";
  if (roles.some((role) => adminRoles.includes(role))) return "/admin";
  if (roles.includes("coach")) return "/coach";
  if (roles.includes("guardian")) return "/guardian";
  if (roles.includes("student")) return "/student";
  return "/forbidden";
}

export function userCanAccess(user: SessionUser, allowedRoles: AcademyRole[]) {
  return user.roles.some((role) => allowedRoles.includes(role));
}

export function workspaceHome(user: SessionUser, pathname?: string) {
  const allowed = new Set<string>(user.roles.map(landingPathForRole));
  const current = pathname ? `/${pathname.split("/")[1]}` : "";
  if (allowed.has(current)) return current;
  if (typeof window !== "undefined") {
    try {
      const saved = sessionStorage.getItem("kg2700.workspace");
      if (saved && allowed.has(saved)) return saved;
    } catch { /* Navigation also works when browser storage is disabled. */ }
  }
  return landingPathForRoles(user.roles);
}

export function safeReturnPath(value: string | null | undefined) {
  if (
    !value
    || !value.startsWith("/")
    || value.startsWith("//")
    || value.includes("\\")
    || /[\u0000-\u001f\u007f]/.test(value)
    || /^\/(sign-in|auth)(\/|\?|$)/.test(value)
  ) return "/portal";
  return value;
}

/** Keep post-login destinations only when they belong to an accessible workspace. */
export function authorizedReturnPath(user: SessionUser, value?: string | null) {
  const requested = safeReturnPath(value);
  const fallback = workspaceHome(user);
  let pathname: string;
  try {
    pathname = decodeURIComponent(new URL(requested, "https://portal.local").pathname);
  } catch {
    return fallback;
  }
  if (safeReturnPath(pathname) === "/portal" || pathname === "/" || pathname === "/forbidden") return fallback;
  const workspace = `/${pathname.split("/")[1]}`;
  if (["/admin", "/coach", "/guardian", "/student"].includes(workspace)
    && !user.roles.some((role) => landingPathForRole(role) === workspace)) return fallback;
  return requested;
}

export const ADMIN_ROLES = adminRoles;
