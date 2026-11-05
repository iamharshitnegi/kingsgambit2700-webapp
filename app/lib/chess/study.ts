import { Chess, DEFAULT_POSITION } from "chess.js";

export type StudyArrow = {
  startSquare: string;
  endSquare: string;
  color: string;
};
export type StudyNode = {
  id: string;
  parentId: string | null;
  fen: string;
  san: string;
  uci: string;
  children: string[];
  comment: string;
  arrows: StudyArrow[];
};
export type Study = {
  headers: Record<string, string>;
  nodes: Record<string, StudyNode>;
  currentId: string;
};

export function newStudy(fen = DEFAULT_POSITION): Study {
  const game = new Chess(fen);
  return {
    headers: { Event: "Academy study", Result: "*" },
    currentId: "root",
    nodes: {
      root: {
        id: "root",
        parentId: null,
        fen: game.fen(),
        san: "",
        uci: "",
        children: [],
        comment: "",
        arrows: [],
      },
    },
  };
}

export function addStudyMove(
  study: Study,
  move: string | { from: string; to: string; promotion?: string },
): Study {
  if (Object.keys(study.nodes).length >= 10000)
    throw new Error("A study can contain up to 10,000 moves.");
  const parent = study.nodes[study.currentId];
  const game = new Chess(parent.fen);
  const played = game.move(move);
  const uci = played.from + played.to + (played.promotion || "");
  const existing = parent.children.find((id) => study.nodes[id].uci === uci);
  if (existing) return { ...study, currentId: existing };
  const id = `move-${Object.keys(study.nodes).length}`;
  const node: StudyNode = {
    id,
    parentId: parent.id,
    fen: game.fen(),
    san: played.san,
    uci,
    children: [],
    comment: "",
    arrows: [],
  };
  return {
    ...study,
    currentId: id,
    nodes: {
      ...study.nodes,
      [id]: node,
      [parent.id]: { ...parent, children: [...parent.children, id] },
    },
  };
}

export function updateStudyNode(
  study: Study,
  change: Partial<Pick<StudyNode, "comment" | "arrows">>,
): Study {
  return {
    ...study,
    nodes: {
      ...study.nodes,
      [study.currentId]: { ...study.nodes[study.currentId], ...change },
    },
  };
}

export function importStudy(pgn: string): Study {
  if (pgn.length > 5_000_000)
    throw new Error("PGN files must be smaller than 5 MB.");
  const headers: Record<string, string> = {};
  const body = pgn.replace(
    /^\s*\[([A-Za-z][A-Za-z0-9_]*)\s+"((?:\\.|[^"\\])*)"\]\s*$/gm,
    (_, key: string, value: string) => {
      headers[key] = value.replace(/\\(["\\])/g, "$1");
      return "";
    },
  );
  let study = newStudy(headers.FEN || DEFAULT_POSITION);
  study.headers = { ...study.headers, ...headers };
  const tokens: string[] = [];
  for (let at = 0; at < body.length;) {
    if (/\s/.test(body[at])) {
      at++;
      continue;
    }
    if (body[at] === ";") {
      const end = body.indexOf("\n", at);
      at = end === -1 ? body.length : end + 1;
      continue;
    }
    if (body[at] === "{") {
      const end = body.indexOf("}", at + 1);
      if (end === -1 || body.slice(at + 1, end).includes("{"))
        throw new Error("PGN contains an unclosed or nested comment.");
      tokens.push(body.slice(at, end + 1));
      at = end + 1;
      continue;
    }
    if (body[at] === "}")
      throw new Error("PGN contains an unmatched comment delimiter.");
    if (body[at] === "(" || body[at] === ")") {
      tokens.push(body[at++]);
      continue;
    }
    const token = body.slice(at).match(/^[^\s(){};]+/)?.[0];
    if (!token) throw new Error("PGN contains invalid text.");
    tokens.push(token);
    at += token.length;
  }
  const stack: string[] = [];
  for (let token of tokens) {
    if (token === "(") {
      if (stack.length >= 64 || study.currentId === "root")
        throw new Error("Invalid or excessively nested PGN variation.");
      stack.push(study.currentId);
      study = { ...study, currentId: study.nodes[study.currentId].parentId! };
    } else if (token === ")") {
      const resume = stack.pop();
      if (!resume)
        throw new Error("PGN contains an unmatched closing variation.");
      study = { ...study, currentId: resume };
    } else if (token.startsWith("{")) {
      const arrows = [...study.nodes[study.currentId].arrows];
      const comment = token
        .slice(1, -1)
        .replace(
          /\[%(cal|csl) ([^\]]+)\]/g,
          (_, kind: string, text: string) => {
            for (const annotation of text.split(",")) {
              const match = annotation
                .trim()
                .match(
                  kind === "cal"
                    ? /^([GRBY])([a-h][1-8])([a-h][1-8])$/
                    : /^([GRBY])([a-h][1-8])$/,
                );
              if (match)
                arrows.push({
                  startSquare: match[2],
                  endSquare: match[3] || match[2],
                  color: annotationColors[match[1]],
                });
            }
            return "";
          },
        )
        .trim();
      study = updateStudyNode(study, {
        arrows,
        comment: [study.nodes[study.currentId].comment, comment]
          .filter(Boolean)
          .join(" "),
      });
    } else {
      token = token
        .replace(/^\d+\.(?:\.\.)?/, "")
        .replace(/\$\d+/g, "")
        .replace(/[!?]+$/g, "");
      if (!token || token === "..." || /^(1-0|0-1|1\/2-1\/2|\*)$/.test(token))
        continue;
      try {
        study = addStudyMove(study, token);
      } catch {
        throw new Error(`The PGN contains an illegal move: ${token}.`);
      }
    }
  }
  if (stack.length) throw new Error("PGN contains an unclosed variation.");
  return { ...study, currentId: "root" };
}

