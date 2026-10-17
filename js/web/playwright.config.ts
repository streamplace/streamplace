import { defineConfig, devices } from "@playwright/test";

// Playwright e2e config for the Streamplace web app.
//
// The tests assume a Streamplace server is running with the web frontend
// enabled. Start it with `make dev-web` (which proxies to Vite dev server
// on :5173) or point STREAMPLACE_E2E_BASE_URL at a running instance.
//
// Run: pnpm e2e
// Run with UI: pnpm e2e:ui

const BASE_URL =
  process.env.STREAMPLACE_E2E_BASE_URL ?? "http://127.0.0.1:5173";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,
  reporter: process.env.CI ? "github" : "html",
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
