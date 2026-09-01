import { expect, test } from "@playwright/test";

// Web counterpart of .maestro/03-go-live.yaml. On mobile the "Go Live" tab ->
// "Start streaming" prompts an unauthenticated user to log in. The web app's
// streaming entry point is the Live Dashboard (/live); visiting it while logged
// out surfaces the same Log In / Sign Up prompt.
test("03-go-live: streaming requires login", async ({ page }) => {
  await page.goto("/");

  await page.getByRole("link", { name: "Link to /live" }).first().click();
  await expect(page.getByText("Live Dashboard").first()).toBeVisible();
  await expect(page.getByRole("button", { name: "Log In" })).toBeVisible();
});
