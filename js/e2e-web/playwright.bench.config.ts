import { defineConfig, devices } from "@playwright/test";
import base from "./playwright.config";

// Benchmarks own their setup (no live stream or shared storage state), so the
// harness can run them with SP_E2E_INGEST=none. Traces, videos and
// screenshots stay off: they would skew the timings.
export default defineConfig({
  testDir: "./bench",
  outputDir: "bench-test-results",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  timeout: 30 * 60_000,
  expect: base.expect,
  reporter: [["list"]],
  use: {
    ...base.use,
    storageState: undefined,
    trace: "off",
    video: "off",
    screenshot: "off",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
