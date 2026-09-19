import { expect, test } from "@playwright/test";

// Web counterpart of .maestro/03-go-live.yaml. On mobile the "Go Live" tab ->
// "Start streaming" prompts an unauthenticated user to log in. The web app's
// streaming entry point is the Live Dashboard (/live); visiting it while logged
// out opens the login modal and leaves the dashboard behind a spinner.
test("03-go-live: streaming requires login", async ({ page }) => {
  await page.goto("/live");

  await expect(page.getByText("Log in").first()).toBeVisible({
    timeout: 30_000,
  });
});
