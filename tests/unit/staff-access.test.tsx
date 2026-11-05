import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import StaffAccess from "@/app/components/portal/workspace/StaffAccess";
import { WorkspaceFeedback } from "@/app/components/portal/workspace/WorkspaceUI";

const api = vi.hoisted(() => vi.fn());
vi.mock("@/app/lib/api/client", () => ({ apiFetch: api }));
const coach = { id: "coach", displayName: "Maya", email: "maya@example.test", roles: ["coach", "academic_manager"], status: "active", createdAt: "2026-09-21T10:00:00Z", updatedAt: "2026-09-22T10:00:00Z" };
beforeEach(() => {
  api.mockReset();
  HTMLDialogElement.prototype.showModal = function () { this.setAttribute("open", ""); };
  HTMLDialogElement.prototype.close = function () { this.removeAttribute("open"); };
});
afterEach(cleanup);

describe("staff directory", () => {
  it("shows all roles and suspends with a version check without replacing roles", async () => {
    const mutate = vi.fn().mockResolvedValue(true);
    api.mockResolvedValueOnce({ staff: [coach] }).mockResolvedValueOnce({ staff: [{ ...coach, status: "suspended", updatedAt: "2026-09-22T11:00:00Z" }] });
    render(<StaffAccess actorId="owner" mutate={mutate} />);
    expect(await screen.findByText(/Coach · Academic manager/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Edit staff access" }));
    const dialog = within(screen.getByRole("dialog"));
    await userEvent.selectOptions(dialog.getByLabelText("Account status"), "suspended");
    await userEvent.click(dialog.getByRole("button", { name: "Save staff access" }));
    await waitFor(() => expect(mutate).toHaveBeenCalledWith("/academy/staff/coach", { displayName: "Maya", status: "suspended", expectedUpdatedAt: coach.updatedAt }, "Staff access suspended and all remembered browsers signed out.", "PATCH"));
    expect(await screen.findByRole("status")).toHaveTextContent("all remembered browsers signed out");
    expect(screen.getByText("suspended")).toBeInTheDocument();
    expect(mutate).toHaveBeenCalledOnce();
  });

  it("keeps failed edits open and prevents selecting self-suspension", async () => {
    api.mockResolvedValueOnce({ staff: [{ ...coach, id: "owner", roles: ["super_admin"] }] });
    render(<WorkspaceFeedback value={{ error: "This record changed. Refresh and try again.", busy: false }}><StaffAccess actorId="owner" mutate={vi.fn().mockResolvedValue(false)} /></WorkspaceFeedback>);
    await userEvent.click(await screen.findByRole("button", { name: "Edit staff access" }));
    const dialog = within(screen.getByRole("dialog"));
    expect(dialog.getByRole("option", { name: "Suspended" })).toBeDisabled();
    await userEvent.click(dialog.getByRole("button", { name: "Save staff access" }));
    expect(await dialog.findByRole("alert")).toHaveTextContent("This record changed");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });
});
