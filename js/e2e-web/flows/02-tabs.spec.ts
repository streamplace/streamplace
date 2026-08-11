import { expect, test } from "@playwright/test";

// Web counterpart of .maestro/02-tabs.yaml. The web app renders the desktop
// layout (a sidebar of nav links) rather than the mobile tab bar, so we
// exercise the sidebar: Home -> Settings -> Home. Nav links carry accessible
// names of the form "Link to /<route>".
test("02-tabs: primary navigation", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Streamplace").first()).toBeVisible();

  await page.getByRole("link", { name: "Link to /settings" }).first().click();
  await expect(page.getByText("Advanced").first()).toBeVisible();

  await page
    .getByRole("link", { name: "Link to /", exact: true })
    .first()
    .click();
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
    timeout: 30_000,
  });
});
