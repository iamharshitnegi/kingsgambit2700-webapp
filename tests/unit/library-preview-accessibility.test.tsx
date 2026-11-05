import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import LibraryPanel from "@/app/components/portal/workspace/LibraryPanel";
import type { LearningWorkspace } from "@/app/lib/academy-types";

vi.mock("@/app/components/portal/workspace/TeachingBoard", () => ({ default: () => <div>Selected study</div> }));
afterEach(cleanup);

it("keeps preview controls outside the pieces and supports keyboard position selection", async () => {
  const learning: LearningWorkspace = {
    pagination: { limit: 500, hasMore: false, nextLimit: 1000 }, placements: [], canPlace: false,
    students: [], assignments: [], games: [], threads: [], notifications: [], rubrics: [], assessments: [], certificates: [], canPublish: false, canCreateResources: false,
    resources: [{ id: "opening", ownerId: "coach", title: "Opening study", kind: "pgn", visibility: "shared", status: "published", levelSlug: "beginner", tags: [], version: 1, content: '[Event "One"]\n\n1. e4 *\n\n[Event "Two"]\n\n1. d4 *' }],
  };
  const { container } = render(<LibraryPanel learning={learning} act={vi.fn()} />);
  await userEvent.click(screen.getByRole("button", { name: "Open resource" }));
  const previews = container.querySelector(".academy-position-previews") as HTMLElement;
  expect(previews.querySelector("button [role=button]")).toBeNull();
  expect(within(previews).getAllByRole("button", { name: "White king on e1" })).toHaveLength(2);
  const second = within(previews).getByRole("button", { name: "Select position 2: Two" });
  second.focus(); await userEvent.keyboard(" ");
  expect(second).toHaveAttribute("aria-pressed", "true");
  expect(screen.getByLabelText("Game or position")).toHaveValue("1");
});
