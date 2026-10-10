import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// The About entry is a top-level sidebar destination while signed out — the
// settings menu is not somewhere a brand-new viewer would think to look — and
// folds back into Settings > About once you are signed in.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test("about-link: signed out, the sidebar links straight to the About page", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
    timeout: 30_000,
  });

  await page.getByRole("link", { name: "About", exact: true }).first().click();
  await expect(page).toHaveURL(/\/settings\/about$/);
  await expect(page.getByText(/^Streamplace v/).first()).toBeVisible();
});

test("about-link: signed in, About stays in the settings menu", async ({
  page,
}) => {
  test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");
  await loginThroughPds(page);

  await page.goto(`${HTTPS_URL}/`);
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
    timeout: 30_000,
  });
  // the sidebar drops "Log in" once the session lands, so it is showing the
  // signed-in list by the time the absence check runs
  await expect(
    page.getByRole("link", { name: "Log in", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("link", { name: "About", exact: true }),
  ).toHaveCount(0);

  await page
    .getByRole("link", { name: "Settings", exact: true })
    .first()
    .click();
  await expect(page.getByText("Advanced").first()).toBeVisible();
  await page.getByText("About", { exact: true }).first().click();
  await expect(page).toHaveURL(/\/settings\/about$/);
});
