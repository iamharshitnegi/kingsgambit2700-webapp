"use client";

import { Chess } from "chess.js";
import type { Arrow } from "react-chessboard";
import Chessboard from "@/app/components/chess/AccessibleChessboard";
import { useEffect, useMemo, useState } from "react";
import {
  addStudyMove,
  exportStudy,
  importStudy,
  newStudy,
  pawnPosition,
  studyPath,
  updateStudyNode,
  type Study,
} from "@/app/lib/chess/study";
import { useEngine } from "@/app/lib/chess/use-engine";
import PositionEditor from "./PositionEditor";
import { Field, Modal, download, formValues, value } from "./WorkspaceUI";

type Settings = {
  orientation: "white" | "black";
  coordinates: boolean;
  moves: boolean;
  pawns: boolean;
  bestArrow: boolean;
  theme: string;
};
const defaultSettings: Settings = {
  orientation: "white",
  coordinates: true,
  moves: true,
  pawns: false,
  bestArrow: true,
  theme: "academy",
};
const themes: Record<string, [string, string]> = {
  academy: ["#B08C5C", "#EDDEC1"],
  forest: ["#769656", "#EEEED2"],
  slate: ["#708090", "#E6EBEF"],
};

function safeStudy(pgn?: string, fen?: string) {
  try {
    return pgn ? importStudy(pgn) : newStudy(fen);
  } catch {
    return newStudy();
  }
}

