import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/demo",
  outputDir: "test-results/demo",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: "list",
  use: { baseURL: "http://127.0.0.1:3001", trace: "retain-on-failure", ...(process.env.PLAYWRIGHT_EXECUTABLE_PATH ? { launchOptions: { executablePath: process.env.PLAYWRIGHT_EXECUTABLE_PATH } } : {}) },
  webServer: {
    command: "KG_NEXT_DIST_DIR=.next-demo npm run dev:demo -- --hostname 127.0.0.1 --port 3001",
    url: "http://127.0.0.1:3001/demo",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [{ name: "demo-desktop", use: { ...devices["Desktop Chrome"] } }, { name: "demo-mobile", use: { ...devices["Pixel 7"] } }],
});
