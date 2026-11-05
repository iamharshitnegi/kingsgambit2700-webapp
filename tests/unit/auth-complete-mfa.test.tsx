import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { APIError } from "@/app/lib/api/client";
import AuthComplete from "@/app/components/portal/AuthComplete";

const navigation = vi.hoisted(() => {
  const replace = vi.fn();
  return {
    params: new URLSearchParams(),
    replace,
    router: { replace },
  };
});

const auth = vi.hoisted(() => ({
  getSession: vi.fn(),
  clearSession: vi.fn(),
  getMFAStatus: vi.fn(),
  beginTOTPEnrollment: vi.fn(),
  confirmTOTPEnrollment: vi.fn(),
  completeMFAChallenge: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => navigation.router,
  useSearchParams: () => navigation.params,
}));

vi.mock("@/app/lib/auth-client", () => auth);

const coach = {
  id: "01945be0-6d64-77e6-935d-c78a4f688e85",
  email: "coach@example.test",
  roles: ["coach"],
} as const;

const administrator = {
  id: "01945be0-6d64-77e6-935d-c78a4f688e86",
  email: "admin@example.test",
  roles: ["operations_admin"],
} as const;

beforeEach(() => {
  navigation.params = new URLSearchParams();
  navigation.replace.mockReset();
  auth.getSession.mockReset();
  auth.clearSession.mockReset();
  auth.getMFAStatus.mockReset();
  auth.beginTOTPEnrollment.mockReset();
  auth.confirmTOTPEnrollment.mockReset();
  auth.completeMFAChallenge.mockReset();
});

afterEach(cleanup);