export default function TeachingBoard({
  pgn,
  fen,
  title = "Teaching board",
  onSave,
  onLoadPosition,
}: {
  pgn?: string;
  fen?: string;
  title?: string;
  onSave?: (pgn: string) => Promise<boolean>;
  onLoadPosition?: (fen: string, moves: string[]) => void;
}) {
  const [study, setStudy] = useState<Study>(() => safeStudy(pgn, fen));
  const [settings, setSettings] = useState<Settings>(defaultSettings);
  const [engineEnabled, setEngine] = useState(false);
  const [dialog, setDialog] = useState<
    "import" | "position" | "settings" | null
  >(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [selectedSquare, setSelected] = useState<string | null>(null);
  const [promotion, setPromotion] = useState("q");
  const [color, setColor] = useState("#b45309");
  const [deadline, setDeadline] = useState<number | null>(null);
  const [remaining, setRemaining] = useState(0);
  const [saving, setSaving] = useState(false);
  const node = study.nodes[study.currentId];
  const engine = useEngine(node.fen, engineEnabled);
  const path = useMemo(() => studyPath(study), [study]);
  useEffect(() => {
    setStudy(safeStudy(pgn, fen));
  }, [pgn, fen]);
  useEffect(() => {
    try {
      const saved = JSON.parse(
        localStorage.getItem("kg2700.board-preferences") || "null",
      );
      if (saved)
        setSettings({
          ...defaultSettings,
          ...saved,
          theme: themes[saved.theme] ? saved.theme : "academy",
        });
    } catch {
      /* Defaults remain available. */
    }
  }, []);
  useEffect(() => {
    if (!deadline) return;
    const tick = () => {
      const seconds = Math.max(0, Math.ceil((deadline - Date.now()) / 1000));
      setRemaining(seconds);
      if (!seconds) {
        setDeadline(null);
        setNotice("Time is up.");
      }
    };
    tick();
    const interval = setInterval(tick, 250);
    return () => clearInterval(interval);
  }, [deadline]);
  function preferences(change: Partial<Settings>) {
    const next = { ...settings, ...change };
    setSettings(next);
    try {
      localStorage.setItem("kg2700.board-preferences", JSON.stringify(next));
    } catch {
      /* Optional preference storage. */
    }
  }
  function play(
    move: string | { from: string; to: string; promotion?: string },
  ) {
    try {
      setStudy(addStudyMove(study, move));
      setSelected(null);
      setError("");
      setNotice("");
      return true;
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Choose a legal move.");
      return false;
    }
  }
  function jump(id: string) {
    setStudy({ ...study, currentId: id });
    setSelected(null);
  }
  function last() {
    let target = node;
    while (target.children.length) target = study.nodes[target.children[0]];
    jump(target.id);
  }
  const colors = themes[settings.theme] || themes.academy;
  const bestMove =
    engineEnabled && settings.bestArrow ? engine.lines[0]?.moves[0] : undefined;
  const arrows: Arrow[] = [
    ...node.arrows,
    ...(bestMove
      ? [
          {
            startSquare: bestMove.slice(0, 2),
            endSquare: bestMove.slice(2, 4),
            color: "rgba(22,101,52,0.7)",
          },
        ]
      : []),
  ];
  const chess = new Chess(node.fen);
  const status = chess.isCheckmate()
    ? "Checkmate"
    : chess.isDraw()
      ? "Draw"
      : `${chess.turn() === "w" ? "White" : "Black"} to move${chess.isCheck() ? " · check" : ""}`;
  return (
    <section className="academy-study">
      <div className="workspace-section-heading">
        <div>
          <span className="eyebrow">Prepare · explore · explain</span>
          <h2>{title}</h2>
        </div>
        <div className="academy-actions">
          <button onClick={() => setDialog("import")}>Import PGN</button>
          <button
            onClick={() =>
              download(
                "academy-study.pgn",
                exportStudy(study),
                "application/x-chess-pgn",
              )
            }
          >
            Export PGN
          </button>
          {onSave && (
            <button
              className="button button-dark"
              disabled={saving}
              onClick={async () => {
                setSaving(true);
                try {
                  if (await onSave(exportStudy(study)))
                    setNotice("Study saved.");
                } finally {
                  setSaving(false);
                }
              }}
            >
              Save study
            </button>
          )}
        </div>
      </div>
      <div className="academy-board-layout">
        <div>
          <div className="academy-board-status">
            <strong>{status}</strong>
            {deadline && (
              <span role="timer">
                {Math.floor(remaining / 60)}:
                {String(remaining % 60).padStart(2, "0")}
              </span>
            )}
          </div>
          <Chessboard
            options={{
              id: "academy-study-board",
              position: settings.pawns ? pawnPosition(node.fen) : node.fen,
              boardOrientation: settings.orientation,
              showNotation: settings.coordinates,
              darkSquareStyle: { backgroundColor: colors[0] },
              lightSquareStyle: { backgroundColor: colors[1] },
              allowDragging: !settings.pawns,
              arrows,
              onArrowsChange: ({ arrows }) =>
                setStudy(updateStudyNode(study, { arrows })),
              onPieceDrop: ({ sourceSquare, targetSquare }) =>
                !!targetSquare &&
                play({ from: sourceSquare, to: targetSquare, promotion }),
              onSquareClick: ({ square }) => {
                if (settings.pawns) return;
                if (selectedSquare && selectedSquare !== square)
                  play({ from: selectedSquare, to: square, promotion });
                else setSelected(square);
              },
              squareStyles: selectedSquare
                ? { [selectedSquare]: { boxShadow: "inset 0 0 0 4px #8C652D" } }
                : {},
            }}
          />
          <div className="academy-board-navigation">
            <button
              onClick={() => jump("root")}
              disabled={!node.parentId}
              aria-label="First position"
            >
              |◀
            </button>
            <button
              onClick={() => node.parentId && jump(node.parentId)}
              disabled={!node.parentId}
              aria-label="Previous move"
            >
              ◀
            </button>
            <button
              onClick={() => jump(node.children[0])}
              disabled={!node.children.length}
              aria-label="Next move"
            >
              ▶
            </button>
            <button
              onClick={last}
              disabled={!node.children.length}
              aria-label="Last move"
            >
              ▶|
            </button>
            <button
              onClick={() =>
                preferences({
                  orientation:
                    settings.orientation === "white" ? "black" : "white",
                })
              }
            >
              Flip board
            </button>
          </div>
          <form
            className="academy-inline-form"
            onSubmit={(event) => {
              const form = formValues(event);
              if (play(value(form, "move"))) event.currentTarget.reset();
            }}
          >
            <Field label="Enter a move">
              <input
                name="move"
                placeholder="e4 or e2e4"
                required
                autoComplete="off"
              />
            </Field>
            <Field label="Promotion">
              <select
                value={promotion}
                onChange={(event) => setPromotion(event.target.value)}
              >
                <option value="q">Queen</option>
                <option value="r">Rook</option>
                <option value="b">Bishop</option>
                <option value="n">Knight</option>
              </select>
            </Field>
            <button className="button button-dark">Play</button>
          </form>
          <div className="academy-actions">
            <button onClick={() => setDialog("position")}>
              Set up position
            </button>
            <button onClick={() => setDialog("settings")}>
              Board settings
            </button>
            <button
              onClick={() => {
                setStudy(newStudy());
                setNotice(
                  "New board. Saved studies are available in your library.",
                );
              }}
            >
              New board
            </button>
          </div>
          {onLoadPosition && (
            <button
              className="button button-dark"
              onClick={() =>
                onLoadPosition(
                  node.fen,
                  path.map((item) => item.uci),
                )
              }
            >
              Load this position into class
            </button>
          )}
        </div>
        <aside className="academy-analysis-panel">
          <div className="academy-tabs">
            <button
              className={!engineEnabled ? "is-active" : ""}
              onClick={() => setEngine(false)}
            >
              Moves
            </button>
            <button
              className={engineEnabled ? "is-active" : ""}
              onClick={() => setEngine(true)}
            >
              Engine
            </button>
          </div>
          {engineEnabled ? (
            <div className="academy-engine">
              <p>Evaluation is from White’s perspective.</p>
              <small>
                Stockfish 19 ·{" "}
                <a href="/engine/COPYING.txt" target="_blank" rel="noreferrer">
                  GPLv3 license
                </a>{" "}
                · <a href="/engine/stockfish-19-source.tar.gz">Source</a>
              </small>
              {engine.error && <p role="alert">{engine.error}</p>}
              {engine.lines.map((line) => (
                <article key={line.rank}>
                  <strong>{line.score}</strong>
                  <small>
                    Line {line.rank} · Depth {line.depth}
                  </small>
                  <p>{line.moves.join(" ")}</p>
                </article>
              ))}
              {!engine.lines.length && !engine.error && (
                <p role="status">Analyzing the position…</p>
              )}
            </div>
          ) : (
            settings.moves && (
              <div
                className="academy-move-tree"
                aria-label="Moves and variations"
              >
                {Object.values(study.nodes)
                  .filter((item) => item.parentId)
                  .map((item) => (
                    <button
                      key={item.id}
                      className={item.id === node.id ? "is-active" : ""}
                      onClick={() => jump(item.id)}
                      title={item.comment || undefined}
                    >
                      <small>
                        {study.nodes[item.parentId!].fen.split(" ")[5]}
                        {study.nodes[item.parentId!].fen.split(" ")[1] === "w"
                          ? "."
                          : "…"}
                      </small>{" "}
                      {item.san}
                      {study.nodes[item.parentId!].children.indexOf(item.id) >
                        0 && <span> variation</span>}
                    </button>
                  ))}
                {Object.keys(study.nodes).length === 1 && (
                  <p>
                    Play a move, load a lesson, or import a PGN. Return to an
                    earlier position to explore a variation.
                  </p>
                )}
              </div>
            )
          )}
          <Field label="Position notes">
            <textarea
              rows={4}
              value={node.comment}
              onChange={(event) =>
                setStudy(
                  updateStudyNode(study, { comment: event.target.value }),
                )
              }
              maxLength={5000}
              placeholder="What should the learner notice?"
            />
          </Field>
          <div className="academy-annotation-tools">
            <Field label="Annotation color">
              <select
                value={color}
                onChange={(event) => setColor(event.target.value)}
              >
                <option value="#b45309">Gold</option>
                <option value="#166534">Green</option>
                <option value="#1d4ed8">Blue</option>
                <option value="#b91c1c">Red</option>
              </select>
            </Field>
            <button
              onClick={() =>
                setStudy(
                  updateStudyNode(study, {
                    arrows: node.arrows.map((arrow) => ({ ...arrow, color })),
                  }),
                )
              }
            >
              Apply color
            </button>
            <button
              onClick={() => setStudy(updateStudyNode(study, { arrows: [] }))}
            >
              Clear arrows
            </button>
          </div>
          <small>
            Right-drag to draw an arrow; right-click to highlight a square.
          </small>
          <form
            className="academy-inline-form"
            onSubmit={(event) => {
              const form = formValues(event);
              setDeadline(Date.now() + Number(form.get("minutes")) * 60000);
            }}
          >
            <Field label="Timer (minutes)">
              <input
                name="minutes"
                type="number"
                min={1}
                max={180}
                defaultValue={5}
                required
              />
            </Field>
            <button>Start timer</button>
            {deadline && (
              <button type="button" onClick={() => setDeadline(null)}>
                Stop
              </button>
            )}
          </form>
        </aside>
      </div>
      {error && (
        <p role="alert" className="form-message form-message-error">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="workspace-notice">
          {notice}
        </p>
      )}
      {dialog === "import" && (
        <Modal title="Import a PGN" close={() => setDialog(null)}>
          <form
            className="academy-form"
            onSubmit={(event) => {
              const form = formValues(event);
              try {
                setStudy(importStudy(value(form, "pgn")));
                setDialog(null);
                setError("");
              } catch (cause) {
                setError(
                  cause instanceof Error ? cause.message : "Invalid PGN.",
                );
              }
            }}
          >
            <Field label="PGN text">
              <textarea name="pgn" rows={10} required />
            </Field>
            {error && <p role="alert">{error}</p>}
            <button className="button button-dark">Load game</button>
          </form>
        </Modal>
      )}
      {dialog === "position" && (
        <PositionEditor
          fen={node.fen}
          close={() => setDialog(null)}
          apply={(fen) => {
            setStudy(newStudy(fen));
            setError("");
          }}
        />
      )}
      {dialog === "settings" && (
        <Modal title="Board settings" close={() => setDialog(null)}>
          <div className="academy-form">
            {(["coordinates", "moves", "pawns", "bestArrow"] as const).map(
              (key) => (
                <label className="academy-check" key={key}>
                  <input
                    type="checkbox"
                    checked={settings[key]}
                    onChange={(event) =>
                      preferences({ [key]: event.target.checked })
                    }
                  />
                  {
                    {
                      coordinates: "Show coordinates",
                      moves: "Show move list",
                      pawns: "Show pawn structure only",
                      bestArrow: "Show engine best-move arrow",
                    }[key]
                  }
                </label>
              ),
            )}
            <Field label="Board appearance">
              <select
                value={settings.theme}
                onChange={(event) => preferences({ theme: event.target.value })}
              >
                {Object.keys(themes).map((theme) => (
                  <option key={theme}>{theme}</option>
                ))}
              </select>
            </Field>
            <p>
              Appearance preferences are saved on this device. Save your study
              to retain its moves and comments.
            </p>
          </div>
        </Modal>
      )}
    </section>
  );
}
