"use client";

import { useEffect, useRef, useState } from "react";
export type EngineLine = {
  rank: number;
  depth: number;
  score: string;
  moves: string[];
};
type Analysis = {
  fen: string;
  lines: Record<number, EngineLine>;
  error: string;
};

/** Serializes UCI searches: a stopped search must send bestmove before the next one starts. */
export class EngineSearch {
  private ready = false;
  private busy = false;
  private desired = "";
  private searching = "";
  constructor(
    private send: (message: string) => void,
    private receive: (fen: string, line: EngineLine) => void,
  ) {}
  position(fen: string) {
    this.desired = fen;
    if (this.busy) this.send("stop");
    else this.start();
  }
  message(message: string) {
    if (message === "uciok") {
      this.send("setoption name MultiPV value 3");
      this.send("setoption name Hash value 16");
      this.send("isready");
    } else if (message === "readyok") {
      this.ready = true;
      this.start();
    } else if (message.startsWith("bestmove")) {
      this.busy = false;
      if (this.searching !== this.desired) this.start();
    } else if (
      message.startsWith("info ") &&
      this.busy &&
      this.searching === this.desired
    ) {
      const depth = message.match(/\bdepth (\d+)/);
      const score = message.match(/\bscore (cp|mate) (-?\d+)/);
      const pv = message.match(/\bpv (.+)$/);
      const rank = Number(message.match(/\bmultipv (\d+)/)?.[1] || 1);
      if (!depth || !score || !pv) return;
      const evaluation =
        Number(score[2]) * (this.searching.split(" ")[1] === "w" ? 1 : -1);
      const formatted =
        score[1] === "mate"
          ? `Mate ${evaluation}`
          : `${evaluation >= 0 ? "+" : ""}${(evaluation / 100).toFixed(2)}`;
      this.receive(this.searching, {
        rank,
        depth: Number(depth[1]),
        score: formatted,
        moves: pv[1].split(" ").slice(0, 12),
      });
    }
  }
  private start() {
    if (!this.ready || this.busy || !this.desired) return;
    this.searching = this.desired;
    this.busy = true;
    this.send(`position fen ${this.searching}`);
    this.send("go depth 18 movetime 2000");
  }
}

export function useEngine(fen: string, enabled: boolean) {
  const search = useRef<EngineSearch | null>(null);
  const [analysis, setAnalysis] = useState<Analysis>({
    fen: "",
    lines: {},
    error: "",
  });
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    let engine: Worker;
    try {
      engine = new Worker("/engine/stockfish-19-lite-single.js");
    } catch {
      queueMicrotask(() => {
        if (active)
          setAnalysis({
            fen: "",
            lines: {},
            error: "The engine could not start in this browser.",
          });
      });
      return;
    }
    const coordinator = new EngineSearch(
      (message) => engine.postMessage(message),
      (position, line) => {
        if (active)
          setAnalysis((previous) => ({
            fen: position,
            error: "",
            lines: {
              ...(previous.fen === position ? previous.lines : {}),
              [line.rank]: line,
            },
          }));
      },
    );
    search.current = coordinator;
    engine.onmessage = (event: MessageEvent<string>) =>
      coordinator.message(String(event.data));
    engine.onerror = () => {
      if (active)
        setAnalysis({
          fen: "",
          lines: {},
          error:
            "The engine could not be loaded. Your board and saved studies remain available.",
        });
    };
    engine.postMessage("uci");
    return () => {
      active = false;
      engine.postMessage("stop");
      engine.terminate();
      search.current = null;
    };
  }, [enabled]);
  useEffect(() => {
    search.current?.position(fen);
  }, [fen, enabled]);
  return {
    lines:
      enabled && analysis.fen === fen
        ? Object.values(analysis.lines).sort((a, b) => a.rank - b.rank)
        : [],
    error: analysis.error,
  };
}
