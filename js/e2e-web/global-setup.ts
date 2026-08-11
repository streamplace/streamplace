import { chromium, expect, type FullConfig } from "@playwright/test";
import { STORAGE_STATE } from "./storage";

// Web counterpart to .maestro/00-server-setup.yaml.
//
// The mobile app ships pointed at production, so the Maestro suite opens
// Settings -> Advanced and enters the test node's URL. The web app served by
// the harness is the same react-native codebase, so we do the same thing —
// once, here — and persist the result (redux-persist writes it to
// localStorage) via storageState so every flow starts already pointed at the
// harness. react-navigation's linking config exposes the advanced-settings
// screen at /settings/advanced, so we can deep-link straight to it instead of
// clicking through the nav.
export default async function globalSetup(_config: FullConfig) {
  const SERVER_URL = process.env.SERVER_URL;
  if (!SERVER_URL) {
    throw new Error("SERVER_URL is not set — start the harness first");
  }

  const browser = await chromium.launch();
  const page = await browser.newPage();
  try {
    await page.goto(`${SERVER_URL}/settings/advanced`);

    // toggle "use custom node" on, which reveals the URL field + save button
    const toggle = page.getByTestId("settings-use-custom-node");
    await expect(toggle).toBeVisible({ timeout: 60_000 });
    await toggle.click();

    // the react-native-web TextInput renders as an <input>; fill it
    const urlField = page
      .locator(
        '[data-testid="settings-custom-node-url"], [data-testid="settings-custom-node-url"] input',
      )
      .first();
    await expect(urlField).toBeVisible();
    await urlField.fill(SERVER_URL);

    await page.getByTestId("settings-save-node").click();

    // The harness WHIP stream takes a few seconds after boot to ingest its
    // first segments and register as live. The home feed fetches getLiveUsers
    // on mount and doesn't aggressively re-poll, so wait for the stream to be
    // live *before* loading the feed — otherwise the flows race the stream and
    // see an empty "no one is streaming" feed.
    await waitForLiveStream(SERVER_URL);

    // prove the app is now talking to the test node: its isolated firehose only
    // has our looping test stream, so a card must appear on the home feed
    await page.goto(`${SERVER_URL}/`);
    await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
      timeout: 30_000,
    });

    await page.context().storageState({ path: STORAGE_STATE });
  } finally {
    await browser.close();
  }
}

// Poll getLiveUsers until the harness's looping test stream shows up (it needs
// a few seconds after boot to ingest its first segments).
async function waitForLiveStream(serverUrl: string, timeoutMs = 90_000) {
  const deadline = Date.now() + timeoutMs;
  let lastErr = "";
  while (Date.now() < deadline) {
    try {
      // limit is required in practice: the endpoint returns an empty result
      // when it's omitted (the app always passes one).
      const res = await fetch(
        `${serverUrl}/xrpc/place.stream.live.getLiveUsers?limit=10`,
      );
      if (res.ok) {
        const body = (await res.json()) as { streams?: unknown[] };
        if (Array.isArray(body.streams) && body.streams.length > 0) return;
      } else {
        lastErr = `status ${res.status}`;
      }
    } catch (e) {
      lastErr = (e as Error).message;
    }
    await new Promise((r) => setTimeout(r, 2000));
  }
  throw new Error(
    `timed out waiting for a live stream from ${serverUrl} (last: ${lastErr || "empty feed"})`,
  );
}
