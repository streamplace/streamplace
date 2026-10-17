import { expect, test } from "./fixtures";

// Regression tests for navigation flows that have historically caused
// crashes. Each test reproduces a specific user journey that broke.

test.describe("navigation regression", () => {
  test("settings to dashboard and back does not crash", async ({ page }) => {
    // This reproduces a crash where navigating from the regular site
    // to settings, then to the dashboard (via "creator settings" link),
    // then back to the stream page would throw because the
    // FullscreenProvider / SidebarProvider contexts get torn down
    // and remounted across the layout boundary.
    await page.goto("/");
    await page.waitForLoadState("networkidle");

    // Navigate to settings.
    await page.goto("/settings");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();

    // Navigate to dashboard (the "creator settings" link in settings nav).
    await page.goto("/dashboard/stream");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();

    // Navigate back to a stream page (the "Back to Streamplace" link).
    await page.goto("/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();

    // The page should be interactive, not a white screen / error boundary.
    // Check that we don't see the error boundary.
    const errorBoundary = page.getByText(/something went wrong/i);
    await expect(errorBoundary).toHaveCount(0);

    // Check that the regular chrome (header or sidebar) reappeared.
    const header = page.locator("header");
    const sidebar = page.locator("aside, [data-sidebar]");
    const hasChrome = (await header.count()) + (await sidebar.count());
    expect(hasChrome).toBeGreaterThan(0);
  });

  test("dashboard to home to dashboard does not crash", async ({ page }) => {
    // Another layout-boundary crossing: dashboard → home → dashboard.
    await page.goto("/dashboard");
    await page.waitForLoadState("networkidle");

    await page.goto("/");
    await page.waitForLoadState("networkidle");

    await page.goto("/dashboard");
    await page.waitForLoadState("networkidle");

    await expect(page.locator("body")).toBeVisible();
    const errorBoundary = page.getByText(/something went wrong/i);
    await expect(errorBoundary).toHaveCount(0);
  });

  test("rapid navigation between layouts does not crash", async ({ page }) => {
    // Rapidly switching between ChromeLayout and DashboardChrome
    // routes to catch race conditions in provider mount/unmount.
    const routes = ["/", "/dashboard", "/settings", "/dashboard/stream", "/"];
    for (const route of routes) {
      await page.goto(route);
      await page.waitForLoadState("domcontentloaded");
      await expect(page.locator("body")).toBeVisible();
    }
    const errorBoundary = page.getByText(/something went wrong/i);
    await expect(errorBoundary).toHaveCount(0);
  });

  test("stream page to dashboard and back preserves theme", async ({
    page,
  }) => {
    // Navigating across layout boundaries should not reset the
    // dark/light theme class on <html>.
    await page.goto("/");
    await page.waitForLoadState("networkidle");
    const html = page.locator("html");
    const initialDark = await html.evaluate((el) =>
      el.classList.contains("dark"),
    );

    await page.goto("/dashboard");
    await page.waitForLoadState("networkidle");
    const dashboardDark = await html.evaluate((el) =>
      el.classList.contains("dark"),
    );
    expect(dashboardDark).toBe(initialDark);

    await page.goto("/");
    await page.waitForLoadState("networkidle");
    const finalDark = await html.evaluate((el) =>
      el.classList.contains("dark"),
    );
    expect(finalDark).toBe(initialDark);
  });
});
