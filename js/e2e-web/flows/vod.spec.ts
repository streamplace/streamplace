import { expect, test } from "@playwright/test";

// The harness publishes a VOD titled "e2e test video" for the test account
// (pkg/cmd/e2e.go) and exports its AT URI as VIDEO_URI. A video page's tab is
// titled after the video, both in the page the node serves and once the app
// has loaded, so open tabs and bookmarks tell videos apart.
const VIDEO_URI = process.env.VIDEO_URI;
const VIDEO_TITLE = "e2e test video";
const RECOVERED_VIDEO_TITLE = "e2e recovered video";
const UNHOSTED_VIDEO_TITLE = "e2e unhosted video";

for (const scope of ["global", "profile"] as const) {
  test(`08-vod: ${scope} listing recovers hosted videos after boot`, async ({
    page,
  }) => {
    const handle = process.env.ACCOUNT_HANDLE;
    if (scope === "profile" && !handle) {
      throw new Error(
        "ACCOUNT_HANDLE is required for the profile video listing",
      );
    }

    // This fixture's video and track were indexed before launch, but its own
    // media.origin existed only in the server repo. The real listing API must
    // expose it after startup reconciliation, without exposing unhosted videos.
    await page.goto(scope === "global" ? "/video" : `/${handle}/video`);
    await expect(
      page.getByText(RECOVERED_VIDEO_TITLE, { exact: true }),
    ).toBeVisible({ timeout: 30_000 });
    await expect(
      page.getByText(UNHOSTED_VIDEO_TITLE, { exact: true }),
    ).toHaveCount(0);
  });
}

function videoPath(): string {
  const match = VIDEO_URI?.match(
    /^at:\/\/([^/]+)\/place\.stream\.video\/([^/]+)$/,
  );
  if (!match) {
    throw new Error(`VIDEO_URI is not a place.stream.video URI: ${VIDEO_URI}`);
  }
  return `/${match[1]}/video/${match[2]}`;
}

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
