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
  await page.getByTestId("desktop-save").click();
  await expect(page.getByText("Saved").first()).toBeVisible();
}

// codecs are the video codecs the plugin says it can send.
async function clientConfiguration(page: Page, codecs = ["h264"]) {
  const response = await page.request.post(
    `${SERVER_URL}/api/ingest/client-configuration`,
    {
      data: {
        authentication: STREAM_KEY,
        client: { supported_codecs: codecs },
      },
    },
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

  await expect
    .poll(() => clientConfiguration(page), { timeout: 10_000 })
    .toMatchObject({
      status: { result: "success" },
      encoder_configurations: [
        {
          codec: "h264",
          width: 1920,
          height: 1080,
          framerate: { numerator: 30, denominator: 1 },
          settings: {
            bitrate: 4500,
            rate_control: "CBR",
            keyint_sec: 1,
            bf: 0,
          },
        },
      ],
      audio_configurations: {
        live: [
          {
            codec: "aac",
            track_id: 0,
            channels: 2,
            settings: { bitrate: 160 },
          },
        ],
      },
    });

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
  await expect
    .poll(async () => {
      const config = await clientConfiguration(page);
      return [config.encoder_configurations, config.audio_configurations];
    })
    .toEqual([undefined, undefined]);
});

test("desktop: AV1 is encoded first, with an H.264 fallback", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${HTTPS_URL}/desktop`);
  await expect(page.getByTestId("desktop-video")).toBeVisible({
    timeout: 30_000,
  });

  await toggle(page, "desktop-video", true);
  await page.getByText("1080p", { exact: true }).click();
  await page.getByText("2160p", { exact: true }).click();
  await page.getByText("AV1", { exact: true }).click();
  await page.getByText("120 fps", { exact: true }).click();
  await page.getByTestId("desktop-video-bitrate").fill("20000");
  await save(page);

  const settings = (bitrate: number) => ({
    bitrate,
    rate_control: "CBR",
    keyint_sec: 1,
    bf: 0,
  });
  const av1 = {
    codec: "av1",
    width: 3840,
    height: 2160,
    framerate: { numerator: 120, denominator: 1 },
    settings: settings(20000),
  };
  const av1Plugin = async () =>
    (await clientConfiguration(page, ["h264", "av1"])).encoder_configurations;
  await expect.poll(av1Plugin).toMatchObject([
    av1,
    {
      codec: "h264",
      width: 1920,
      height: 1080,
      framerate: { numerator: 60, denominator: 1 },
      settings: settings(6000),
    },
  ]);

  // Kept across a reload; back to H.264 alone, the fallback goes with it.
  await page.reload();
  await expect(page.getByText("Also H.264 1080p")).toBeVisible({
    timeout: 30_000,
  });
  await page.getByText("H.264", { exact: true }).click();
  await save(page);
  await expect.poll(av1Plugin).toMatchObject([{ ...av1, codec: "h264" }]);

  await toggle(page, "desktop-video", false);
  await save(page);
});
