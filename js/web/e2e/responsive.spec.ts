import { expect, test } from "./fixtures";

// Responsive tests: verify the app renders at different viewport sizes
// without horizontal overflow or broken layouts.

test.describe("responsive", () => {
  test("mobile viewport (375px)", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 812 },
    });
    const page = await context.newPage();
    await page.goto("/");
    await page.waitForLoadState("networkidle");

    // No horizontal scroll.
    const scrollWidth = await page.evaluate(() => document.body.scrollWidth);
    const clientWidth = await page.evaluate(() => document.body.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1); // 1px tolerance

    await context.close();
  });

  test("desktop viewport (1440px)", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
    });
    const page = await context.newPage();
    await page.goto("/");
    await page.waitForLoadState("networkidle");

    const scrollWidth = await page.evaluate(() => document.body.scrollWidth);
    const clientWidth = await page.evaluate(() => document.body.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);

    await context.close();
  });

  test("stream page mobile layout", async ({ browser }) => {
    const context = await browser.newContext({
      viewport: { width: 375, height: 812 },
    });
    const page = await context.newPage();
    await page.goto("/test-user");
    await page.waitForLoadState("networkidle");

    // Video section should be visible and fit within viewport.
    const videoSection = page.locator(".bg-black").first();
    await expect(videoSection).toBeVisible({ timeout: 10000 });

    const scrollWidth = await page.evaluate(() => document.body.scrollWidth);
    const clientWidth = await page.evaluate(() => document.body.clientWidth);
    expect(scrollWidth).toBeLessThanOrEqual(clientWidth + 1);

    await context.close();
  });
});
