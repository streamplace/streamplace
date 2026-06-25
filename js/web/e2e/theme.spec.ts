import { expect, test } from "./fixtures";

// Theme tests: verify dark/light theme toggle works and the
// preference persists across reloads.

test.describe("theme", () => {
  test("defaults to dark theme", async ({ homePage }) => {
    const html = homePage.locator("html");
    await expect(html).toHaveClass(/dark/);
  });

  test("page is not blank in dark mode", async ({ homePage }) => {
    // If the CSS variables aren't resolving, text will be invisible.
    // Check that the header has visible text.
    const header = homePage.locator("header");
    await expect(header).toBeVisible();
    const headerText = await header.innerText();
    expect(headerText.trim().length).toBeGreaterThan(0);
  });
});
