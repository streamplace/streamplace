import { expect, test } from "@playwright/test";
import { uploadFixture } from "./upload-fixture";

// VOD links carry a playback start position as `?t=` (YouTube-style: plain
// seconds, or an `1h2m3s` duration). Point it at a video with real media and
// the player must open there instead of at 0:00.
//
// The harness's own test VOD has no source tracks, so this uploads the file
// the harness is streaming (E2E_FIXTURE_MP4) through the app's own /upload
// workflow first — that is what gives the player something seekable. The
// upload needs a signed-in user, so this needs the harness's HTTPS mode, like
// the other OAuth flows.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const ACCOUNT_DID = process.env.ACCOUNT_DID;
const START_SECONDS = 151;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("vod-start-time: ?t= starts playback partway into a VOD", async ({
  page,
}) => {
  // Uploading and transcoding the fixture can outlast the suite's default 90s
  // test timeout on a loaded CI runner.
  test.setTimeout(10 * 60_000);

  const watchPath = await uploadFixture(page);
  expect(watchPath).toContain(ACCOUNT_DID as string);

  await page.goto(`${HTTPS_URL}${watchPath}?t=${START_SECONDS}`);
  const video = page.locator("video").first();
  await expect(video).toBeVisible({ timeout: 30_000 });

  // The player opens at the requested position (the fragment containing it,
  // so the exact frame is a little after it), not at 0:00.
  await expect
    .poll(
      () => video.evaluate((element: HTMLVideoElement) => element.currentTime),
      { timeout: 60_000 },
    )
    .toBeGreaterThanOrEqual(START_SECONDS);
});

test("vod-start-time: playback recovers after the HLS chunk fails", async ({
  page,
}) => {
  test.setTimeout(10 * 60_000);
  const watchPath = await uploadFixture(page);
  // Chromium may advertise native HLS. Exercise browsers that need hls.js,
  // while still decoding and playing the real uploaded media.
  await page.addInitScript(() => {
    const canPlayType = HTMLMediaElement.prototype.canPlayType;
    HTMLMediaElement.prototype.canPlayType = function (type: string) {
      return type === "application/vnd.apple.mpegurl"
        ? ""
        : canPlayType.call(this, type);
    };
  });

  let attempts = 0;
  await page.context().route(/\/hls-[^/]+\.js$/, async (route) => {
    attempts++;
    if (attempts === 1) {
      await route.fulfill({ status: 503, body: "Temporarily unavailable" });
    } else {
      await route.continue();
    }
  });

  await page.goto(`${HTTPS_URL}${watchPath}?t=${START_SECONDS}`);
  const video = page.locator("video").first();
  await expect(video).toBeVisible({ timeout: 30_000 });
  await expect.poll(() => attempts, { timeout: 30_000 }).toBeGreaterThan(1);
  await expect
    .poll(
      () =>
        video.evaluate(
          (element: HTMLVideoElement, startSeconds: number) => ({
            paused: element.paused,
            progressed: element.currentTime > startSeconds + 1,
          }),
          START_SECONDS,
        ),
      { timeout: 60_000 },
    )
    .toEqual({ paused: false, progressed: true });
});
