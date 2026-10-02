import { expect, test } from "@playwright/test";

// Web-only coverage: the streaming entry point is the Live Dashboard (/live).
// Visiting it while logged out opens the login modal and leaves the dashboard
// behind a spinner.
test("go-live: streaming requires login", async ({ page }) => {
  await page.goto("/live");

  await expect(page.getByText("Log in").first()).toBeVisible({
    timeout: 30_000,
  });
});
