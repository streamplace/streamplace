import { expect, test } from "@playwright/test";

// Native counterpart: .maestro/logged-out/lazy-navigation.yaml. Only web
// exports fetch separate chunks; native imports resolve from the installed bundle.
test("lazy navigation: defer settings until opened and keep shell usable", async ({
  page,
}) => {
  const scripts: string[] = [];
  page.on("request", (request) => {
    if (request.resourceType() === "script") scripts.push(request.url());
  });

  await page.goto("/");
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
  expect(scripts.some((url) => /\/settings-[^/]+\.js$/.test(url))).toBe(false);
  expect(
    scripts.some((url) => /\/advanced-category-settings-[^/]+\.js$/.test(url)),
  ).toBe(false);

  let releaseChunk!: () => void;
  const heldChunk = new Promise<void>((resolve) => {
    releaseChunk = resolve;
  });
  await page.route(/\/settings-[^/]+\.js$/, async (route) => {
    await heldChunk;
    await route.continue();
  });

  try {
    await page
      .getByRole("link", { name: "Settings", exact: true })
      .first()
      .click();
    await expect(page.getByLabel("Loading screen")).toBeVisible();
    await expect(
      page.getByRole("link", { name: "Home", exact: true }).first(),
    ).toBeVisible();
    // Leaving a pending screen must not leave the entire app suspended.
    await page.getByRole("link", { name: "Home", exact: true }).first().click();
    await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
  } finally {
    releaseChunk();
  }

  await page
    .getByRole("link", { name: "Settings", exact: true })
    .first()
    .click();
  await expect(page.getByText("Advanced", { exact: true })).toBeVisible();
  expect(scripts.some((url) => /\/settings-[^/]+\.js$/.test(url))).toBe(true);
  expect(
    scripts.some((url) => /\/advanced-category-settings-[^/]+\.js$/.test(url)),
  ).toBe(false);

  await page.getByText("Advanced", { exact: true }).click();
  await expect(page.getByTestId("settings-use-custom-node")).toBeVisible();
  expect(
    scripts.some((url) => /\/advanced-category-settings-[^/]+\.js$/.test(url)),
  ).toBe(true);

  // A cold deep link must load the same category without visiting Settings first.
  await page.reload();
  await expect(page.getByTestId("settings-use-custom-node")).toBeVisible();
});
