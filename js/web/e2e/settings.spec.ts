import { expect, test } from "./fixtures";

// Settings page tests: verify settings routes load and render forms.
// These pages are auth-gated, so without a session they may redirect
// or show a login prompt. The key invariant: they don't crash.

test.describe("settings", () => {
  test("settings index renders", async ({ page }) => {
    await page.goto("/settings");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("settings sub-pages don't crash", async ({ page }) => {
    const pages = [
      "/settings/account",
      "/settings/about",
      "/settings/advanced",
      "/settings/backup",
      "/settings/badges",
      "/settings/badge-issuer",
      "/settings/branding",
      "/settings/chat-profile",
      "/settings/danmu",
      "/settings/languages",
      "/settings/notifications",
      "/settings/privacy",
    ];
    for (const url of pages) {
      await page.goto(url);
      await page.waitForLoadState("networkidle");
      await expect(page.locator("body")).toBeVisible();
    }
  });
});
