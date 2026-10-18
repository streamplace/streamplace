import { expect, test, type Page } from "@playwright/test";

// Web counterpart of .maestro/logged-out/user-search.yaml. Searches the real
// harness node, which finds the harness account by its handle prefix; only the
// failure and slow-response tests intercept the search request. The web app
// renders the desktop sidebar, so navigation goes through its "Search" link.
const HANDLE = process.env.ACCOUNT_HANDLE!;
const DID = process.env.ACCOUNT_DID!;

// "sp-1a2b3c4d.<domain>" -> "SP-1A2B3C4D": a mixed-case prefix of the handle.
const PREFIX = HANDLE.split(".")[0].toUpperCase();
const SEARCH_URL = "**/xrpc/place.stream.live.searchActorsTypeahead*";

async function search(page: Page, query: string) {
  const input = page.getByTestId("search-input");
  await input.fill(query);
  await page.getByTestId("search-submit").click();
}

test("user-search: find an account by handle prefix and open it", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByRole("link", { name: "Search", exact: true }).first().click();
  await expect(page).toHaveURL(/\/search$/);
  await expect(page.getByTestId("search-hint")).toBeVisible();

  await search(page, `nobody-${Date.now()}`);
  await expect(page.getByTestId("search-no-results")).toBeVisible();
  await expect(page.getByTestId(`search-result-${DID}`)).toBeHidden();

  // case, leading @ and surrounding whitespace don't matter
  await search(page, `  @${PREFIX}  `);
  const result = page.getByTestId(`search-result-${DID}`);
  await expect(result).toBeVisible();
  await expect(result).toContainText(HANDLE);
  await expect(page.getByTestId("search-no-results")).toBeHidden();

  // emptying the box drops the stale result, whatever it was searching for
  await page.getByTestId("search-input").fill("");
  await expect(result).toBeHidden();
  await expect(page.getByTestId("search-hint")).toBeVisible();

  // the keyboard submits too
  await page.getByTestId("search-input").fill(PREFIX);
  await page.getByTestId("search-input").press("Enter");
  await expect(result).toBeVisible();

  // selecting the account opens its stream
  await result.click();
  await expect
    .poll(() => decodeURIComponent(new URL(page.url()).pathname))
    .toBe(`/${HANDLE}`);
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  await expect(page.locator("video").first()).toBeVisible();
});

test("user-search: narrow viewport", async ({ page }) => {
  await page.setViewportSize({ width: 320, height: 568 });
  await page.goto("/search");
  await expect(page.getByTestId("search-hint")).toBeVisible();

  await search(page, PREFIX);
  const result = page.getByTestId(`search-result-${DID}`);
  await expect(result).toBeVisible();
  await expect(result).toContainText(HANDLE);
  const box = await result.boundingBox();
  expect(box!.x).toBeGreaterThanOrEqual(0);
  expect(box!.x + box!.width).toBeLessThanOrEqual(320);
});

test("user-search: a failed request shows an error and can be retried", async ({
  page,
}) => {
  await page.route(SEARCH_URL, (route) =>
    route.fulfill({
      status: 500,
      contentType: "application/json",
      body: JSON.stringify({ error: "InternalServerError" }),
    }),
  );
  await page.goto("/search");
  await search(page, PREFIX);
  await expect(page.getByTestId("search-error")).toBeVisible();
  await expect(page.getByTestId(`search-result-${DID}`)).toBeHidden();

  await page.unroute(SEARCH_URL);
  await page.getByTestId("search-submit").click();
  await expect(page.getByTestId(`search-result-${DID}`)).toBeVisible();
  await expect(page.getByTestId("search-error")).toBeHidden();
});

test("user-search: a slow earlier response doesn't replace a newer search", async ({
  page,
}) => {
  const slowQuery = PREFIX;
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let slowSeen!: () => void;
  const slowRequested = new Promise<void>((resolve) => (slowSeen = resolve));
  await page.route(SEARCH_URL, async (route) => {
    const q = new URL(route.request().url()).searchParams.get("q");
    if (q?.toUpperCase() === slowQuery) {
      slowSeen();
      await released;
    }
    await route.continue();
  });

  await page.goto("/search");
  await search(page, slowQuery);
  await slowRequested;
  await expect(page.getByTestId("search-loading")).toBeVisible();

  await search(page, `nobody-${Date.now()}`);
  await expect(page.getByTestId("search-no-results")).toBeVisible();

  // let the superseded request finish, then give its result time to (not) land
  const slowResponse = page.waitForResponse(
    (response) =>
      new URL(response.url()).searchParams.get("q")?.toUpperCase() ===
      slowQuery,
  );
  release();
  await slowResponse;
  await page.waitForTimeout(500);
  await expect(page.getByTestId(`search-result-${DID}`)).toBeHidden();
  await expect(page.getByTestId("search-no-results")).toBeVisible();
});
