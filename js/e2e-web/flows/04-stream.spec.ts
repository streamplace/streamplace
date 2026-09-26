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
  await page.setViewportSize({ width: 390, height: 844 });
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
});
