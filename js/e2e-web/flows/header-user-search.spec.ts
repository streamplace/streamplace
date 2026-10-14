import { expect, test, type Page } from "@playwright/test";

// The header Search field shows on wide windows only (the sidebar layout,
// >= 981px; the same breakpoint the player-breakpoints flow covers). It
// searches the real harness node, which finds the harness account by its
// handle prefix, as the user types, over whatever page they are on. Only the
// held-request tests intercept the search request.
const HANDLE = process.env.ACCOUNT_HANDLE!;
const DID = process.env.ACCOUNT_DID!;
const VIDEO_URI = process.env.VIDEO_URI;

const PREFIX = HANDLE.split(".")[0].toUpperCase();
const SEARCH_URL = "**/xrpc/place.stream.live.searchActorsTypeahead*";

const input = (page: Page) => page.getByTestId("header-search-input");
const dropdown = (page: Page) => page.getByTestId("header-search-dropdown");
const dismiss = (page: Page) => page.getByTestId("header-search-dismiss");
const result = (page: Page) => page.getByTestId(`header-search-result-${DID}`);
const noResults = (page: Page) => page.getByTestId("header-search-no-results");

const queryOf = (url: string) =>
  new URL(url).searchParams.get("q")?.toLowerCase();

// Holds every search request for `query` until release() is called, and
// resolves `requested` once the app has sent it.
async function holdQuery(page: Page, query: string) {
  let release!: () => void;
  const released = new Promise<void>((resolve) => (release = resolve));
  let seen!: () => void;
  const requested = new Promise<void>((resolve) => (seen = resolve));
  await page.route(SEARCH_URL, async (route) => {
    if (queryOf(route.request().url()) === query.toLowerCase()) {
      seen();
      await released;
    }
    await route.continue();
  });
  return { requested, release };
}

// Lets a held response reach the app, and gives its result time to (not) land.
async function deliver(page: Page, query: string, release: () => void) {
  const response = page.waitForResponse(
    (r) => queryOf(r.url()) === query.toLowerCase(),
  );
  release();
  await response;
  await page.waitForTimeout(500);
}

test("header-user-search: searches while typing without leaving the page", async ({
  page,
}) => {
  await page.goto("/settings");
  await input(page).click();
  // no submit: results arrive as the keystrokes do
  await input(page).pressSequentially(`nobody-${Date.now()}`);
  await expect(noResults(page)).toBeVisible();
  await expect(result(page)).toBeHidden();

  await input(page).fill("");
  await expect(dropdown(page)).toBeHidden();

  await input(page).pressSequentially(PREFIX);
  await expect(result(page)).toBeVisible();
  await expect(result(page)).toContainText(HANDLE);
  await expect(noResults(page)).toBeHidden();
  await expect(page).toHaveURL(/\/settings$/);

  // selecting the account opens its stream, under the same header
  await result(page).click();
  await expect
    .poll(() => decodeURIComponent(new URL(page.url()).pathname))
    .toBe(`/${HANDLE}`);
  await expect(dropdown(page)).toBeHidden();
  await expect(input(page)).toBeVisible();
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
});

test("header-user-search: empty input leaves page controls usable", async ({
  page,
}) => {
  await page.goto("/settings");
  await input(page).fill(PREFIX);
  await expect(result(page)).toBeVisible();
  await input(page).fill("");
  await expect(input(page)).toBeFocused();

  // One real click must reach the page, not an invisible dismiss layer.
  await page.getByText("Advanced", { exact: true }).click({ force: true });
  await expect(page).toHaveURL(/\/settings\/advanced$/);

  // Focusing an empty field must not install that layer either.
  await input(page).click();
  await page.getByRole("link", { name: "Home", exact: true }).click({
    force: true,
  });
  await expect(page).toHaveURL(/\/$/);
});

test("header-user-search: Escape and an outside press close the results", async ({
  page,
}) => {
  await page.goto("/settings");
  await expect(dismiss(page)).toHaveCount(0);

  await input(page).pressSequentially(PREFIX);
  await expect(result(page)).toBeVisible();
  await input(page).press("Escape");
  await expect(dropdown(page)).toBeHidden();
  await expect(dismiss(page)).toHaveCount(0);

  // typing again brings the results back
  await input(page).pressSequentially("-");
  await input(page).press("Backspace");
  await expect(result(page)).toBeVisible();

  // the invisible dismiss layer takes the press, not the page underneath
  await dismiss(page).click();
  await expect(dropdown(page)).toBeHidden();
  await expect(dismiss(page)).toHaveCount(0);
  await expect(page).toHaveURL(/\/settings$/);
});

test("header-user-search: the latest query wins over an earlier held request", async ({
  page,
}) => {
  const final = `${PREFIX}-nobody-${Date.now()}`;
  const slow = await holdQuery(page, PREFIX);
  await page.goto("/settings");

  // The first request (a match for the account) is still out while the user
  // keeps typing; the final text has to be searched anyway.
  const finalRequest = page.waitForRequest(
    (r) => queryOf(r.url()) === final.toLowerCase(),
  );
  await input(page).fill(PREFIX);
  await slow.requested;
  await input(page).pressSequentially(final.slice(PREFIX.length));
  await finalRequest;
  await expect(noResults(page)).toBeVisible();

  await deliver(page, PREFIX, slow.release);
  await expect(result(page)).toBeHidden();
  await expect(noResults(page)).toBeVisible();
});

for (const how of ["clearing the field", "pressing Escape"]) {
  test(`header-user-search: a late response after ${how} stays hidden`, async ({
    page,
  }) => {
    const slow = await holdQuery(page, PREFIX);
    await page.goto("/settings");
    await input(page).fill(PREFIX);
    await slow.requested;

    if (how === "pressing Escape") {
      await input(page).press("Escape");
    } else {
      await input(page).fill("");
    }
    await deliver(page, PREFIX, slow.release);
    await expect(dropdown(page)).toBeHidden();
    await expect(result(page)).toBeHidden();
  });
}

test("header-user-search: shows on wide windows only, following resizes", async ({
  page,
}) => {
  await page.goto("/settings");
  await expect(input(page)).toBeVisible();

  await page.setViewportSize({ width: 980, height: 800 });
  await expect(input(page)).toHaveCount(0);
  await page.setViewportSize({ width: 981, height: 800 });
  await expect(input(page)).toBeVisible();

  // an open dropdown goes with the field
  await input(page).pressSequentially(PREFIX);
  await expect(result(page)).toBeVisible();
  await page.setViewportSize({ width: 320, height: 568 });
  await expect(input(page)).toHaveCount(0);
  await expect(dropdown(page)).toHaveCount(0);

  // search stays reachable through its own page
  await page.goto("/search");
  await expect(page.getByTestId("search-input")).toBeVisible();
  await expect(input(page)).toHaveCount(0);
  await page.getByTestId("search-input").fill(PREFIX);
  await page.getByTestId("search-submit").click();
  await expect(page.getByTestId(`search-result-${DID}`)).toBeVisible();
});

test("header-user-search: present on home, stream and video pages", async ({
  page,
}) => {
  const video = VIDEO_URI?.match(
    /^at:\/\/([^/]+)\/place\.stream\.video\/([^/]+)$/,
  );
  expect(
    video,
    `VIDEO_URI is not a place.stream.video URI: ${VIDEO_URI}`,
  ).toBeTruthy();
  for (const path of ["/", `/${HANDLE}`, `/${video![1]}/video/${video![2]}`]) {
    await page.goto(path);
    await expect(input(page), path).toBeVisible();
  }
});
