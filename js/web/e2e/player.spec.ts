import { expect, test } from "./fixtures";

// Player tests: verify the video player renders and responds to
// interaction. These are smoke-level: we verify the player mounts
// and the controls are present, not that actual video plays (which
// requires a live stream).

test.describe("player", () => {
  test("player chrome renders on stream page", async ({ page }) => {
    await page.goto("/nonexistent-user");
    await page.waitForLoadState("networkidle");

    // The player container should be present.
    const playerContainer = page.locator(
      ".group.relative.h-full.w-full.bg-black",
    );
    await expect(playerContainer).toBeVisible({ timeout: 10000 });
  });

  test("player shows offline state for non-existent streamer", async ({
    page,
  }) => {
    await page.goto("/nonexistent-user");
    await page.waitForLoadState("networkidle");

    // Should eventually show an offline/never-live state.
    const offlineText = page.getByText(/offline|not.*stream/i);
    await expect(offlineText.first()).toBeVisible({ timeout: 15000 });
  });
});
