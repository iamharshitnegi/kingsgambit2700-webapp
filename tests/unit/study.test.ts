import { describe, expect, it } from "vitest";
import {
  addStudyMove,
  exportStudy,
  importStudy,
  newStudy,
  studyPath,
} from "@/app/lib/chess/study";

describe("saved chess studies", () => {
  it("round-trips main lines, nested variations and comments", () => {
    const pgn =
      '[Event "Lesson"]\n\n1. e4 {Centre} (1. d4 d5 (1... Nf6)) e5 2. Nf3 *';
    const study = importStudy(pgn);
    const again = importStudy(exportStudy(study));
    expect(
      Object.values(again.nodes)
        .map((node) => node.uci)
        .sort(),
    ).toEqual(
      Object.values(study.nodes)
        .map((node) => node.uci)
        .sort(),
    );
    expect(again.nodes.root.children).toHaveLength(2);
    expect(
      Object.values(again.nodes).find((node) => node.san === "e4")?.comment,
    ).toBe("Centre");
  });
  it("reuses an existing branch and preserves a promotion from a custom position", () => {
    const base = newStudy("8/P6k/8/8/8/8/8/7K w - - 0 1");
    const played = addStudyMove(base, { from: "a7", to: "a8", promotion: "n" });
    const replayed = addStudyMove({ ...played, currentId: "root" }, "a8=N");
    expect(Object.keys(replayed.nodes)).toHaveLength(2);
    expect(studyPath(replayed)[0].uci).toBe("a7a8n");
    expect(importStudy(exportStudy(replayed)).nodes.root.fen).toBe(
      base.nodes.root.fen,
    );
  });
  it("preserves root notes, arrows, highlights and semicolons within comments", () => {
    const study = importStudy(
      "{Start; plan [%csl Ge4]} 1. e4 {Centre [%cal Re2e4]} *",
    );
    const restored = importStudy(exportStudy(study));
    expect(restored.nodes.root.comment).toBe("Start; plan");
    expect(restored.nodes.root.arrows[0]).toMatchObject({
      startSquare: "e4",
      endSquare: "e4",
      color: "#166534",
    });
    expect(
      Object.values(restored.nodes).find((node) => node.san === "e4")
        ?.arrows[0],
    ).toMatchObject({ startSquare: "e2", endSquare: "e4", color: "#b91c1c" });
    expect(() => importStudy("1. e4 {unfinished")).toThrow(/comment/);
    expect(() => importStudy("1. e4 }")).toThrow(/comment/);
  });
  it("rejects illegal moves and broken variation structure", () => {
    expect(() => importStudy("1. e4 e5 2. Ke3 *")).toThrow(/illegal move/);
    expect(() => importStudy("1. e4 (1. d4 *")).toThrow(/unclosed/);
    expect(() => importStudy("1. e4 ) *")).toThrow(/unmatched/);
  });
});
