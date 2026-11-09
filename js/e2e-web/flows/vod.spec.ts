import { expect, test } from "@playwright/test";
import { watchPath } from "./vod-upload";

// The harness publishes a VOD titled "e2e test video" for the test account
// (pkg/cmd/e2e.go) and exports its AT URI as VIDEO_URI. A video page's tab is
// titled after the video, both in the page the node serves and once the app
// has loaded, so open tabs and bookmarks tell videos apart.
const VIDEO_URI = process.env.VIDEO_URI!;
const VIDEO_TITLE = "e2e test video";
const videoPath = () => watchPath(VIDEO_URI);

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
  await page.goto(videoPath());
  // The node's page already carries the title; wait for the app to load the
  // video (it shows the title) so the check below sees the app's own title.
  await expect(page.getByText(VIDEO_TITLE).first()).toBeVisible();
  await expect(page).toHaveTitle(VIDEO_TITLE);
});
