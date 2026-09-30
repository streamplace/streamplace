import { expect, test } from "@playwright/test";

// Web counterpart of .maestro/logged-out/tabs.yaml. The web app renders the desktop
// layout (a sidebar of nav links) rather than the mobile tab bar, so we
// exercise the sidebar: Home -> Settings -> Home. Nav links carry their label
// ("Home", "Settings", ...) as the accessible name.
test("tabs: primary navigation", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Streamplace").first()).toBeVisible();

  await page
    .getByRole("link", { name: "Settings", exact: true })
    .first()
    .click();
  await expect(page.getByText("Advanced").first()).toBeVisible();

  await page.getByRole("link", { name: "Home", exact: true }).first().click();
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
    timeout: 30_000,
  });
});
