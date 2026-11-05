import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import PortalShell from "@/app/components/portal/PortalShell";
import PortalGate from "@/app/components/portal/PortalGate";
import PublicNavbar from "@/app/components/public/PublicNavbar";
import SignInForm from "@/app/components/forms/SignInForm";
import { fallbackOrganization } from "@/app/lib/fallback-content";
import type { AcademyRole, SessionResponse } from "@/app/lib/api/types";

const mocks = vi.hoisted(() => ({
  replace: vi.fn(), logout: vi.fn(), pathname: "/guardian", search: "", session: null as SessionResponse | null, error: null as string | null,
}));
vi.mock("next/navigation", () => ({ usePathname: () => mocks.pathname, useRouter: () => ({ replace: mocks.replace }), useSearchParams: () => new URLSearchParams(mocks.search) }));
vi.mock("@/app/lib/auth-client", () => ({ logout: mocks.logout, registerWithPassword: vi.fn(), signInWithPassword: vi.fn() }));
vi.mock("@/app/lib/use-session", () => ({ useSession: () => ({ session: mocks.session, error: mocks.error }) }));
vi.mock("@/app/lib/demo/mode", () => ({ isDemoMode: () => false }));
vi.mock("@/app/components/forms/GoogleSignIn", () => ({ default: () => null }));
vi.mock("@/app/components/forms/MagicLinkForm", () => ({ default: () => null }));

const roles: [AcademyRole, string][] = [["guardian", "/guardian"], ["student", "/student"], ["coach", "/coach"],
  ["academic_manager", "/admin"], ["operations_admin", "/admin"], ["finance_admin", "/admin"], ["super_admin", "/admin"]];

beforeEach(() => {
  vi.clearAllMocks(); sessionStorage.clear(); mocks.session = null; mocks.error = null; mocks.search = "";
  vi.stubGlobal("matchMedia", vi.fn(() => ({ matches: false, addEventListener: vi.fn(), removeEventListener: vi.fn() })));
});
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });

function signedIn(role: AcademyRole) { return { authenticated: true, csrfToken: "csrf", user: { id: "user", displayName: "Academy user", roles: [role] } } satisfies SessionResponse; }

describe("authenticated academy navigation", () => {
  it.each(roles)("keeps %s home, logos and academy links in %s", async (role, path) => {
    mocks.pathname = path; mocks.session = signedIn(role);
    render(<PortalShell user={mocks.session.user!} title="Workspace" eyebrow="Academy"><p>Workspace contents</p></PortalShell>);
    for (const logo of screen.getAllByRole("link", { name: "KingsGambit2700 home" })) expect(logo).toHaveAttribute("href", path);
    expect(screen.getByRole("link", { name: "Back to academy" })).toHaveAttribute("href", path);
    expect(screen.getByRole("link", { name: "Visit website" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "Home" })).toHaveAttribute("href", path);
    expect(mocks.logout).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "Sign out" }));
    await waitFor(() => expect(mocks.logout).toHaveBeenCalledOnce());
    expect(mocks.replace).toHaveBeenCalledWith("/sign-in");
  });

  it.each(roles)("public navigation restores %s access", (role, path) => {
    mocks.pathname = "/programs"; mocks.session = signedIn(role);
    render(<PublicNavbar organization={fallbackOrganization} />);
    for (const link of screen.getAllByRole("link", { name: /My portal/ })) expect(link).toHaveAttribute("href", path);
    expect(screen.queryByRole("link", { name: /^Sign in/ })).not.toBeInTheDocument();
  });

  it("retains the authorized workspace when leaving a classroom", () => {
    mocks.pathname = "/classroom/class-one"; sessionStorage.setItem("kg2700.workspace", "/coach");
    render(<PortalShell user={{ id: "staff", roles: ["super_admin", "coach"] }} title="Class" eyebrow="Academy"><p>Board</p></PortalShell>);
    expect(screen.getByRole("link", { name: "Back to academy" })).toHaveAttribute("href", "/coach");
  });

  it("shows retry state without redirecting on a bootstrap outage", () => {
    mocks.pathname = "/coach"; mocks.error = "Session service unavailable";
    render(<PortalGate allowedRoles={["coach"]}>{() => <p>Private data</p>}</PortalGate>);
    expect(screen.getByRole("alert")).toHaveTextContent("Session service unavailable");
    expect(mocks.replace).not.toHaveBeenCalled();
    expect(screen.queryByText("Private data")).not.toBeInTheDocument();
  });

  it("keeps an already authenticated workspace visible during a transient outage", () => {
    mocks.pathname = "/coach"; mocks.session = signedIn("coach"); mocks.error = "Temporary connection error";
    render(<PortalGate allowedRoles={["coach"]}>{() => <p>Private data</p>}</PortalGate>);
    expect(screen.getByText("Private data")).toBeVisible(); expect(mocks.replace).not.toHaveBeenCalled();
  });

  it("routes incomplete staff authentication through MFA", async () => {
    mocks.pathname = "/coach"; mocks.session = { authenticated: false, mfaRequired: true };
    render(<PortalGate allowedRoles={["coach"]}>{() => <p>Private data</p>}</PortalGate>);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/auth/complete?mfa=required&next=%2Fcoach"));
  });

  it("redirects an authenticated visitor away from sign-in", async () => {
    mocks.pathname = "/sign-in"; mocks.session = signedIn("coach");
    render(<SignInForm />);
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/coach"));
    expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
  });
});
