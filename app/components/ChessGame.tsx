'use client';

import { useState, useEffect, useMemo } from 'react';
import { Chess } from 'chess.js';
import dynamic from 'next/dynamic';

// Dynamic import to prevent SSR issues with the board library
const Chessboard = dynamic(() => import('react-chessboard').then((mod) => mod.Chessboard), { 
  ssr: false,
  loading: () => <div className="w-full h-96 bg-gray-100 flex items-center justify-center text-gray-400">Loading Board...</div>
});

interface ChessGameProps {
  pgn: string;
  title?: string;
}

export default function ChessGame({ pgn, title }: ChessGameProps) {
  // 1. STATE: Only store the "Master" game logic and the current position index
  const [masterGame, setMasterGame] = useState<Chess | null>(null);
  const [moveIndex, setMoveIndex] = useState(-1); // -1 = Start Position

  // 2. LOAD PGN: Runs once when PGN changes
  useEffect(() => {
    try {
      const g = new Chess();
      // Clean whitespace/newlines from PGN
      const cleanPgn = pgn.replace(/\n/g, ' ').trim();
      g.loadPgn(cleanPgn);
      
      setMasterGame(g);
      setMoveIndex(-1); // Reset to start
    } catch (e) {
      console.error("PGN Error:", e);
    }
  }, [pgn]);

  // 3. DERIVED STATE: Calculate the board position on the fly
  // We use useMemo so we don't re-calculate this on unrelated renders
  const visualFen = useMemo(() => {
    // If no game loaded, return empty start board
    if (!masterGame) return new Chess().fen();

    const g = new Chess();
    const history = masterGame.history();

    // Replay moves up to the current index
    for (let i = 0; i <= moveIndex; i++) {
      if (history[i]) {
        g.move(history[i]);
      }
    }
    return g.fen();
  }, [masterGame, moveIndex]);

  // 4. HANDLERS: Simply update the index
  function handleNext() {
    if (!masterGame) return;
    const allMoves = masterGame.history();
    if (moveIndex < allMoves.length - 1) {
      setMoveIndex(prev => prev + 1);
    }
  }

  function handlePrev() {
    if (moveIndex >= 0) {
      setMoveIndex(prev => prev - 1);
    }
  }

  // Prevent render if game isn't loaded yet
  if (!masterGame) return null;

  return (
    <div className="my-10 mx-auto max-w-md bg-white p-4 rounded-xl shadow-lg border border-slate-200">
      {title && <h3 className="text-center font-bold text-brand-dark mb-3">{title}</h3>}

      <div className="w-full aspect-square relative border border-gray-300">
        {/* @ts-ignore */}
        <Chessboard options={{
          id: "ReplayBoard",
          position: visualFen,
          animationDurationInMs: 200,
        }}
        />
      </div>

      <div className="flex justify-center items-center gap-4 mt-4 select-none">
        <button 
          onClick={handlePrev}
          disabled={moveIndex === -1}
          className="bg-brand-primary text-white px-4 py-2 rounded disabled:opacity-50 disabled:cursor-not-allowed hover:bg-slate-700 w-24 transition"
        >
          Prev
        </button>

        <span className="font-mono font-bold text-gray-600 w-24 text-center">
             {moveIndex === -1 ? "Start" : `Move ${Math.ceil((moveIndex + 1) / 2)}`}
        </span>

        <button 
          onClick={handleNext}
          disabled={masterGame && moveIndex === masterGame.history().length - 1}
          className="bg-brand-primary text-white px-4 py-2 rounded disabled:opacity-50 disabled:cursor-not-allowed hover:bg-slate-700 w-24 transition"
        >
          Next
        </button>
      </div>
      
      {/* Optional: Debug info to confirm the FEN updates */}
      <div className="mt-4 p-2 bg-gray-50 text-[10px] text-gray-400 font-mono text-center truncate">
        {visualFen}
      </div>
    </div>
  );
}