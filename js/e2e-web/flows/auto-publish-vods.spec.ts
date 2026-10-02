import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Automatic VOD publishing is the autoPublishVods field of the account's
// place.stream.server.settings record for this node. Flip it in Privacy &
// Security and back, reloading each time so only the saved record can set the
// toggle. The harness node has no beta-invite issuer, so the test account is
// in the VOD beta that gates the toggle. It records no streams (no S3), so
// publishing itself is covered by the Go tests in pkg/statedb and
// pkg/atproto. Mobile: .maestro/logged-in/auto-publish-vods.yaml.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

async function setSetting(page: Page, testId: string, on: boolean) {
  const toggle = page.getByTestId(testId);
  const saved = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(
      "/xrpc/com.atproto.repo.putRecord",
    ),
  );
  await toggle.click();
  const response = await saved;
  expect(response.ok(), `putRecord returned ${response.status()}`).toBe(true);
  await expect(toggle).toHaveAttribute("aria-checked", String(on));

  await page.reload();
  await expect(page.getByTestId(testId)).toHaveAttribute(
    "aria-checked",
    String(on),
    { timeout: 30_000 },
  );
}

test("auto-publish-vods: recording opt-out survives another toggle", async ({
  page,
}) => {
  await loginThroughPds(page);

  await page.goto(`${HTTPS_URL}/settings/privacy`);
  const toggle = page.getByTestId("settings-auto-publish-vods");
  await expect(toggle).toBeVisible({ timeout: 30_000 });
  await expect(toggle).toHaveAttribute("aria-checked", /^(true|false)$/);
  const initial = (await toggle.getAttribute("aria-checked")) === "true";

  await setSetting(page, "settings-auto-publish-vods", !initial);
  await setSetting(page, "settings-auto-publish-vods", initial);

  const recording = page.getByTestId("settings-livestream-recording");
  await expect(recording).toBeVisible();
  const recordingInitiallyOn =
    (await recording.getAttribute("aria-checked")) === "true";
  if (!recordingInitiallyOn) {
    await setSetting(page, "settings-livestream-recording", true);
  }

  // An opt-out PUT must finish before another setting can be saved. A second
  // PUT based on the old cached record would silently re-enable recording.
  let releasePut!: () => void;
  const blockedPut = new Promise<void>((resolve) => {
    releasePut = resolve;
  });
  let signalPut!: () => void;
  const putStarted = new Promise<void>((resolve) => {
    signalPut = resolve;
  });
  await page.route(
    "**/xrpc/com.atproto.repo.putRecord",
    async (route) => {
      signalPut();
      await blockedPut;
      await route.continue();
    },
    { times: 1 },
  );
  const saved = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(
      "/xrpc/com.atproto.repo.putRecord",
    ),
  );
  await recording.click();
  try {
    await putStarted;
    await expect(toggle).toBeDisabled();
  } finally {
    releasePut();
  }
  const response = await saved;
  expect(response.ok(), `putRecord returned ${response.status()}`).toBe(true);
  await expect(toggle).toBeEnabled();
  await setSetting(page, "settings-auto-publish-vods", !initial);
  await expect(recording).toHaveAttribute("aria-checked", "false");

  await setSetting(page, "settings-auto-publish-vods", initial);
  if (recordingInitiallyOn) {
    await setSetting(page, "settings-livestream-recording", true);
  }
});
