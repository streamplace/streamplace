import { expect, test } from "./fixtures";

// Smoke tests: verify the app loads and core navigation works.
// These are the minimum viable e2e tests that should pass for any
// deploy to be considered functional.

test.describe("app smoke", () => {
  test("home page loads", async ({ homePage }) => {
    await expect(homePage).toHaveTitle(/Streamplace/i);
  });

  test("navigation sidebar is visible", async ({ homePage }) => {
    const sidebar = homePage.locator("aside, [data-sidebar]");
    await expect(sidebar).toBeVisible();
  });

  test("header is visible", async ({ homePage }) => {
    const header = homePage.locator("header");
    await expect(header).toBeVisible();
  });

  test("shows live streams or empty state", async ({ homePage }) => {
    // Either we see stream cards or the "no one streaming" empty state.
    const streamCards = homePage.locator("[class*='group flex flex-col']");
    const emptyState = homePage.getByText(/no one|nobody/i);

    // Wait a moment for data to load.
    await homePage.waitForTimeout(2000);

    // One of the two should be visible.
    const hasCards = (await streamCards.count()) > 0;
    const hasEmpty = (await emptyState.count()) > 0;
    expect(hasCards || hasEmpty).toBeTruthy();
  });
});
