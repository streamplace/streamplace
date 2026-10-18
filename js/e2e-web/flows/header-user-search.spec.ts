import { expect, test, type Locator, type Page } from "@playwright/test";
import {
  avatarUrl,
  fixtureActors,
  stubProfiles,
  stubSearch,
  type Actor,
} from "./search-fixtures";

// The header Search field shows on wide windows only (the sidebar layout,
// >= 981px; the same breakpoint the player-breakpoints flow covers). It
// searches the real harness node, which finds the harness account by its
// handle prefix, as the user types, over whatever page they are on. Only the
// held-request tests, and the ones needing many accounts or profile pictures
// (see search-fixtures.ts), intercept the search request.
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

const rowOf = (page: Page, actor: Actor) =>
  page.getByTestId(`header-search-result-${actor.did}`);
const avatarOf = (page: Page, actor: Actor) =>
  page.getByTestId(`header-search-avatar-${actor.did}`);

const escapeRegExp = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

// The harness account among fixture accounts, as a search would list it.
const harnessActor: Actor = { did: DID, handle: HANDLE };

// Content can arrive in steps; wait for two matching measurements.
async function settledBox(locator: Locator) {
  let box = await locator.boundingBox();
  for (let i = 0; i < 20; i++) {
    await locator.page().waitForTimeout(100);
    const next = await locator.boundingBox();
    if (box && next && box.y === next.y && box.height === next.height) {
      return next;
    }
    box = next;
  }
  throw new Error("box never settled");
}

// Every result sits inside the dropdown, which has nothing around them but
// its own border and padding: neither clipping rows nor stretching past them.
// Returns the dropdown's height.
async function expectFitsRows(page: Page, actors: Actor[]) {
  const box = await settledBox(dropdown(page));
  const first = (await rowOf(page, actors[0]).boundingBox())!;
  const last = (await rowOf(page, actors[actors.length - 1]).boundingBox())!;
  const slack = 12;
  expect(first.y).toBeGreaterThanOrEqual(box.y);
  expect(first.y - box.y).toBeLessThanOrEqual(slack);
  expect(last.y + last.height).toBeLessThanOrEqual(box.y + box.height + 0.5);
  expect(box.y + box.height - (last.y + last.height)).toBeLessThanOrEqual(
    slack,
  );
  return box.height;
}

// Which result is highlighted, as a screen reader would find out.
async function expectActive(
  page: Page,
  actors: Actor[],
  active: number | null,
) {
  for (const [i, actor] of actors.entries()) {
    await expect(rowOf(page, actor)).toHaveAttribute(
      "aria-selected",
      String(i === active),
    );
  }
  if (active === null) {
    await expect(input(page)).not.toHaveAttribute("aria-activedescendant", /./);
  } else {
    await expect(input(page)).toHaveAttribute(
      "aria-activedescendant",
      `header-search-result-${actors[active].did}`,
    );
  }
}

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
    await route.fallback();
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

