import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

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
const FIXTURE = process.env.E2E_FIXTURE_MP4;
const START_SECONDS = 151;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");
test.skip(!FIXTURE, "harness did not export E2E_FIXTURE_MP4");

// Upload the harness fixture through /upload and return the published video's
// watch path. The app creates the draft up front and puts the bytes up in the
// background, so the draft editor opens (with the tid in the URL) well before
// processing finishes.
async function uploadFixture(page: Page): Promise<string> {
  await loginThroughPds(page);
  await page.goto(`${HTTPS_URL}/upload`);
  await page.setInputFiles(
    'input[type="file"][accept="video/*"]',
    FIXTURE as string,
  );

  // The app mints the draft up front and sends the bytes in the background, so
  // the draft editor opens (with the tid in the URL) before processing ends.
  // Watch for the tus PATCH: reloading below while it is in flight would abort
  // it (the app guards the page with beforeunload, which cancels the reload).
  const uploaded = page.waitForResponse(
    (response) =>
      response.request().method() === "PATCH" &&
      new URL(response.url()).pathname.startsWith("/api/upload/"),
    { timeout: 2 * 60_000 },
  );
  await page.getByRole("button", { name: "Upload video", exact: true }).click();
  await page.waitForURL(/\/upload\/video\//, { timeout: 30_000 });
  expect((await uploaded).ok()).toBe(true);

  // Publish only appears once the node has finished transcoding the upload,
  // and the editor reads the draft once, so reload until it is ready. The
  // budget is headroom for a slow transcode on a loaded CI runner.
  const publish = page.getByRole("button", { name: "Publish", exact: true });
  await expect(async () => {
    await page.reload();
    await expect(publish).toBeVisible({ timeout: 20_000 });
  }).toPass({ timeout: 10 * 60_000 });

  // Publish copies the draft as it stands, so give it a title first.
  await page
    .getByPlaceholder("Give your video a title")
    .fill(`t-param test video ${Date.now()}`);
  await page.getByRole("button", { name: "Save Draft", exact: true }).click();

  const published = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      new URL(response.url()).pathname ===
        "/xrpc/place.stream.vod.publishDraft",
  );
  await publish.click();
  const { videoUri } = (await (await published).json()) as {
    videoUri: string;
  };

  const match = videoUri.match(
    /^at:\/\/([^/]+)\/place\.stream\.video\/([^/]+)$/,
  );
  if (!match) {
    throw new Error(`publishDraft returned an unexpected uri: ${videoUri}`);
  }
  return `/${match[1]}/video/${match[2]}`;
}

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
