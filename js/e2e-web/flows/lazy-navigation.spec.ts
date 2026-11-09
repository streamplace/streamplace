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

test("lazy navigation: retry a failed chunk without losing navigation", async ({
  page,
}) => {
  let attempts = 0;
  await page.route(/\/settings-[^/]+\.js$/, async (route) => {
    attempts++;
    if (attempts === 1) {
      await route.fulfill({ status: 503, body: "Temporarily unavailable" });
    } else {
      await route.continue();
    }
  });

  await page.goto("/");
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
  const deploymentCheck = page.waitForResponse(
    (response) =>
      response.request().resourceType() === "fetch" &&
      new URL(response.url()).pathname === "/settings",
  );
  await page
    .getByRole("link", { name: "Settings", exact: true })
    .first()
    .click();
  await expect(page.getByRole("button", { name: "Try again" })).toBeVisible();
  await deploymentCheck;
  await expect(
    page
      .getByTestId("app-crash-screen")
      .getByText("Streamplace", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Home", exact: true }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByText("Advanced", { exact: true })).toBeVisible();
  expect(attempts).toBe(2);
  await page.getByRole("link", { name: "Home", exact: true }).first().click();
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
});

test("lazy navigation: a new deployment reloads the requested category", async ({
  page,
}) => {
  // Serve the old document once, then expose the new deployment's real HTML.
  await page.route("**/", async (route) => {
    if (!route.request().isNavigationRequest()) return route.continue();
    const response = await route.fetch();
    const body = (await response.text()).replace(
      /(<script id="bootstrap-scripts" type="application\/json">)(.*?)(<\/script>)/,
      (_, start, manifest, end) =>
        start +
        JSON.stringify(
          JSON.parse(manifest).map((src: string) => `${src}?deployment=old`),
        ) +
        end,
    );
    await route.fulfill({ response, body });
  });
  let attempts = 0;
  await page.route(/\/advanced-category-settings-[^/]+\.js$/, async (route) => {
    if (++attempts === 1) {
      await route.fulfill({ status: 404, body: "Removed by deployment" });
    } else {
      await route.continue();
    }
  });

  await page.goto("/");
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
  await page
    .getByRole("link", { name: "Settings", exact: true })
    .first()
    .click();
  await page.getByText("Advanced", { exact: true }).click();
  await expect(page.getByTestId("settings-use-custom-node")).toBeVisible();
  await expect(page).toHaveURL(/\/settings\/advanced$/);
  expect(
    await page.evaluate(
      () =>
        (
          performance.getEntriesByType(
            "navigation",
          )[0] as PerformanceNavigationTiming
        ).type,
    ),
  ).toBe("reload");
  expect(attempts).toBe(2);
});

test("web bundles: immutable scripts and revalidated deep-link HTML", async ({
  page,
  request,
}) => {
  const scripts = new Map<string, string>();
  page.on("response", (response) => {
    if (
      response.request().resourceType() === "script" &&
      new URL(response.url()).origin === new URL(page.url()).origin
    ) {
      scripts.set(response.url(), response.headers()["cache-control"]);
    }
  });
  const document = await page.goto("/settings/advanced");
  expect(document?.headers()["cache-control"]).toBe("no-cache");
  await expect(page.getByTestId("settings-use-custom-node")).toBeVisible();
  expect(
    [...scripts.keys()].some((url) =>
      /\/advanced-category-settings-[^/]+\.js$/.test(url),
    ),
  ).toBe(true);
  for (const cacheControl of scripts.values()) {
    expect(cacheControl).toBe("public, max-age=31536000, immutable");
  }
  const missing = await request.get(
    "/_expo/static/js/web/settings-00000000000000000000000000000000.js",
  );
  expect(missing.status()).toBe(404);
  expect(missing.headers()["cache-control"]).toBe("no-store");
  expect(missing.headers()["content-type"]).not.toContain("text/html");
});
