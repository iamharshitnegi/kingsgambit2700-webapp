import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, expect, it } from "vitest";
import PublicNavbar from "@/app/components/public/PublicNavbar";
import { fallbackOrganization } from "@/app/lib/fallback-content";

vi.mock("next/navigation", () => ({ usePathname: () => "/programs" }));

describe("public navigation", () => {
  it("exposes and closes the mobile navigation accessibly", async () => {
    const user = userEvent.setup();
    render(<PublicNavbar organization={fallbackOrganization} />);
    const toggle = screen.getByRole("button", { name: "Open navigation" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("navigation", { name: "Primary navigation" })).toBeInTheDocument();
  });
});
