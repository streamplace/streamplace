import { expect, test } from "./fixtures";

// Route smoke tests: verify every route loads without crashing.
// These catch broken imports, missing providers, and 500s from the
// server-side rendering layer. They don't verify content deeply,
// just that the page renders something.

test.describe("route smoke", () => {
  test("home (/)", async ({ homePage }) => {
    await expect(homePage.locator("body")).toBeVisible();
  });

  test("login (/login)", async ({ page }) => {
    await page.goto("/login");
    await page.waitForLoadState("networkidle");
    // Should show a login form or "completing sign in" state.
    await expect(page.locator("body")).toBeVisible();
    const hasForm = await page.getByRole("textbox").count();
    const hasText = await page.getByText(/log in|sign|completing/i).count();
    expect(hasForm + hasText).toBeGreaterThan(0);
  });

  test("search (/search)", async ({ page }) => {
    await page.goto("/search");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("videos (/videos)", async ({ page }) => {
    await page.goto("/videos");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("stream page (/$user)", async ({ page }) => {
    await page.goto("/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("VOD page (/$user/video/$tid)", async ({ page }) => {
    // Non-existent VOD: should load without crashing.
    await page.goto("/test-user/video/nonexistent");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("chat popout (/chat-popout/$user)", async ({ page }) => {
    await page.goto("/chat-popout/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("dashboard (/dashboard)", async ({ page }) => {
    await page.goto("/dashboard");
    await page.waitForLoadState("networkidle");
    // Should show login prompt or loading state since we're not authed.
    await expect(page.locator("body")).toBeVisible();
    const hasLoginPrompt = await page.getByText(/log in|loading/i).count();
    expect(hasLoginPrompt).toBeGreaterThan(0);
  });

  test("dashboard sub-routes redirect or gate", async ({ page }) => {
    // These should all redirect to login or show the auth gate
    // rather than crashing.
    const routes = [
      "/dashboard/stream",
      "/dashboard/keys",
      "/dashboard/multistream",
      "/dashboard/recommendations",
      "/dashboard/webhooks",
      "/dashboard/upload",
      "/dashboard/videos",
    ];
    for (const route of routes) {
      await page.goto(route);
      await page.waitForLoadState("networkidle");
      await expect(page.locator("body")).toBeVisible();
    }
  });
});