describe("staff MFA completion", () => {
  it("enrolls an authenticator and requires recovery-code acknowledgement", async () => {
    const user = userEvent.setup();
    const recoveryCodes = Array.from({ length: 10 }, (_, index) => `ABCD-EFGH-IJKL-MNOP-QRST-${String(index).padStart(4, "0")}`);
    auth.getSession.mockResolvedValue({ authenticated: false, mfaRequired: true });
    auth.getMFAStatus.mockResolvedValue({
      required: true,
      enrolled: false,
      enrollmentPending: false,
      verifiedThisSession: false,
      csrfToken: "status-csrf",
    });
    auth.beginTOTPEnrollment.mockResolvedValue({
      secret: "JBSWY3DPEHPK3PXP",
      otpauthUri: "otpauth://totp/KingsGambit2700:coach@example.test?secret=JBSWY3DPEHPK3PXP",
      expiresAt: "2026-08-30T12:05:00Z",
    });
    auth.confirmTOTPEnrollment.mockResolvedValue({
      authenticated: true,
      csrfToken: "rotated-csrf",
      user: coach,
      recoveryCodes,
    });

    render(<AuthComplete />);

    expect(screen.queryByRole("img", { name: "Authenticator setup QR code" })).not.toBeInTheDocument();
    await user.click(await screen.findByRole("button", { name: "Set up authenticator" }));
    expect(await screen.findByRole("img", { name: "Authenticator setup QR code" })).toBeVisible();
    expect(await screen.findByLabelText("Authenticator setup secret")).toHaveTextContent("JBSWY3DPEHPK3PXP");
    expect(screen.getByRole("link", { name: "Open in authenticator app" })).toHaveAttribute(
      "href",
      expect.stringMatching(/^otpauth:\/\/totp\//),
    );

    await user.type(screen.getByLabelText("Six-digit authenticator code"), "123456");
    await user.click(screen.getByRole("button", { name: "Confirm authenticator" }));

    expect(await screen.findByRole("heading", { name: "Save your recovery codes now." })).toBeInTheDocument();
    expect(screen.queryByRole("img", { name: "Authenticator setup QR code" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Authenticator setup secret")).not.toBeInTheDocument();
    expect(screen.getByText(recoveryCodes[0])).toBeInTheDocument();
    const continueButton = screen.getByRole("button", { name: "Continue to academy portal" });
    expect(continueButton).toBeDisabled();

    await user.click(screen.getByRole("checkbox", { name: "I saved all recovery codes in a secure place." }));
    await user.click(continueButton);

    expect(auth.confirmTOTPEnrollment).toHaveBeenCalledWith("123456");
    expect(navigation.replace).toHaveBeenCalledWith("/coach");
    expect(screen.queryByText(recoveryCodes[0])).not.toBeInTheDocument();
  });

  it("completes an existing authenticator challenge and honors a same-origin next path", async () => {
    const user = userEvent.setup();
    navigation.params = new URLSearchParams({ mfa: "required", next: "/admin?tab=finance" });
    auth.getSession.mockResolvedValue({ authenticated: false });
    auth.getMFAStatus.mockResolvedValue({
      required: true,
      enrolled: true,
      enrollmentPending: false,
      verifiedThisSession: false,
      csrfToken: "status-csrf",
    });
    auth.completeMFAChallenge.mockResolvedValue({
      authenticated: true,
      csrfToken: "challenge-csrf",
      user: administrator,
    });

    render(<AuthComplete />);

    await user.type(await screen.findByLabelText("Six-digit authenticator code"), "654321");
    expect(screen.queryByRole("img", { name: "Authenticator setup QR code" })).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Verify and continue" }));

    expect(auth.completeMFAChallenge).toHaveBeenCalledWith({ code: "654321" });
    expect(navigation.replace).toHaveBeenCalledWith("/admin?tab=finance");
  });

  it("accepts a recovery code and rejects an unsafe return path", async () => {
    const user = userEvent.setup();
    navigation.params = new URLSearchParams({ next: "/\\evil.example/steal" });
    auth.getSession.mockResolvedValue({ authenticated: false, mfaRequired: true });
    auth.getMFAStatus.mockResolvedValue({
      required: true,
      enrolled: true,
      enrollmentPending: false,
      verifiedThisSession: false,
      csrfToken: "status-csrf",
    });
    auth.completeMFAChallenge.mockResolvedValue({
      authenticated: true,
      csrfToken: "challenge-csrf",
      user: coach,
    });

    render(<AuthComplete />);

    await user.click(await screen.findByRole("radio", { name: "Recovery code" }));
    await user.type(screen.getByRole("textbox", { name: "Recovery code" }), "ABCD-EFGH-IJKL-MNOP-QRST-UVWX");
    await user.click(screen.getByRole("button", { name: "Verify and continue" }));

    expect(auth.completeMFAChallenge).toHaveBeenCalledWith({ recoveryCode: "ABCD-EFGH-IJKL-MNOP-QRST-UVWX" });
    expect(navigation.replace).toHaveBeenCalledWith("/coach");
  });

  it("shows the API failure instead of silently entering a portal", async () => {
    auth.getSession.mockResolvedValue({ authenticated: false, mfaRequired: true });
    auth.getMFAStatus.mockRejectedValue(new Error("This MFA challenge has expired."));

    render(<AuthComplete />);

    expect(await screen.findByRole("alert")).toHaveTextContent("This MFA challenge has expired.");
    expect(screen.getByRole("link", { name: "Sign in again" })).toHaveAttribute("href", "/sign-in?next=%2Fportal");
    await waitFor(() => expect(navigation.replace).not.toHaveBeenCalled());
  });
});

const expired = () => new APIError("Request a new magic link to continue", 401, { type: "about:blank", title: "MFA challenge expired", status: 401, code: "mfa_challenge_expired" });

describe("expired MFA sign-in recovery", () => {
  it.each(["status", "setup", "enrollment", "challenge"])("returns to sign-in after expiry during %s", async (phase) => {
    const user = userEvent.setup();
    navigation.params = new URLSearchParams({ mfa: "required", next: "/admin/settings" });
    auth.getSession.mockResolvedValue({ authenticated: false, mfaRequired: true });
    auth.getMFAStatus.mockResolvedValue({ required: true, enrolled: phase === "challenge", enrollmentPending: false, verifiedThisSession: false, csrfToken: "csrf" });
    if (phase === "status") auth.getMFAStatus.mockRejectedValue(expired());
    if (phase === "setup") auth.beginTOTPEnrollment.mockRejectedValue(expired());
    if (phase === "enrollment") {
      auth.beginTOTPEnrollment.mockResolvedValue({ secret: "JBSWY3DPEHPK3PXP", otpauthUri: "otpauth://totp/Academy?secret=JBSWY3DPEHPK3PXP", expiresAt: "2026-09-22T12:05:00Z" });
      auth.confirmTOTPEnrollment.mockRejectedValue(expired());
    }
    if (phase === "challenge") auth.completeMFAChallenge.mockRejectedValue(expired());
    render(<AuthComplete />);
    if (phase === "setup" || phase === "enrollment") await user.click(await screen.findByRole("button", { name: "Set up authenticator" }));
    if (phase === "enrollment" || phase === "challenge") {
      await user.type(await screen.findByLabelText("Six-digit authenticator code"), "123456");
      await user.click(screen.getByRole("button", { name: phase === "enrollment" ? "Confirm authenticator" : "Verify and continue" }));
    }
    await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith("/sign-in?error=mfa_challenge_expired&next=%2Fadmin%2Fsettings"));
    expect(auth.clearSession).toHaveBeenCalledOnce();
    expect(screen.queryByLabelText("Authenticator setup secret")).not.toBeInTheDocument();
  });

  it("restarts missing sign-in sessions without retaining an unsafe return path", async () => {
    navigation.params = new URLSearchParams({ next: "//outside.example" });
    auth.getSession.mockResolvedValue({ authenticated: false });
    render(<AuthComplete />);
    await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith("/sign-in?error=mfa_challenge_expired&next=%2Fportal"));
    expect(auth.getMFAStatus).not.toHaveBeenCalled();
  });

  it("keeps incorrect codes on the MFA form without resetting sign-in", async () => {
    const user = userEvent.setup();
    auth.getSession.mockResolvedValue({ authenticated: false, mfaRequired: true });
    auth.getMFAStatus.mockResolvedValue({ required: true, enrolled: true, verifiedThisSession: false });
    auth.completeMFAChallenge.mockRejectedValue(new APIError("Check the code and try again", 401, { type: "about:blank", title: "Invalid MFA credential", status: 401, code: "invalid_mfa_credential" }));
    render(<AuthComplete />);
    await user.type(await screen.findByLabelText("Six-digit authenticator code"), "123456");
    await user.click(screen.getByRole("button", { name: "Verify and continue" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Check the code");
    expect(auth.clearSession).not.toHaveBeenCalled();
    expect(navigation.replace).not.toHaveBeenCalled();
  });
});

it("routes a guardian to their own portal after an admin account signs out", async () => {
  navigation.params = new URLSearchParams({ next: "/admin/settings" });
  auth.getSession.mockResolvedValue({ authenticated: true, user: { id: "guardian-1", roles: ["guardian"] } });
  render(<AuthComplete />);
  await waitFor(() => expect(navigation.replace).toHaveBeenCalledWith("/guardian"));
  expect(navigation.replace).not.toHaveBeenCalledWith("/admin/settings");
});
