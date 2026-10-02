import { expect, test } from "@playwright/test";

// Mirror of .maestro/logged-out/smoke.yaml: the app loads and renders the home feed.
test("smoke: app loads", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Streamplace").first()).toBeVisible();
});
