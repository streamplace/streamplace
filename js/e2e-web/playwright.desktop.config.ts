import { defineConfig } from "@playwright/test";

// Desktop owns its flows: no web global setup, storage state, or live stream.
export default defineConfig({
  testDir: "./desktop",
  outputDir: "desktop-test-results",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  timeout: 180_000,
  expect: { timeout: 30_000 },
  reporter: process.env.CI
    ? [
        ["list"],
        ["html", { outputFolder: "desktop-playwright-report", open: "never" }],
      ]
    : [["list"]],
  projects: [{ name: "electron" }],
});
