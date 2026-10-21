import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { withInvalidHandle } from "./invalid-handle-fixture";

// Shared behavior: .maestro/logged-out/invalid-handle.yaml. Move the real
// harness account to an unverifiable .test handle, then restore it. Only public
// AppView identity discovery (which doesn't index harness DIDs) and a search
// result boundary are substituted; feed, stream, websocket and playback are real.
const DID = process.env.ACCOUNT_DID!;
test.setTimeout(150_000);

for (const shell of ["expo", "vite"] as const) {
  test(`invalid handle: ${shell} home and search remain DID-addressable`, async ({
    page,
    context,
    baseURL,
  }) => {
    // Vite creates its search agent from OAuth metadata, which requires HTTPS.
    const origin =
      shell === "vite" ? process.env.SERVER_HTTPS_URL || baseURL! : baseURL!;
    if (shell === "vite") {
      await context.addCookies([
        { name: "sp_web_beta", value: "1", url: origin },
      ]);
      await page.addInitScript(() => {
        localStorage.setItem("player-transport", "hls");
      });
    }
    await page.route(
      "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?*",
      (route) =>
        route.fulfill({
          headers: { "Access-Control-Allow-Origin": "*" },
          json: { did: DID, handle: "handle.invalid" },
        }),
    );
    await page.route(
      "**/xrpc/place.stream.live.searchActorsTypeahead*",
      (route) =>
        route.fulfill({
          json: { actors: [{ did: DID, handle: "handle.invalid" }] },
        }),
    );

    await withInvalidHandle(async () => {
      await page.goto(`${origin}/`);
      const card = page
        .getByRole("link")
        .filter({ hasText: "e2e test stream" })
        .first();
      await expect(card).toContainText(DID);
      await expect(card).not.toContainText("handle.invalid");
      await expect(card).not.toContainText(`@${DID}`);
      await card.click();
      await expect
        .poll(() => decodeURIComponent(new URL(page.url()).pathname))
        .toBe(`/${DID}`);
      const video = page.locator("video").first();
      await expect(video).toBeVisible();
      await expect
        .poll(() =>
          video.evaluate((element: HTMLVideoElement) => element.currentTime),
        )
        .toBeGreaterThan(0);
      await expect(
        page.getByText("handle.invalid", { exact: true }),
      ).toHaveCount(0);
      await expect(page.locator('a[href*="handle.invalid"]')).toHaveCount(0);

      if (shell === "vite") {
        await context.grantPermissions(["clipboard-read", "clipboard-write"], {
          origin: new URL(origin).origin,
        });
        await page.getByRole("button", { name: "Copy", exact: true }).click();
        await page
          .getByRole("menuitem", { name: "Copy Link", exact: true })
          .click();
        await expect
          .poll(() => page.evaluate(() => navigator.clipboard.readText()))
          .toBe(`${origin}/${encodeURIComponent(DID)}`);
      }

      await page.goto(`${origin}/search`);
      if (shell === "expo") {
        await page.getByTestId("search-input").fill("broken");
        await page.getByTestId("search-submit").click();
      } else {
        await page.getByRole("searchbox").fill("broken");
      }
      const result = page.getByRole("link").filter({ hasText: DID }).first();
      await expect(result).toBeVisible();
      await expect(result).not.toContainText("handle.invalid");
      await expect(result).not.toContainText(`@${DID}`);
      await result.click();
      await expect
        .poll(() => decodeURIComponent(new URL(page.url()).pathname))
        .toBe(`/${DID}`);
    });
  });
}

test("invalid handle: login typeahead recovers when the same DID verifies again", async ({
  page,
}) => {
  const url = process.env.SERVER_HTTPS_URL;
  test.skip(!url, "The login form requires the harness's HTTPS metadata.");
  const endpoint =
    "https://public.api.bsky.app/xrpc/app.bsky.actor.searchActorsTypeahead";
  await page.route(`${endpoint}*`, (route) => {
    const query = new URL(route.request().url()).searchParams.get("q");
    return route.fulfill({
      headers: { "Access-Control-Allow-Origin": "*" },
      json: {
        actors: [
          {
            did: DID,
            handle: query === "fix" ? "handle.invalid" : "fixed.example.test",
          },
        ],
      },
    });
  });
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/login`);
  const input = page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(input).toBeVisible();
  const invalidResponse = page.waitForResponse(
    (response) =>
      response.url().startsWith(endpoint) &&
      new URL(response.url()).searchParams.get("q") === "fix",
  );
  await input.fill("fix");
  await invalidResponse;
  await expect(page.getByText("handle.invalid", { exact: true })).toHaveCount(
    0,
  );

  await input.fill("fixe");
  await expect(
    page.getByText("fixed.example.test", { exact: true }),
  ).toBeVisible();
  await input.press("Tab");
  await expect(input).toHaveValue("fixed.example.test");
});
