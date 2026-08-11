import { expect, test } from "./fixtures";

// Sidebar active state: the highlighted nav item should follow the
// current route instead of being stuck on Home.
test.describe("sidebar active state", () => {
  const activeNavButtons = (page: import("@playwright/test").Page) =>
    page.locator('[data-sidebar="menu-button"][data-active]');

  test("home is active on /", async ({ page }) => {
    await page.goto("/");
    await page.waitForLoadState("networkidle");
    await expect(activeNavButtons(page)).toHaveText(["Home"]);
  });

  test("videos is active on /videos", async ({ page }) => {
    await page.goto("/videos");
    await page.waitForLoadState("networkidle");
    await expect(activeNavButtons(page)).toHaveText(["Videos"]);
  });

  test("settings is active on /settings", async ({ page }) => {
    await page.goto("/settings");
    await page.waitForLoadState("networkidle");
    await expect(activeNavButtons(page)).toHaveText(["Settings"]);
  });

  test("settings stays active on a settings sub-route", async ({ page }) => {
    await page.goto("/settings/account");
    await page.waitForLoadState("networkidle");
    await expect(activeNavButtons(page)).toHaveText(["Settings"]);
  });

  test("no nav item is active on a stream page", async ({ page }) => {
    await page.goto("/test-user");
    await page.waitForLoadState("networkidle");
    await expect(activeNavButtons(page)).toHaveCount(0);
  });
});
