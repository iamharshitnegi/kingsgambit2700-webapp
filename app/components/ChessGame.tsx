"use client";

import dynamic from "next/dynamic";
import { useMemo, useState } from "react";
import { Chess } from "chess.js";

const Chessboard = dynamic(
  () => import("./chess/AccessibleChessboard"),
  { ssr: false, loading: () => <div className="aspect-square bg-brand-cream grid place-items-center text-stone-500">Loading board…</div> },
);

type Props = { pgn: string; title?: string };

function ChessReplay({ pgn, title }: Props) {
  const [moveIndex, setMoveIndex] = useState(-1);
  const game = useMemo(() => {
    try {
      const parsed = new Chess();
      parsed.loadPgn(pgn.replace(/\n/g, " ").trim());
      return parsed;
    } catch {
      return null;
    }
  }, [pgn]);

  const history = useMemo(() => game?.history() ?? [], [game]);
  const position = useMemo(() => {
    const replay = new Chess();
    for (let index = 0; index <= moveIndex; index += 1) {
      if (history[index]) replay.move(history[index]);
    }
    return replay.fen();
  }, [history, moveIndex]);

  if (!game) {
    return <div className="my-10 border border-brand-border bg-brand-paper p-6 text-sm text-stone-600">This game score could not be displayed.</div>;
  }

  return (
    <figure className="my-10 mx-auto max-w-lg bg-brand-paper p-4 border border-brand-border">
      {title && <figcaption className="font-serif text-center text-xl mb-4">{title}</figcaption>}
      <div className="aspect-square border border-brand-border">
        <Chessboard options={{ id: `replay-${pgn.length}`, position, animationDurationInMs: 180 }} />
      </div>
      <div className="flex justify-center items-center gap-4 mt-4">
        <button type="button" onClick={() => setMoveIndex((value) => Math.max(-1, value - 1))} disabled={moveIndex === -1} className="button button-outline disabled:opacity-40">Previous</button>
        <span className="text-xs text-stone-600 w-20 text-center">{moveIndex === -1 ? "Start" : `Move ${Math.ceil((moveIndex + 1) / 2)}`}</span>
        <button type="button" onClick={() => setMoveIndex((value) => Math.min(history.length - 1, value + 1))} disabled={moveIndex === history.length - 1} className="button button-dark disabled:opacity-40">Next</button>
      </div>
    </figure>
  );
}

export default function ChessGame(props: Props) {
  return <ChessReplay key={props.pgn} {...props} />;
}
