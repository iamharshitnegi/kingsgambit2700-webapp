import { copyFile, mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
const root = new URL("../", import.meta.url);
const directory = new URL("public/engine/", root);
await mkdir(directory, { recursive: true });
for (const name of [
  "stockfish-19-lite-single.js",
  "stockfish-19-lite-single.wasm",
]) {
  await copyFile(
    new URL(`node_modules/stockfish/bin/${name}`, root),
    new URL(name, directory),
  );
}
await copyFile(
  new URL("node_modules/stockfish/Copying.txt", root),
  new URL("COPYING.txt", directory),
);
console.log(
  `Stockfish 19 lite single-thread assets prepared in ${fileURLToPath(directory)}`,
);
