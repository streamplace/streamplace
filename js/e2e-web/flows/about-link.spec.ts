import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// The About entry is a top-level sidebar destination while signed out — the
// settings menu is not somewhere a brand-new viewer would think to look — and
// folds back into Settings > About once you are signed in.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

const sidebarLink = (page: Page, name: string) =>
  page.getByRole("link", { name, exact: true }).first();

test("about-link: signed out, the sidebar links straight to the About page", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
    timeout: 30_000,
  });

  await sidebarLink(page, "About").click();
  await expect(page).toHaveURL(/\/settings\/about$/);
  await expect(page.getByText(/^Streamplace v/).first()).toBeVisible();

  // Only the destination you're on lights up, though Settings matches every
  // /settings/* path by prefix.
  await expect(sidebarLink(page, "About")).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(sidebarLink(page, "Settings")).not.toHaveAttribute(
    "aria-current",
    "page",
  );

  // ...and that prefix still carries every other settings page.
  await page.goto("/settings/advanced");
  await expect(page.getByText("Advanced").first()).toBeVisible();
  await expect(sidebarLink(page, "Settings")).toHaveAttribute(
    "aria-current",
    "page",
  );
  await expect(sidebarLink(page, "About")).not.toHaveAttribute(
    "aria-current",
    "page",
  );
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
  await expect(sidebarLink(page, "Log in")).toHaveCount(0);
  await expect(sidebarLink(page, "About")).toHaveCount(0);

  await sidebarLink(page, "Settings").click();
  await expect(page.getByText("Advanced").first()).toBeVisible();
  await page.getByText("About", { exact: true }).first().click();
  await expect(page).toHaveURL(/\/settings\/about$/);
});
