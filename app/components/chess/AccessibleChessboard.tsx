"use client";

import { createContext, useContext, useMemo, type CSSProperties, type ReactNode } from "react";
import { Chessboard, defaultPieces, useChessboardContext, type ChessboardOptions } from "react-chessboard";

const SquareName = createContext<string | null>(null);
const hiddenText: CSSProperties = {
  position: "absolute", width: 1, height: 1, padding: 0, margin: -1,
  overflow: "hidden", clipPath: "inset(50%)", whiteSpace: "nowrap", border: 0,
};
const pieceNames: Record<string, string> = {
  wP: "White pawn", wN: "White knight", wB: "White bishop", wR: "White rook", wQ: "White queen", wK: "White king",
  bP: "Black pawn", bN: "Black knight", bB: "Black bishop", bR: "Black rook", bQ: "Black queen", bK: "Black king",
};

function namedPieces(pieces: NonNullable<ChessboardOptions["pieces"]>) {
  return Object.fromEntries(Object.entries(pieces).map(([code, Piece]) => {
    function NamedPiece(props: Parameters<typeof Piece>[0]) {
      const square = useContext(SquareName);
      const name = pieceNames[code] || code;
      return <><span aria-hidden="true" style={{ display: "contents" }}><Piece {...props} /></span><span style={hiddenText}>{square ? `${name} on ${square}` : name}</span></>;
    }
    return [code, NamedPiece];
  }));
}
const namedDefaultPieces = namedPieces(defaultPieces);

function StyledSquare({ square, children }: { square: string; children: ReactNode }) {
  const { squareStyles } = useChessboardContext();
  return <div style={{ width: "100%", height: "100%", ...squareStyles[square] }}>{children}</div>;
}

/** Name the library's native drag buttons without replacing their interaction. */
export default function AccessibleChessboard({ options = {} }: { options?: ChessboardOptions }) {
  const pieces = useMemo(() => options.pieces ? namedPieces(options.pieces) : namedDefaultPieces, [options.pieces]);
  const squareRenderer = options.squareRenderer;
  return <Chessboard options={{ ...options, pieces, squareRenderer: (square) => (
    <SquareName value={square.square}>
      {squareRenderer?.(square) || <StyledSquare square={square.square}>{square.children}</StyledSquare>}
    </SquareName>
  ) }} />;
}
