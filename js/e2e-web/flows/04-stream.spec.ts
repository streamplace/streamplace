import { expect, test } from "@playwright/test";

// Mirror of .maestro/04-stream.yaml: open the looping test stream from the home
// feed and confirm playback context loads. The home feed card carries
// testID="home-stream-card" (data-testid on web); the "Now streaming - e2e test
// stream" chat system message arrives over the chat socket shortly after open.
test("04-stream: open test stream from feed", async ({ page }) => {
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

test("04-stream: portrait player exposes playback controls", async ({
  page,
}) => {
  await page.setViewportSize({ width: 320, height: 568 });
  await page.goto("/");

  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });

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

  const unmuteButton = page.getByLabel("Unmute");
  const unmuteBox = await unmuteButton.boundingBox();
  if (!unmuteBox) {
    throw new Error("portrait player mute button has no bounding box");
  }
  await expect
    .poll(
      () =>
        unmuteButton.evaluate(
          (element) => getComputedStyle(element).pointerEvents,
        ),
      { timeout: 5_000 },
    )
    .toBe("none");
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
