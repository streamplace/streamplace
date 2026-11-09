import { expect, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Upload the harness fixture (E2E_FIXTURE_MP4) through the app's own /upload
// workflow and return the published video's AT URI. The harness's own test VOD
// has no source tracks, so this is what gives the player real media. Needs the
// harness's HTTPS mode for the login.
export async function uploadFixture(page: Page): Promise<string> {
  await loginThroughPds(page);
  await page.goto(`${process.env.SERVER_HTTPS_URL}/upload`);
  await page.setInputFiles(
    'input[type="file"][accept="video/*"]',
    process.env.E2E_FIXTURE_MP4!,
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
    .fill(`e2e upload ${Date.now()}`);
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
  return videoUri;
}

// The app's watch path for a place.stream.video AT URI, addressed by `user`
// (a handle or DID) when given, else by the URI's DID.
export function watchPath(videoUri: string, user?: string): string {
  const match = videoUri.match(
    /^at:\/\/([^/]+)\/place\.stream\.video\/([^/]+)$/,
  );
  if (!match) {
    throw new Error(`not a place.stream.video uri: ${videoUri}`);
  }
  return `/${user ?? match[1]}/video/${match[2]}`;
}
