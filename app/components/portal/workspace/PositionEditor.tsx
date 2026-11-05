"use client";
import { Chess, DEFAULT_POSITION } from "chess.js";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import { useState } from "react";
import { Field, Modal } from "./WorkspaceUI";

type Pieces = Record<string, { pieceType: string }>;
function piecesFor(fen: string): Pieces {
  return Object.fromEntries(
    new Chess(fen)
      .board()
      .flat()
      .filter((piece) => piece !== null)
      .map((piece) => [
        piece.square,
        { pieceType: `${piece.color}${piece.type.toUpperCase()}` },
      ]),
  );
}
export function positionFEN(
  pieces: Pieces,
  turn: string,
  castling: string,
  enPassant: string,
) {
  const ranks: string[] = [];
  for (let rank = 8; rank >= 1; rank--) {
    let row = "";
    let empty = 0;
    for (const file of "abcdefgh") {
      const piece = pieces[`${file}${rank}`]?.pieceType;
      if (!piece) empty++;
      else {
        if (empty) row += empty;
        empty = 0;
        row +=
          piece[0] === "w" ? piece[1].toUpperCase() : piece[1].toLowerCase();
      }
    }
    if (empty) row += empty;
    ranks.push(row);
  }
  return `${ranks.join("/")} ${turn} ${castling || "-"} ${enPassant || "-"} 0 1`;
}
const symbols: Record<string, string> = {
  wK: "♔",
  wQ: "♕",
  wR: "♖",
  wB: "♗",
  wN: "♘",
  wP: "♙",
  bK: "♚",
  bQ: "♛",
  bR: "♜",
  bB: "♝",
  bN: "♞",
  bP: "♟",
};
const pieceNames: Record<string, string> = {
  K: "king",
  Q: "queen",
  R: "rook",
  B: "bishop",
  N: "knight",
  P: "pawn",
};
export default function PositionEditor({
  fen,
  close,
  apply,
}: {
  fen: string;
  close: () => void;
  apply: (fen: string) => void;
}) {
  const [pieces, setPieces] = useState(() => piecesFor(fen));
  const [selected, setSelected] = useState("wP");
  const [turn, setTurn] = useState(fen.split(" ")[1]);
  const [castling, setCastling] = useState(fen.split(" ")[2]);
  const [enPassant, setEnPassant] = useState(fen.split(" ")[3]);
  const [input, setInput] = useState(fen);
  const [square, setSquare] = useState("e4");
  const [error, setError] = useState("");
  function place(square: string) {
    setPieces((previous) => {
      const next = { ...previous };
      if (selected === "erase") delete next[square];
      else next[square] = { pieceType: selected };
      return next;
    });
  }
  function load(value: string) {
    try {
      const game = new Chess(value);
      setPieces(piecesFor(game.fen()));
      setTurn(game.turn());
      setCastling(game.fen().split(" ")[2]);
      setEnPassant(game.fen().split(" ")[3]);
      setError("");
    } catch {
      setError(
        "That FEN is invalid. Include exactly one king for each side and valid position fields.",
      );
    }
  }
  return (
    <Modal title="Set up a position" close={close}>
      <div className="academy-position-editor">
        <Chessboard
          options={{
            id: "position-editor",
            position: pieces,
            onSquareClick: ({ square }) => place(square),
            onPieceDrop: ({ sourceSquare, targetSquare }) => {
              if (!targetSquare || !pieces[sourceSquare]) return false;
              setPieces((previous) => {
                const next = {
                  ...previous,
                  [targetSquare]: previous[sourceSquare],
                };
                delete next[sourceSquare];
                return next;
              });
              return true;
            },
          }}
        />
        <div className="academy-stack">
          <div className="academy-piece-palette">
            {Object.entries(symbols).map(([piece, symbol]) => (
              <button
                key={piece}
                aria-label={`${piece[0] === "w" ? "White" : "Black"} ${pieceNames[piece[1]]}`}
                aria-pressed={selected === piece}
                onClick={() => setSelected(piece)}
              >
                {symbol}
              </button>
            ))}
            <button
              aria-pressed={selected === "erase"}
              onClick={() => setSelected("erase")}
            >
              Erase
            </button>
          </div>
          <p>
            Choose a piece, then select a square. Drag pieces to rearrange them.
          </p>
          <Field label="Square">
            <select
              value={square}
              onChange={(event) => setSquare(event.target.value)}
            >
              {Array.from({ length: 8 }, (_, rank) =>
                Array.from("abcdefgh", (file) => `${file}${rank + 1}`),
              )
                .flat()
                .map((square) => (
                  <option key={square}>{square}</option>
                ))}
            </select>
          </Field>
          <button onClick={() => place(square)}>Place on {square}</button>
        </div>
      </div>
      <div className="academy-form">
        <div className="academy-form-grid">
          <Field label="Side to move">
            <select
              value={turn}
              onChange={(event) => setTurn(event.target.value)}
            >
              <option value="w">White</option>
              <option value="b">Black</option>
            </select>
          </Field>
          <Field label="Castling rights">
            <input
              value={castling}
              onChange={(event) => setCastling(event.target.value)}
              placeholder="KQkq or -"
              pattern="(-|K?Q?k?q?)"
            />
          </Field>
          <Field label="En passant square">
            <input
              value={enPassant}
              onChange={(event) => setEnPassant(event.target.value)}
              placeholder="-"
            />
          </Field>
        </div>
        <div className="academy-actions">
          <button onClick={() => load(DEFAULT_POSITION)}>
            Initial position
          </button>
          <button
            onClick={() => {
              setPieces({});
              setCastling("-");
              setEnPassant("-");
            }}
          >
            Clear board
          </button>
        </div>
        <Field label="Import FEN">
          <textarea
            value={input}
            onChange={(event) => setInput(event.target.value)}
            rows={3}
          />
        </Field>
        <button onClick={() => load(input)}>Preview FEN</button>
        {error && (
          <p className="form-message form-message-error" role="alert">
            {error}
          </p>
        )}
        <button
          className="button button-dark"
          onClick={() => {
            try {
              const game = new Chess(
                positionFEN(pieces, turn, castling, enPassant),
              );
              apply(game.fen());
              close();
            } catch {
              setError(
                "This position is invalid. Check both kings, pawns, side to move, castling, and en passant.",
              );
            }
          }}
        >
          Use this position
        </button>
      </div>
    </Modal>
  );
}
