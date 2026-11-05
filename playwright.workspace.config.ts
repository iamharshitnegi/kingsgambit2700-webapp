import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./tests/workspace",
  timeout: 120_000,
  outputDir: "test-results-workspace",
  workers: 1,
  retries: 0,
  reporter: "list",
  use: { baseURL: "http://localhost:3005", channel: process.env.PLAYWRIGHT_CHANNEL, actionTimeout: 15000, trace: "retain-on-failure", screenshot: "only-on-failure" },
});
