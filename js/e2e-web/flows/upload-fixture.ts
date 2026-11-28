import { expect, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Upload the harness fixture through the app's own /upload workflow and return
// the published video's watch path. The harness's built-in test VOD has no
// source tracks, so flows that need real media upload E2E_FIXTURE_MP4 this way.
//
// Needs the harness's HTTPS mode: the upload is a signed-in OAuth flow.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const FIXTURE = process.env.E2E_FIXTURE_MP4;

export async function uploadFixture(
  page: Page,
  beforePublish?: (page: Page) => Promise<void>,
): Promise<string> {
  if (!HTTPS_URL || !FIXTURE) {
    throw new Error(
      "uploadFixture needs SERVER_HTTPS_URL and E2E_FIXTURE_MP4 from the harness",
    );
  }

  await loginThroughPds(page);
  await page.goto(`${HTTPS_URL}/upload`);
  await page.setInputFiles('input[type="file"][accept="video/*"]', FIXTURE);

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
    .fill(`e2e upload ${Date.now()}`);
  const [saved] = await Promise.all([
    page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        new URL(response.url()).pathname ===
          "/xrpc/place.stream.vod.updateDraft",
    ),
    page.getByRole("button", { name: "Save Draft", exact: true }).click(),
  ]);
  expect(saved.ok(), `updateDraft: HTTP ${saved.status()}`).toBe(true);
  await beforePublish?.(page);

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
