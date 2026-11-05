import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import DeviceSessions from "@/app/components/portal/workspace/DeviceSessions";
const mocks = vi.hoisted(() => ({ api: vi.fn(), session: vi.fn(), logout: vi.fn(), replace: vi.fn() }));
vi.mock("@/app/lib/api/client", () => ({ apiFetch: mocks.api }));
vi.mock("@/app/lib/auth-client", () => ({ getSession: mocks.session, logout: mocks.logout }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mocks.replace }) }));
const current = { id: "current", createdAt: "2026-09-22T10:00:00Z", lastSeenAt: "2026-09-22T11:00:00Z", current: true, remembered: true };
const other = { ...current, id: "other", current: false };
beforeEach(() => { vi.resetAllMocks(); mocks.session.mockResolvedValue({ authenticated: true }); });
afterEach(cleanup);
describe("remembered browsers settings", () => {
  it("removes another browser without signing out this browser", async () => {
    mocks.api.mockResolvedValueOnce({ items: [current, other] }).mockResolvedValueOnce(undefined).mockResolvedValueOnce({ items: [current] });
    render(<DeviceSessions />);
    await userEvent.click(await screen.findByRole("button", { name: "Sign out browser 2" }));
    await waitFor(() => expect(mocks.api).toHaveBeenCalledWith("/auth/sessions/other", { method: "DELETE" }));
    expect(await screen.findByRole("status")).toHaveTextContent("That browser has been signed out");
    expect(screen.getByText("This browser")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Sign out browser 2" })).not.toBeInTheDocument();
    expect(mocks.logout).not.toHaveBeenCalled();
  });
  it("explicitly signs out this browser through the coordinated logout flow", async () => {
    mocks.api.mockResolvedValue({ items: [current] });
    render(<DeviceSessions />);
    await userEvent.click(await screen.findByRole("button", { name: "Sign out this browser" }));
    await waitFor(() => expect(mocks.logout).toHaveBeenCalledOnce());
    expect(mocks.replace).toHaveBeenCalledWith("/sign-in");
  });
});
