import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Mirror of .maestro/logged-out/stream.yaml: open the looping test stream from the home
// feed and confirm playback context loads. The home feed card carries
// testID="home-stream-card" (data-testid on web); the "Now streaming - e2e test
// stream" chat system message arrives over the chat socket shortly after open.
test("stream: open test stream from feed", async ({ page }) => {
  await page.goto("/");

  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();

  // the stream page mounts a video element and posts a "Now streaming - ..."
  // system message into chat once playback context is established
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  await expect(page.locator("video").first()).toBeVisible();
});

test("stream: portrait player exposes playback controls", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 568 });
  await page.goto("/");

  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();

  // Chat may virtualize its initial system message once history fills up.
  // Playback controls depend on the player being ready, not that chat row.
  const video = page.locator("video").first();
  await expect(video).toBeVisible({ timeout: 30_000 });
  const videoBox = await video.boundingBox();
  if (!videoBox) {
    throw new Error("portrait player video has no bounding box");
  }
  await page.mouse.move(
    videoBox.x + videoBox.width / 2,
    videoBox.y + videoBox.height + 16,
  );
  await page.mouse.move(
    videoBox.x + videoBox.width / 2,
    videoBox.y + videoBox.height / 2,
  );

  await expect(page.getByLabel("Mute")).toBeVisible();
  await expect(page.getByLabel("Enter fullscreen")).toBeVisible();

  await page.getByLabel("Mute").click();
  await expect(page.getByLabel("Unmute")).toBeVisible();
  await expect
    .poll(() => video.evaluate((element: HTMLVideoElement) => element.muted))
    .toBe(true);

  const unmuteButton = page.getByLabel("Unmute");
  const unmuteBox = await unmuteButton.boundingBox();
  if (!unmuteBox) {
    throw new Error("portrait player mute button has no bounding box");
  }
  await expect
    .poll(() =>
      Promise.all(
        [unmuteButton, page.getByLabel("Back", { exact: true })].map(
          (control) =>
            control.evaluate(
              (element) => getComputedStyle(element).pointerEvents,
            ),
        ),
      ),
    )
    .toEqual(["none", "none"]);
  await page.mouse.click(
    unmuteBox.x + unmuteBox.width / 2,
    unmuteBox.y + unmuteBox.height / 2,
  );
  await expect
    .poll(() =>
      unmuteButton.evaluate((element) => {
        let opacity = 1;
        let current: Element | null = element;
        while (current) {
          opacity *= Number.parseFloat(getComputedStyle(current).opacity);
          current = current.parentElement;
        }
        return opacity;
      }),
    )
    .toBeGreaterThan(0.99);
  await expect
    .poll(() => video.evaluate((element: HTMLVideoElement) => element.muted))
    .toBe(true);

  await page.getByLabel("Enter fullscreen").click();
  await expect
    .poll(() => page.evaluate(() => Boolean(document.fullscreenElement)))
    .toBe(true);
  const exitFullscreen = page
    .locator(":fullscreen")
    .getByLabel("Exit fullscreen");
  await expect(exitFullscreen).toBeVisible();
  await exitFullscreen.click();
  await expect
    .poll(() => page.evaluate(() => Boolean(document.fullscreenElement)))
    .toBe(false);
  await expect(page.getByLabel("Enter fullscreen")).toBeVisible();
});

// Crossing the desktop/mobile breakpoint moves the player to another branch
// of the layout. The video has to survive that: same element, still playing
// what it was (a WebRTC srcObject, or a src, which hls.js sets to a new blob:
// URL on the element or a <source> child each time it starts over).
async function expectVideoSurvivesResize(page: Page) {
  const video = page.locator("video").first();
  await expect
    .poll(() =>
      video.evaluate((v: HTMLVideoElement) => !v.paused && v.readyState >= 2),
    )
    .toBe(true);
  await video.evaluate((v: HTMLVideoElement) => {
    const src = v.src + (v.querySelector("source")?.src ?? "");
    (window as any).__spVideo = { el: v, src, srcObject: v.srcObject };
  });

  const videoWidth = () =>
    page.evaluate(
      () => document.querySelector("video")?.getBoundingClientRect().width,
    );
  for (const width of [900, 1280]) {
    const oldWidth = await videoWidth();
    await page.setViewportSize({ width, height: 720 });
    // The layout has caught up once the video's box changes.
    await expect.poll(videoWidth).not.toBe(oldWidth);
    const state = await page.evaluate(() => {
      const before = (window as any).__spVideo;
      const v = document.querySelector("video")!;
      const src = v.src + (v.querySelector("source")?.src ?? "");
      return {
        sameElement: v === before.el,
        sameSource: src === before.src && v.srcObject === before.srcObject,
      };
    });
    expect(state).toEqual({ sameElement: true, sameSource: true });
    await expect(video).toBeVisible();
    const before = await video.evaluate((v: HTMLVideoElement) => v.currentTime);
    await expect
      .poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime))
      .toBeGreaterThan(before);
  }
}

test("stream: resizing the window keeps the same playing video", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto("/");
  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();
  await expectVideoSurvivesResize(page);
});

// The streamer watching their own stream over HLS plays it with their own
// playback session, which is part of the playlist URL. A layout change must
// not drop it, or the URL changes and hls.js starts over.
test("stream: the streamer's own HLS video survives a resize", async ({
  page,
}) => {
  const httpsUrl = process.env.SERVER_HTTPS_URL;
  test.skip(!httpsUrl, "harness started without its HTTPS hostnames");
  await page.setViewportSize({ width: 1280, height: 720 });
  await loginThroughPds(page);
  await page.evaluate(() => localStorage.setItem("player-protocol", "hls"));

  await page.goto(`${httpsUrl}/`);
  // The player's own master playlist URL, once it has the session (the
  // node's variant playlists carry a sid of their own). hls.js attaches its
  // new source in the same task as this request.
  const sessionPlaylist = page.waitForRequest((r) =>
    /getLivePlaylist\?streamer=[^&]+&sid=/.test(r.url()),
  );
  await page.getByTestId("home-stream-card").first().click();
  await sessionPlaylist;
  await expectVideoSurvivesResize(page);
});
