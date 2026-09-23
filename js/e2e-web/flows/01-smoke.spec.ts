import { expect, test } from "@playwright/test";

// Mirror of .maestro/01-smoke.yaml: the app loads and renders the home feed.
test("01-smoke: app loads", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Streamplace").first()).toBeVisible();
});
