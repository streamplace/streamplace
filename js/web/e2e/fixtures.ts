import { test as base, expect, type Page } from "@playwright/test";

// Shared fixtures for e2e tests. Provides a clean home page for each
// test and helper utilities for interacting with the Streamplace web app.

type AppFixtures = {
  homePage: Page;
};

export const test = base.extend<AppFixtures>({
  homePage: async ({ page }, use) => {
    await page.goto("/");
    await page.waitForLoadState("networkidle");
    await use(page);
  },
});

export { expect };