const annotationColors: Record<string, string> = {
  G: "#166534",
  R: "#b91c1c",
  B: "#1d4ed8",
  Y: "#b45309",
};
function commentText(node: StudyNode) {
  const parts = [node.comment.replace(/[{}]/g, "")];
  for (const kind of ["cal", "csl"]) {
    const arrows = node.arrows.filter((arrow) =>
      kind === "cal"
        ? arrow.startSquare !== arrow.endSquare
        : arrow.startSquare === arrow.endSquare,
    );
    if (arrows.length)
      parts.push(
        `[%${kind} ${arrows.map((arrow) => `${Object.entries(annotationColors).find(([, color]) => color === arrow.color)?.[0] || "Y"}${arrow.startSquare}${kind === "cal" ? arrow.endSquare : ""}`).join(",")}]`,
      );
  }
  return parts.some(Boolean) ? ` {${parts.filter(Boolean).join(" ")}}` : "";
}
function nodeText(study: Study, node: StudyNode) {
  const before = study.nodes[node.parentId!].fen.split(" ");
  const number = `${before[5]}${before[1] === "w" ? "." : "..."}`;
  return `${number} ${node.san}${commentText(node)}`;
}

export function exportStudy(study: Study): string {
  // Iterative traversal avoids overflowing the stack on long study files.
  const words: string[] = [commentText(study.nodes.root).trim()].filter(
    Boolean,
  );
  const pending: Array<string | { parent: string }> = [{ parent: "root" }];
  while (pending.length) {
    const item = pending.pop()!;
    if (typeof item === "string") {
      words.push(item);
      continue;
    }
    const children = study.nodes[item.parent].children;
    if (!children.length) continue;
    const main = study.nodes[children[0]];
    pending.push({ parent: main.id });
    for (let index = children.length - 1; index >= 1; index--) {
      const alternate = study.nodes[children[index]];
      pending.push(
        ")",
        { parent: alternate.id },
        nodeText(study, alternate),
        "(",
      );
    }
    pending.push(nodeText(study, main));
  }
  const headers = { ...study.headers };
  if (study.nodes.root.fen !== DEFAULT_POSITION) {
    headers.SetUp = "1";
    headers.FEN = study.nodes.root.fen;
  } else {
    delete headers.SetUp;
    delete headers.FEN;
  }
  const tags = Object.entries(headers)
    .map(
      ([key, value]) =>
        `[${key} "${value
          .replace(/\\/g, "\\\\")
          .replace(/"/g, '\\"')
          .replace(/[\r\n]/g, " ")}"]`,
    )
    .join("\n");
  return `${tags}\n\n${words.join(" ")} ${headers.Result || "*"}`;
}

export function studyPath(study: Study): StudyNode[] {
  const path: StudyNode[] = [];
  let node = study.nodes[study.currentId];
  while (node.parentId) {
    path.unshift(node);
    node = study.nodes[node.parentId];
  }
  return path;
}

export function pawnPosition(fen: string) {
  const game = new Chess(fen);
  return Object.fromEntries(
    game
      .board()
      .flat()
      .filter((piece) => piece?.type === "p")
      .map((piece) => [piece!.square, { pieceType: `${piece!.color}P` }]),
  );
}
