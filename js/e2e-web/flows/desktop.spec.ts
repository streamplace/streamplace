import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Streamplace Desktop's screen (/desktop) keeps the renditions OBS encodes as
// records on the node (place.stream.local.*), and the node hands them to the
// OBS plugin when it asks how to stream (/api/ingest/client-configuration,
// the request the plugin sends with the stream key). The plugin's own use of
// the answer is C (obs-plugin/); the OBS card needs Electron and is covered by
// js/desktop's tests.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const SERVER_URL = process.env.SERVER_URL!;
const STREAM_KEY = process.env.STREAM_KEY!;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

async function toggle(page: Page, testId: string, on: boolean) {
  const row = page.getByTestId(testId);
  if ((await row.getAttribute("aria-checked")) !== String(on)) {
    await row.click();
  }
  await expect(row).toHaveAttribute("aria-checked", String(on));
}

async function save(page: Page) {
  const saved = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(
      "/xrpc/place.stream.local.deleteRecord",
    ),
  );
  await page.getByTestId("desktop-save").click();
  expect((await saved).ok()).toBe(true);
  await expect(page.getByText("Saved").first()).toBeVisible();
}

async function clientConfiguration(page: Page) {
  const response = await page.request.post(
    `${SERVER_URL}/api/ingest/client-configuration`,
    { data: { authentication: STREAM_KEY } },
  );
  expect(response.ok()).toBe(true);
  return response.json();
}

test("desktop: OBS encodes the renditions set on the node", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${HTTPS_URL}/desktop`);
  await expect(page.getByTestId("desktop-video")).toBeVisible({
    timeout: 30_000,
  });

  await toggle(page, "desktop-video", true);
  await page.getByText("30 fps").click();
  await page.getByTestId("desktop-video-bitrate").fill("4500");
  await toggle(page, "desktop-audio-aac", true);
  await toggle(page, "desktop-audio-opus", false);
  await save(page);

  const config = await clientConfiguration(page);
  expect(config.status.result).toBe("success");
  expect(config.encoder_configurations).toEqual([
    {
      codec: "h264",
      width: 1920,
      height: 1080,
      framerate: { numerator: 30, denominator: 1 },
      settings: { bitrate: 4500, rate_control: "CBR", keyint_sec: 1, bf: 0 },
    },
  ]);
  expect(config.audio_configurations.live).toEqual([
    { codec: "aac", track_id: 0, channels: 2, settings: { bitrate: 160 } },
  ]);

  // Kept on the node, so they come back after a reload.
  await page.reload();
  await expect(page.getByTestId("desktop-video")).toHaveAttribute(
    "aria-checked",
    "true",
    { timeout: 30_000 },
  );
  await expect(page.getByTestId("desktop-video-bitrate")).toHaveValue("4500");
  await expect(page.getByTestId("desktop-audio-opus")).toHaveAttribute(
    "aria-checked",
    "false",
  );

  // Turned off, OBS keeps its own encoders again.
  await toggle(page, "desktop-video", false);
  await toggle(page, "desktop-audio-aac", false);
  await save(page);
  const cleared = await clientConfiguration(page);
  expect(cleared.encoder_configurations).toBeUndefined();
  expect(cleared.audio_configurations).toBeUndefined();
});
