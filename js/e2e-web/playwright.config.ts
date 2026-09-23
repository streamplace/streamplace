import { defineConfig, devices } from "@playwright/test";
import { STORAGE_STATE } from "./storage";

// The web app under test is served by a `streamplace e2e` harness (see
// pkg/cmd/e2e.go), which prints SERVER_URL / ACCOUNT_HANDLE / ACCOUNT_DID.
// hack/e2e-web-local.sh starts that harness and exports SERVER_URL before
// invoking playwright. global-setup then points the app at that node (see
// global-setup.ts), which is where a missing SERVER_URL is reported — we don't
// throw here so the config stays importable by static tooling (knip, etc.).
const SERVER_URL = process.env.SERVER_URL;

export default defineConfig({
  testDir: "./flows",
  // Point the app at the test node once (Settings -> Advanced), then reuse that
  // browser state so every flow starts already configured — the web analogue of
  // .maestro/00-server-setup.yaml.
  globalSetup: require.resolve("./global-setup"),
  // Flows share one harness (one account, one looping stream) and are written
  // to mirror the ordered .maestro/ suite, so run them serially.
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI
    ? [["list"], ["html", { open: "never" }]]
    : [["list"]],
  timeout: 90_000,
  expect: { timeout: 30_000 },
  use: {
    baseURL: SERVER_URL,
    storageState: STORAGE_STATE,
    headless: true,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
    // The app is served over http on loopback with a self-test stream; don't
    // let cert/mixed-content strictness get in the way.
    ignoreHTTPSErrors: true,
    // WebRTC playback needs a permissive autoplay/media posture.
    launchOptions: {
      args: [
        "--autoplay-policy=no-user-gesture-required",
        "--use-fake-ui-for-media-stream",
      ],
    },
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
