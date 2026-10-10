import { expect, test } from "@playwright/test";

// Web-only: native installs bundle the app and retain the existing entrypoint.
test("bootstrap: paint loading UI before app downloads, below 1 MB", async ({
  page,
  request,
}) => {
  const html = await (await request.get("/")).text();
  const initialScripts = [...html.matchAll(/<script[^>]*src="([^"]+)"/g)].map(
    (match) => match[1],
  );
  let bytes = Buffer.byteLength(html);
  for (const src of initialScripts) {
    const response = await request.get(src);
    expect(response.ok()).toBe(true);
    bytes += (await response.body()).length;
  }
  // Includes the HTML and every initial script, not just the smallest chunk.
  expect(bytes).toBeLessThan(1_000_000);

  let release!: () => void;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(/\/__expo-metro-runtime-[^/]+\.js$/, async (route) => {
    await held;
    await route.continue();
  });
  try {
    await page.goto("/");
    await expect(page.getByRole("status")).toHaveText("Loading Streamplace");
    await expect(page.locator("#bootstrap-spinner")).toBeVisible();
    await expect(page.getByTestId("home-stream-card")).toHaveCount(0);
  } finally {
    release();
  }
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
  await expect(page.locator("#web-bootstrap")).toHaveCount(0);
});

test("bootstrap: retry a failed common chunk without replaying Metro", async ({
  page,
}) => {
  let attempts = 0;
  let runtimeRequests = 0;
  page.on("request", (request) => {
    if (/\/__expo-metro-runtime-[^/]+\.js$/.test(request.url()))
      runtimeRequests++;
  });
  await page.route(/\/__common-[^/]+\.js$/, async (route) => {
    attempts++;
    if (attempts === 1) {
      await route.fulfill({ status: 503, body: "Temporarily unavailable" });
    } else {
      await route.continue();
    }
  });
  await page.goto("/");
  await expect(page.getByRole("status")).toHaveText(
    "Could not load Streamplace. Check your connection.",
  );
  await expect(page.locator("#bootstrap-spinner")).toBeHidden();
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
  expect(attempts).toBe(2);
  expect(runtimeRequests).toBe(1);
});
