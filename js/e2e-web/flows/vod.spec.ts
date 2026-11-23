import { expect, test } from "@playwright/test";

// The harness publishes a VOD titled "e2e test video" for the test account
// (pkg/cmd/e2e.go: the harness fixture, playable) and exports its AT URI as
// VIDEO_URI. A video page's tab is titled after the video, both in the page the
// node serves and once the app has loaded, so open tabs and bookmarks tell
// videos apart.
const VIDEO_URI = process.env.VIDEO_URI!;
const VIDEO_TITLE = "e2e test video";
// The HTTPS harness only permits HTTPS OAuth redirects during app initialization.
const APP_URL = process.env.SERVER_HTTPS_URL ?? process.env.SERVER_URL;
const videoPath = () =>
  VIDEO_URI.replace(
    /^at:\/\/([^/]+)\/place\.stream\.video\/([^/]+)$/,
    "/$1/video/$2",
  );

test("vod: the node serves the video page titled after the video", async ({
  request,
}) => {
  // The node indexes the record off the firehose, shortly after the harness
  // writes it.
  await expect
    .poll(async () => (await request.get(videoPath())).text())
    .toContain(`<title>${VIDEO_TITLE}</title>`);
});

test("vod: the app titles the tab after the video", async ({ page }) => {
  await page.goto(`${APP_URL}${videoPath()}`);
  // The node's page already carries the title; wait for the app to load the
  // video (it shows the title) so the check below sees the app's own title.
  await expect(page.getByText(VIDEO_TITLE).first()).toBeVisible();
  await expect(page).toHaveTitle(VIDEO_TITLE);
});

test("vod: the video page plays", async ({ page }) => {
  await page.goto(`${APP_URL}${videoPath()}`);
  const video = page.locator("video").first();
  await expect(video).toBeVisible();
  // Re-resolved on every poll: the app adopts the page's own <video> into its
  // player once it has loaded.
  await expect
    .poll(
      () => video.evaluate((element: HTMLVideoElement) => element.currentTime),
      { timeout: 60_000 },
    )
    .toBeGreaterThan(2);
  await expect(page.locator("#sp-vod-boot")).toHaveCount(0);
  await expect(video).toHaveAttribute("src", /^blob:/);
});

test("vod: the page's own video shows a frame without the app bundle", async ({
  page,
}) => {
  // Block the app bootstrap, not the standalone engine the inline boot imports.
  await page.route(/\/bootstrap-[^/]+\.js$/, (route) => route.abort());
  await page.goto(videoPath(), { waitUntil: "domcontentloaded" });
  await expect
    .poll(
      () =>
        page.evaluate(() => {
          const video =
            document.querySelector<HTMLVideoElement>("#sp-vod-boot video");
          return !!video && video.readyState >= 2 && video.videoWidth > 0;
        }),
      { timeout: 30_000 },
    )
    .toBe(true);
});
