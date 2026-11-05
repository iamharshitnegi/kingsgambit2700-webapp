import { describe, expect, it, vi } from "vitest";
import { EngineSearch } from "@/app/lib/chess/use-engine";

describe("engine search coordination", () => {
  it("waits for readiness and the previous search's bestmove, discarding stale evaluations", () => {
    const send = vi.fn();
    const receive = vi.fn();
    const engine = new EngineSearch(send, receive);
    engine.position("position-one w");
    expect(send).not.toHaveBeenCalled();
    engine.message("uciok");
    engine.message("readyok");
    expect(send).toHaveBeenCalledWith("position fen position-one w");
    engine.message("info depth 10 score cp 50 pv e2e4 e7e5");
    expect(receive).toHaveBeenLastCalledWith(
      "position-one w",
      expect.objectContaining({ score: "+0.50" }),
    );
    receive.mockClear();
    send.mockClear();
    engine.position("position-two b");
    engine.position("position-three b");
    engine.message("info depth 11 score cp 60 pv e2e4");
    expect(receive).not.toHaveBeenCalled();
    expect(send).not.toHaveBeenCalledWith("position fen position-three b");
    engine.message("bestmove e2e4");
    expect(send).toHaveBeenCalledWith("position fen position-three b");
    engine.message("info depth 12 multipv 2 score cp 25 pv e7e5");
    expect(receive).toHaveBeenLastCalledWith(
      "position-three b",
      expect.objectContaining({ score: "-0.25", rank: 2 }),
    );
    send.mockClear();
    engine.message("bestmove e7e5");
    expect(send).not.toHaveBeenCalled();
  });
});
