import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Chess } from "chess.js";
import { afterEach, describe, expect, it, vi } from "vitest";
import Chessboard from "@/app/components/chess/AccessibleChessboard";

afterEach(cleanup);

describe("chessboard accessible names", () => {
  it("names every draggable piece and preserves its square interaction and highlights", async () => {
    const onSquareClick = vi.fn();
    const { container } = render(<Chessboard options={{ id: "accessible", showAnimations: false, onSquareClick, squareStyles: { e2: { boxShadow: "inset 0 0 0 4px red" } } }} />);
    const pieces = screen.getAllByRole("button");
    expect(pieces).toHaveLength(32);
    for (const piece of pieces) expect(piece).toHaveAccessibleName(/^(White|Black) (pawn|knight|bishop|rook|queen|king) on [a-h][1-8]$/);
    expect(screen.getByRole("button", { name: "Black rook on a8" })).toHaveAttribute("aria-roledescription", "draggable");
    await userEvent.click(screen.getByRole("button", { name: "White pawn on e2" }));
    expect(onSquareClick).toHaveBeenCalledWith({ square: "e2", piece: { pieceType: "wP" } });
    expect(container.querySelector("#accessible-square-e2 > div")).toHaveStyle({ boxShadow: "inset 0 0 0 4px red" });
  });

  it("updates square names after moves and retains names on disabled, flipped previews", async () => {
    const game = new Chess();
    const { rerender } = render(<Chessboard options={{ id: "preview", position: game.fen(), showAnimations: false, allowDragging: false }} />);
    expect(screen.getByRole("button", { name: "White pawn on e2" })).toHaveAttribute("aria-disabled", "true");
    game.move("e4");
    rerender(<Chessboard options={{ id: "preview", position: game.fen(), showAnimations: false, allowDragging: false, boardOrientation: "black" }} />);
    await waitFor(() => expect(screen.getByRole("button", { name: "White pawn on e4" })).toBeInTheDocument());
    expect(screen.queryByRole("button", { name: "White pawn on e2" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Black king on e8" })).toHaveAttribute("aria-disabled", "true");
  });
});
