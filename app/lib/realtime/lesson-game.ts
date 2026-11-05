import { Chess } from "chess.js";
import type { ClassroomBoardState } from "./classroom-protocol";

export function importLessonPGN(pgn: string) {
  if (!pgn.trim() || pgn.length > 64_000) throw new Error("Choose a PGN of up to 64 KB.");
  const game = new Chess();
  game.loadPgn(pgn);
  const history = game.history({ verbose: true });
  if (history.length > 512) throw new Error("Import a game with at most 512 moves.");
  return { initialFen: history[0]?.before ?? game.fen(), fen: game.fen(), moves: history.map((move) => `${move.from}${move.to}${move.promotion ?? ""}`) };
}

export function exportLessonPGN(board: ClassroomBoardState, title: string) {
  const initialFen = board.lesson?.initialFen;
  if (!initialFen) throw new Error("Load a position or import a game before exporting this older lesson.");
  const game = new Chess(initialFen);
  for (const move of board.moves) game.move({ from: move.slice(0, 2), to: move.slice(2, 4), promotion: move.slice(4) || undefined });
  game.setHeader("Event", title || "Academy lesson");
  game.setHeader("Site", "KingsGambit2700 classroom");
  return game.pgn({ maxWidth: 80 });
}

export function safeMeetingURL(value?: string) {
  if (!value) return undefined;
  try {
    const url = new URL(value);
    return url.protocol === "https:" && !url.username && !url.password ? url.href : undefined;
  } catch { return undefined; }
}
