import { expect, test } from "@playwright/test";

// Until the viewer clicks, nothing may start playback, muted or not (as on
// iOS in Low Power Mode). The policy flag stops the video's own autoplay;
// Chromium allows muted play() even so, so the init script refuses the
// player's play() calls until the first click. Launch options apply per
// file, hence this one.
test.use({
  launchOptions: { args: ["--autoplay-policy=user-gesture-required"] },
});

// The play button belongs to the video's autoplay attempt, which a layout
// change doesn't repeat: it has to stay up until the viewer presses it.
test("autoplay-blocked: the play button survives a resize", async ({
  page,
}) => {
  await page.addInitScript(() => {
    let clicked = false;
    addEventListener("pointerdown", () => (clicked = true), { capture: true });
    const play = HTMLMediaElement.prototype.play;
    HTMLMediaElement.prototype.play = function () {
      if (clicked) return play.call(this);
      return Promise.reject(new DOMException("blocked", "NotAllowedError"));
    };
  });
  await page.setViewportSize({ width: 900, height: 720 });
  await page.goto(`/${process.env.ACCOUNT_DID}`);
  const playButton = page.getByRole("button", { name: "Play", exact: true });
  await expect(playButton).toBeVisible();

  // Through the desktop layout and back, which moves the player.
  const video = page.locator("video").first();
  const videoWidth = () =>
    video.evaluate((v: HTMLVideoElement) => v.getBoundingClientRect().width);
  for (const width of [1280, 900]) {
    const oldWidth = await videoWidth();
    await page.setViewportSize({ width, height: 720 });
    await expect.poll(videoWidth).not.toBe(oldWidth);
  }
  await expect(playButton).toBeVisible();

  await playButton.click();
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => !v.paused))
    .toBe(true);
  await expect(playButton).toBeHidden();
});
