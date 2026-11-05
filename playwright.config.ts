import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: "http://127.0.0.1:3000",
    trace: "on-first-retry",
  },
  webServer: {
    command: "NEXT_PUBLIC_SITE_URL=http://127.0.0.1:3000 NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:3000 npm run dev -- --webpack --hostname 127.0.0.1",
    // Use public-content fallbacks for SSR; browser API calls use test fixtures.
    env: { API_BASE_URL: "http://127.0.0.1:9", EDITORIAL_CONTENT_ENABLED: "false", NEXT_PUBLIC_DEMO_MODE: "false" },
    url: "http://127.0.0.1:3000",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
  projects: [
    { name: "chromium", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile-chromium", use: { ...devices["Pixel 7"] } },
  ],
});
