import { expect, test } from "./fixtures";

// Embed tests: verify embed routes load and render minimal UI.
// These are used by OBS, info widgets, and danmu overlays.

test.describe("embeds", () => {
  test("stream embed (/embed/$user)", async ({ page }) => {
    await page.goto("/embed/test-user");
    await page.waitForLoadState("networkidle");
    // Embed should render a video player or poster.
    const playerOrVideo = page.locator("video, img");
    await expect(playerOrVideo.first()).toBeVisible({ timeout: 10000 });
  });

  test("VOD embed (/embed/$user/video/$tid)", async ({ page }) => {
    await page.goto("/embed/test-user/video/nonexistent");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("danmu OBS embed (/embed/danmu-obs/$user)", async ({ page }) => {
    await page.goto("/embed/danmu-obs/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("info widget embed (/embed/info-widget/$user)", async ({ page }) => {
    await page.goto("/embed/info-widget/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });
});
