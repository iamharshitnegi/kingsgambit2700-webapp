// Isolated production-mode browser harness. Requires the local Compose infrastructure.
import { spawn, spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { readdir, readFile, rm } from "node:fs/promises";
import { fileURLToPath } from "node:url";
const root = fileURLToPath(new URL("../", import.meta.url));
const databaseURL =
  process.env.KG_TEST_DATABASE_URL || process.env.KG_DATABASE_URL;
if (
  !databaseURL ||
  !["localhost", "127.0.0.1"].includes(new URL(databaseURL).hostname)
)
  throw new Error(
    "Set KG_TEST_DATABASE_URL to a local disposable PostgreSQL database.",
  );
const connection = new URL(databaseURL);
let created = false;
let redisCreated = false;
const schema = `kg_browser_${randomBytes(8).toString("hex")}`;
const redisName = `${schema}_redis`;
const children = [];
function sql(input, scoped = false) {
  const result = spawnSync("psql", ["-X", "-q", "-v", "ON_ERROR_STOP=1"], {
    cwd: root,
    env: {
      ...process.env,
      PGHOST: connection.hostname,
      PGPORT: connection.port || "5432",
      PGUSER: decodeURIComponent(connection.username),
      PGPASSWORD: decodeURIComponent(connection.password),
      PGDATABASE: connection.pathname.slice(1),
      ...(scoped ? { PGOPTIONS: `-c search_path=${schema},public` } : {}),
    },
    input,
    encoding: "utf8",
  });
  if (result.status !== 0)
    throw new Error(result.stderr.replaceAll(databaseURL, "[test database]"));
}
function launch(cmd, args, env) {
  const child = spawn(cmd, args, {
    cwd: root,
    env: { ...process.env, ...env },
    stdio: ["ignore", "inherit", "inherit"],
  });
  children.push(child);
  return child;
}
async function waitFor(url, child) {
  const deadline = Date.now() + 120000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null || child.signalCode)
      throw new Error(`Service exited before startup: ${url}`);
    try {
      if ((await fetch(url)).ok) return;
    } catch {}
    await new Promise((resolve) => setTimeout(resolve, 300));
  }
  throw new Error(`Service did not start: ${url}`);
}
async function cleanup() {
  for (const child of children.reverse()) {
    child.kill("SIGTERM");
    await Promise.race([
      new Promise((resolve) => child.once("exit", resolve)),
      new Promise((resolve) => setTimeout(resolve, 2000)),
    ]);
  }
  try {
    if (created) sql(`DROP SCHEMA IF EXISTS ${schema} CASCADE;`);
  } finally {
    if (redisCreated) spawnSync("docker", ["stop", redisName], { stdio: "ignore" });
    await rm(`/tmp/kg-workspace-mfa-${schema}.json`, { force: true });
  }
}
try {
  const redis = spawnSync(
    "docker",
    [
      "run",
      "--rm",
      "-d",
      "--name",
      redisName,
      "-p",
      "127.0.0.1::6379",
      "redis:8.2.8-alpine",
      "redis-server",
      "--save",
      "",
      "--appendonly",
      "no",
    ],
    { encoding: "utf8" },
  );
  if (redis.status !== 0)
    throw new Error(`Could not start isolated test Redis: ${redis.stderr}`);
  redisCreated = true;
  const redisAddress = spawnSync("docker", ["port", redisName, "6379/tcp"], {
    encoding: "utf8",
  }).stdout.trim();
  if (!/^127\.0\.0\.1:\d+$/.test(redisAddress))
    throw new Error("Isolated Redis did not expose its local port.");
  let redisReady = false;
  for (let attempt = 0; attempt < 40; attempt++) {
    if (
      spawnSync("docker", ["exec", redisName, "redis-cli", "ping"], {
        encoding: "utf8",
      }).stdout.trim() === "PONG"
    ) {
      redisReady = true;
      break;
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  if (!redisReady) throw new Error("Isolated Redis did not become ready.");
  sql(`CREATE SCHEMA ${schema};`);
  created = true;
  const files = (
    await readdir(new URL("../backend/migrations/", import.meta.url))
  )
    .filter((name) => name.endsWith(".up.sql"))
    .sort();
  for (const name of files)
    sql(
      await readFile(
        new URL(`../backend/migrations/${name}`, import.meta.url),
        "utf8",
      ),
      true,
    );
  sql(
    "INSERT INTO users (id,email,display_name,status,created_at,updated_at) VALUES ('01990000-0000-7000-8000-000000000099','browser-admin@example.test','Browser Admin','active',now(),now()); INSERT INTO role_grants (id,user_id,role,granted_at) VALUES ('01990000-0000-7000-8000-000000000098','01990000-0000-7000-8000-000000000099','super_admin',now());",
    true,
  );
  const scoped = new URL(databaseURL);
  scoped.searchParams.set("search_path", `${schema},public`);
  for (const target of ["api", "realtime"]) {
    const build = spawnSync(
      "go",
      ["build", "-o", `/tmp/kg-workspace-e2e-${target}`, `./cmd/${target}`],
      {
        cwd: `${root}/backend`,
        env: { ...process.env, GOCACHE: "/tmp/kg-go-cache" },
        stdio: "inherit",
      },
    );
    if (build.status !== 0) throw new Error(`${target} build failed.`);
  }
  const serviceEnv = {
    KG_REDIS_URL: `redis://${redisAddress}/0`,
    KG_DATABASE_URL: scoped.toString(),
    KG_DATABASE_DIRECT_URL: scoped.toString(),
    KG_ENV: "development",
    KG_API_ADDR: "127.0.0.1:8085",
    KG_REALTIME_ADDR: "127.0.0.1:8086",
    KG_PUBLIC_API_URL: "http://localhost:8085",
    KG_PUBLIC_WEB_URL: "http://localhost:3005",
    KG_ALLOWED_ORIGINS: "http://localhost:3005",
    KG_COOKIE_NAME: "kg_e2e_session",
    KG_COOKIE_SECURE: "false",
    KG_EMAIL_PROVIDER: "smtp",
    KG_SMTP_HOST: "localhost",
    KG_SMTP_PORT: "1025",
    KG_SESSION_IDLE_TTL: "30m",
  };
  const api = launch("/tmp/kg-workspace-e2e-api", [], serviceEnv);
  const realtime = launch("/tmp/kg-workspace-e2e-realtime", [], serviceEnv);
  const web = launch(
    "node",
    [
      "node_modules/next/dist/bin/next",
      "start",
      "--hostname",
      "localhost",
      "--port",
      "3005",
    ],
    {
      KG_NEXT_DIST_DIR: ".next-workspace",
      NEXT_PUBLIC_DEMO_MODE: "false",
      NEXT_PUBLIC_SITE_URL: "http://localhost:3005",
      NEXT_PUBLIC_API_BASE_URL: "http://localhost:8085",
      NEXT_PUBLIC_REALTIME_URL: "ws://localhost:8086/ws/v1",
      API_BASE_URL: "http://localhost:8085",
      EDITORIAL_CONTENT_ENABLED: "false",
    },
  );
  await Promise.all([
    waitFor("http://localhost:8085/health/live", api),
    waitFor("http://localhost:8086/health/live", realtime),
    waitFor("http://localhost:3005/sign-in", web),
  ]);
  console.log(`Isolated workspace running (${schema}).`);
  if (process.argv.includes("--serve"))
    await new Promise((resolve) => {
      process.once("SIGTERM", resolve);
      process.once("SIGINT", resolve);
    });
  else {
    const test = launch(
      "node",
      [
        "node_modules/@playwright/test/cli.js",
        "test",
        "--config=playwright.workspace.config.ts",
      ],
      { KG_BROWSER_SCHEMA: schema },
    );
    process.exitCode = await new Promise((resolve) =>
      test.once("exit", (code) => resolve(code || 0)),
    );
  }
} finally {
  await cleanup();
}
