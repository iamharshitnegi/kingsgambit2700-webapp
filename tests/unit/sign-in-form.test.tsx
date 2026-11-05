import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { SessionResponse } from "@/app/lib/api/types";
import SignInForm from "@/app/components/forms/SignInForm";

const mocks = vi.hoisted(() => ({ replace: vi.fn(), password: vi.fn(), register: vi.fn(), query: "next=/guardian", session: { authenticated: false } as SessionResponse }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mocks.replace }), useSearchParams: () => new URLSearchParams(mocks.query) }));
vi.mock("@/app/lib/auth-client", () => ({ signInWithPassword: mocks.password, registerWithPassword: mocks.register }));
vi.mock("@/app/lib/use-session", () => ({ useSession: () => ({ session: mocks.session, error: null }) }));
vi.mock("@/app/lib/demo/mode", () => ({ isDemoMode: () => false }));
vi.mock("@/app/components/forms/GoogleSignIn", () => ({ default: () => null }));
vi.mock("@/app/components/forms/MagicLinkForm", () => ({ default: () => <div>Email link form</div> }));

beforeEach(() => { vi.resetAllMocks(); mocks.query = "next=/guardian"; mocks.session = { authenticated: false }; });
afterEach(cleanup);

describe("password sign-in", () => {
  it("sends credentials and routes staff through MFA", async () => {
    mocks.password.mockResolvedValue({ authenticated: false, mfaRequired: true });
    const user = userEvent.setup();
    render(<SignInForm />);
    await user.type(screen.getByLabelText("Username or email"), "coach");
    await user.type(screen.getByLabelText("Password"), "LocalChess!2700");
    await user.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/auth/complete?mfa=required&next=%2Fguardian"));
    expect(mocks.password).toHaveBeenCalledWith("coach", "LocalChess!2700");
  });

  it("shows login failure and retains the email fallback", async () => {
    mocks.password.mockRejectedValue(new Error("Check your sign-in details and try again."));
    const user = userEvent.setup();
    render(<SignInForm />);
    await user.type(screen.getByLabelText("Username or email"), "guardian");
    await user.type(screen.getByLabelText("Password"), "wrong");
    await user.click(screen.getByRole("button", { name: "Sign in" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Check your sign-in details");
    expect(mocks.replace).not.toHaveBeenCalled();
    await user.click(screen.getByRole("button", { name: "Email me a sign-in link" }));
    expect(screen.getByText("Email link form")).toBeInTheDocument();
  });

  it("requires email verification after registration", async () => {
    mocks.register.mockResolvedValue({ status: "accepted" });
    const user = userEvent.setup();
    render(<SignInForm />);
    await user.click(screen.getByRole("button", { name: "Create an account" }));
    await user.type(screen.getByLabelText("Your name"), "New Parent");
    await user.type(screen.getByLabelText("Email address"), "parent@example.test");
    await user.type(screen.getByLabelText("Username"), "parent.one");
    await user.type(screen.getByLabelText("Password"), "a memorable passphrase");
    await user.click(screen.getByRole("button", { name: "Create account" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Verify your email");
    expect(mocks.register).toHaveBeenCalledWith(expect.objectContaining({ username: "parent.one", email: "parent@example.test" }));
    expect(mocks.replace).not.toHaveBeenCalled();
  });
});

it("explains expired MFA while offering a fresh sign-in", async () => {
  mocks.query = "error=mfa_challenge_expired&next=/admin/settings";
  render(<SignInForm />);
  expect(await screen.findByRole("alert")).toHaveTextContent("Your sign-in verification expired. Please sign in again.");
  expect(screen.getByRole("button", { name: "Sign in" })).toBeEnabled();
  expect(mocks.replace).not.toHaveBeenCalled();
});

it("replaces an admin return path when a guardian is already signed in", async () => {
  mocks.query = "next=/admin/settings";
  mocks.session = { authenticated: true, csrfToken: "csrf", user: { id: "guardian", roles: ["guardian"] } };
  render(<SignInForm />);
  await waitFor(() => expect(mocks.replace).toHaveBeenCalledWith("/guardian"));
  expect(screen.queryByLabelText("Password")).not.toBeInTheDocument();
});