test("header-user-search: retains results while the replacement query is pending", async ({
  page,
}) => {
  const next = `${PREFIX}-nobody`;
  const pending = await holdQuery(page, next);
  await page.goto("/settings");
  await input(page).fill(PREFIX);
  await expect(result(page)).toBeVisible();
  const before = await settledBox(dropdown(page));

  await input(page).fill(next);
  await pending.requested;
  await expect(result(page)).toBeVisible();
  await expect(page.getByTestId("header-search-loading")).toBeHidden();
  expect((await settledBox(dropdown(page))).height).toBe(before.height);
  // The retained list remains usable, not just a visual placeholder.
  await input(page).press("ArrowDown");
  await expectActive(page, [harnessActor], 0);

  await deliver(page, next, pending.release);
  await expect(result(page)).toBeHidden();
  await expect(noResults(page)).toBeVisible();
  await expect(input(page)).not.toHaveAttribute("aria-activedescendant", /./);
  await input(page).fill(PREFIX);
  await expect(result(page)).toBeVisible();
  await expectActive(page, [harnessActor], null);
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

// Three accounts are enough that "one row" and "maximum height" are both
// visibly wrong.
test("header-user-search: a short list fits its rows on first response and on reopening", async ({
  page,
}) => {
  const actors = fixtureActors(3);
  await stubSearch(page, actors);
  await stubProfiles(page, actors);
  const held = await holdQuery(page, "search-fixture");
  await page.goto("/settings");

  await input(page).fill("search-fixture");
  await held.requested;
  await expect(page.getByTestId("header-search-loading")).toBeVisible();
  // Let the loading popup settle before its content changes.
  await page.waitForTimeout(200);
  await deliver(page, "search-fixture", held.release);
  for (const actor of actors) await expect(rowOf(page, actor)).toBeVisible();
  const first = await expectFitsRows(page, actors);

  await input(page).press("Escape");
  await expect(dropdown(page)).toBeHidden();
  await input(page).click();
  for (const actor of actors) await expect(rowOf(page, actor)).toBeVisible();
  const reopened = await expectFitsRows(page, actors);
  expect(Math.abs(reopened - first)).toBeLessThanOrEqual(1);
});

test("header-user-search: a long list stays within the window and scrolls", async ({
  page,
}) => {
  const actors = fixtureActors(20);
  await stubSearch(page, actors);
  await stubProfiles(page, actors);
  await page.goto("/settings");

  await input(page).fill("search-fixture");
  await expect(rowOf(page, actors[0])).toBeVisible();
  const box = await settledBox(dropdown(page));
  expect(box.y + box.height).toBeLessThanOrEqual(page.viewportSize()!.height);
  // room for several accounts, not a sliver
  expect(box.height).toBeGreaterThanOrEqual(3 * 56);

  const last = rowOf(page, actors[actors.length - 1]);
  await expect(last).not.toBeInViewport();
  await dropdown(page).hover();
  await page.mouse.wheel(0, 5000);
  await expect(last).toBeInViewport({ ratio: 0.9 });
  await expect(rowOf(page, actors[0])).not.toBeInViewport();
});

test("header-user-search: arrow keys highlight accounts and Enter opens the highlighted one", async ({
  page,
}) => {
  const [before, after] = fixtureActors(2);
  const actors = [before, harnessActor, after];
  await stubSearch(page, actors);
  await stubProfiles(page, []);
  await page.goto("/settings");

  await input(page).fill("search-fixture");
  await expect(input(page)).toHaveAttribute("role", "combobox");
  await expect(dropdown(page)).toHaveAttribute("role", "listbox");
  for (const actor of actors) {
    const row = rowOf(page, actor);
    await expect(row).toBeVisible();
    await expect(row).toHaveAttribute("role", "option");
    await expect(row).toHaveAttribute(
      "id",
      `header-search-result-${actor.did}`,
    );
  }
  await expectActive(page, actors, null);
  await input(page).press("ArrowUp");
  await expectActive(page, actors, 2); // from no selection, starts at the end

  await input(page).press("ArrowDown");
  await expectActive(page, actors, 0);
  await input(page).press("ArrowUp");
  await expectActive(page, actors, 2); // wraps to the end
  await input(page).press("ArrowDown");
  await expectActive(page, actors, 0); // and back to the start
  await input(page).press("ArrowDown");
  await expectActive(page, actors, 1);

  // highlighting alone goes nowhere
  await expect(page).toHaveURL(/\/settings$/);
  await input(page).press("Enter");
  await expect
    .poll(() => decodeURIComponent(new URL(page.url()).pathname))
    .toBe(`/${HANDLE}`);
  await expect(dropdown(page)).toBeHidden();
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
});

test("header-user-search: editing or Escape clears the highlight", async ({
  page,
}) => {
  const actors = fixtureActors(3);
  await stubSearch(page, actors);
  await stubProfiles(page, actors);
  await page.goto("/settings");

  await input(page).fill("search-fixture");
  await expect(rowOf(page, actors[0])).toBeVisible();
  await input(page).press("ArrowDown");
  await expectActive(page, actors, 0);

  await input(page).press("x");
  await expectActive(page, actors, null);
  // still clear once the throttled search for the new text has landed
  await page.waitForTimeout(600);
  await expect(rowOf(page, actors[0])).toBeVisible();
  await expectActive(page, actors, null);

  await input(page).press("ArrowDown");
  await input(page).press("ArrowDown");
  await expectActive(page, actors, 1);
  await input(page).press("Escape");
  await expect(dropdown(page)).toBeHidden();
  await expect(page).toHaveURL(/\/settings$/);

  await input(page).press("y");
  await expect(rowOf(page, actors[0])).toBeVisible();
  await expectActive(page, actors, null);
});

test("header-user-search: arrow keys keep the highlighted account in view", async ({
  page,
}) => {
  const actors = fixtureActors(20);
  await stubSearch(page, actors);
  await stubProfiles(page, actors);
  await page.goto("/settings");

  await input(page).fill("search-fixture");
  await expect(rowOf(page, actors[0])).toBeVisible();
  await expect(rowOf(page, actors[19])).not.toBeInViewport();

  await input(page).press("ArrowUp");
  await expectActive(page, actors, 19);
  await expect(rowOf(page, actors[19])).toBeInViewport({ ratio: 0.9 });
  await expect(rowOf(page, actors[0])).not.toBeInViewport();

  // wrapping to the start scrolls back up; then walk well past the first
  // screenful, one account per press
  await input(page).press("ArrowDown");
  await expectActive(page, actors, 0);
  await expect(rowOf(page, actors[0])).toBeInViewport({ ratio: 0.9 });
  for (let i = 1; i <= 12; i++) {
    await input(page).press("ArrowDown");
    await expectActive(page, actors, i);
    await expect(rowOf(page, actors[i])).toBeInViewport({ ratio: 0.9 });
  }
});

test("header-user-search: results show real profile pictures, and live accounts are marked", async ({
  page,
}) => {
  // The harness account is really live (the app's own live-users poller says
  // so); the fixture accounts are not, and only the first has a picture.
  const [pictured, bare] = fixtureActors(2);
  const actors = [harnessActor, pictured, bare];
  await stubSearch(page, actors);
  await stubProfiles(page, [harnessActor, pictured]);
  await page.goto("/settings");

  await input(page).fill("search-fixture");
  for (const actor of actors) await expect(rowOf(page, actor)).toBeVisible();

  for (const actor of [harnessActor, pictured]) {
    const picture = avatarOf(page, actor).locator("img");
    await expect(picture).toBeVisible();
    await expect
      .poll(() =>
        picture.evaluate((el: HTMLImageElement) => ({
          loaded: el.complete,
          width: el.naturalWidth,
          src: el.currentSrc,
        })),
      )
      .toEqual({ loaded: true, width: 64, src: avatarUrl(actor.did) });
  }
  await expect(avatarOf(page, bare).locator("img")).toHaveCount(0);
  await expect(avatarOf(page, bare)).toContainText("S");

  // the live ring takes up room of its own, inside the row
  const live = avatarOf(page, harnessActor);
  const offline = avatarOf(page, pictured);
  await expect(live).toHaveAttribute("aria-label", /\blive\b/i);
  await expect(live).toHaveAttribute(
    "aria-label",
    new RegExp(escapeRegExp(HANDLE)),
  );
  await expect(offline).toHaveAttribute(
    "aria-label",
    new RegExp(escapeRegExp(pictured.handle)),
  );
  await expect(offline).not.toHaveAttribute("aria-label", /live/i);
  const ringWidth = (avatar: Locator) =>
    avatar.evaluate((el) => getComputedStyle(el).borderTopWidth);
  expect(await ringWidth(live)).toBe("2px");
  expect(await ringWidth(offline)).toBe("0px");
  const liveBox = (await live.boundingBox())!;
  expect(liveBox.width).toBeGreaterThan((await offline.boundingBox())!.width);
  const liveRow = (await rowOf(page, harnessActor).boundingBox())!;
  expect(liveBox.y).toBeGreaterThanOrEqual(liveRow.y);
  expect(liveBox.y + liveBox.height).toBeLessThanOrEqual(
    liveRow.y + liveRow.height,
  );
});
