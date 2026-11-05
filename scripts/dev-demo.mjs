import { spawn } from "node:child_process";

const child = spawn(process.execPath, ["node_modules/next/dist/bin/next", "dev", "--webpack", ...process.argv.slice(2)], {
  stdio: "inherit",
  env: { ...process.env, NEXT_PUBLIC_DEMO_MODE: "true" },
});
for (const signal of ["SIGINT", "SIGTERM"]) process.on(signal, () => child.kill(signal));
child.on("exit", (code) => process.exit(code ?? 0));
child.on("error", (error) => { console.error(error.message); process.exit(1); });
