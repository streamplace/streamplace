import { expect, test, type Page } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { stubSearch } from "./search-fixtures";

async function expectPlayback(page: Page) {
  const video = page.locator("video").first();
  await expect(video).toBeVisible();
  await expect
    .poll(() =>
      video.evaluate((element: HTMLVideoElement) => element.currentTime),
    )
    .toBeGreaterThan(0);
}

// Shared native counterpart: .maestro/logged-out/not-found.yaml (iOS/Android).
for (const beta of [false, true]) {
  test.describe(beta ? "Vite not-found" : "Expo not-found", () => {
    test.beforeEach(async ({ context, baseURL }) => {
      if (!baseURL) throw new Error("harness URL required");
      await context.addCookies([
        { name: "sp_web_beta", value: beta ? "1" : "0", url: baseURL },
      ]);
    });

    test("non-account paths show a generic 404 with a working home action", async ({
      page,
    }) => {
      const lookups: string[] = [];
      page.on("request", (request) => {
        if (
          request.url().includes("actor=does-not-exist") ||
          request.url().includes("repo=does-not-exist")
        ) {
          lookups.push(request.url());
        }
      });
      for (const { path, width } of [
        { path: "/does-not-exist", width: 390 },
        { path: "/does-not-exist/?q=example.com:123", width: 1280 },
      ]) {
        await page.setViewportSize({ width, height: 844 });
        await page.goto(path);
        await expect(
          page.getByRole("heading", { name: "404 — Page not found" }),
        ).toBeVisible();
        await expect(
          page.getByText("The page you're looking for doesn't exist.", {
            exact: true,
          }),
        ).toBeVisible();
        await expect(page.locator("video")).toHaveCount(0);
        const home = beta
          ? page
              .locator("section")
              .getByRole("link", { name: "Home", exact: true })
          : page.getByRole("button", { name: "Home", exact: true });
        await expect
          .poll(async () => (await home.boundingBox())?.width ?? Infinity)
          .toBeLessThan(width / 2);
        await test.info().attach(`404 at ${width}px`, {
          body: await page.screenshot({
            path: test.info().outputPath(`404-${width}.png`),
          }),
          contentType: "image/png",
        });
      }
      expect(lookups).toEqual([]);
      const main = page
        .locator("section")
        .filter({ hasText: "404 — Page not found" });
      if (beta) {
        await main.getByRole("link", { name: "Home", exact: true }).click();
      } else {
        await page.getByRole("button", { name: "Home", exact: true }).click();
      }
      await expect(page).toHaveURL(/\/$/);
      if (beta) {
        await expect(
          page.getByRole("link", { name: /e2e test stream/ }),
        ).toBeVisible();
      } else {
        await expect(
          page.getByTestId("home-stream-card").first(),
        ).toBeVisible();
      }
      await page.goto("/settings");
      await expect(page.getByText("Advanced", { exact: true })).toBeVisible();
    });

    test("missing handles and DIDs show a user-not-found page", async ({
      page,
    }) => {
      for (const user of [
        "missing.example.invalid",
        "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa",
      ]) {
        await page.goto(`/${user}`);
        await expect(
          page.getByRole("heading", { name: "User not found", exact: true }),
        ).toBeVisible();
        await expect(
          page.getByText(
            `We couldn't find an Atmosphere account for ${user.startsWith("did:") ? user : `@${user}`}.`,
            { exact: true },
          ),
        ).toBeVisible();
        await expect(page.locator("video")).toHaveCount(0);
        await expect(page.getByText("@user", { exact: true })).toHaveCount(0);
        await expect(page.getByText("404 — Page not found")).toHaveCount(0);
        await page.screenshot({
          path: test
            .info()
            .outputPath(
              user.startsWith("did:")
                ? "missing-did.png"
                : "missing-handle.png",
            ),
        });
      }
    });

    test("handles and DIDs still open a playable stream", async ({ page }) => {
      const did = process.env.ACCOUNT_DID;
      const handle = process.env.ACCOUNT_HANDLE;
      if (!did || !handle) throw new Error("harness account required");
      // Existence must not depend on a public Bluesky profile.
      await page.route("https://public.api.bsky.app/**", (route) =>
        route.fulfill({
          status: 400,
          headers: { "Access-Control-Allow-Origin": "*" },
          json: { error: "InvalidRequest", message: "Profile not found" },
        }),
      );
      for (const user of [handle, did]) {
        await page.goto(`/${user}`);
        await expectPlayback(page);
      }
    });

    test("lookup outages offer retry instead of reporting a missing user", async ({
      page,
    }) => {
      const did = process.env.ACCOUNT_DID;
      if (!did) throw new Error("harness account required");
      let unavailable = true;
      await page.route("**/xrpc/com.atproto.repo.describeRepo?*", (route) =>
        unavailable
          ? route.fulfill({ status: 503, body: "temporarily unavailable" })
          : route.fallback(),
      );
      await page.goto(`/${did}`);
      await expect(
        page.getByRole("heading", { name: "We couldn't load this creator" }),
      ).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "User not found", exact: true }),
      ).toHaveCount(0);
      await expect(page.locator("video")).toHaveCount(0);
      unavailable = false;
      await page
        .getByRole("button", { name: "Try again", exact: true })
        .click();
      await expectPlayback(page);
    });

    test("switching accounts waits for the new lookup instead of reusing the player", async ({
      page,
      context,
    }) => {
      const did = process.env.ACCOUNT_DID;
      const handle = process.env.ACCOUNT_HANDLE;
      if (!did || !handle) throw new Error("harness account required");
      let origin = "";
      if (beta) {
        // Vite's header search requires an authenticated PDS agent.
        origin = process.env.SERVER_HTTPS_URL!;
        await context.addCookies([
          { name: "sp_web_beta", value: "0", url: origin },
        ]);
        await pointAppAtNode(page, origin);
        await context.addCookies([
          { name: "sp_web_beta", value: "1", url: origin },
        ]);
        await page.goto(`${origin}/login`);
        await page.locator("#login-handle").fill(handle);
        await page
          .getByRole("button", { name: "Continue", exact: true })
          .click();
        await page.waitForURL(
          (url) => url.origin === process.env.PDS_HTTPS_URL,
        );
        await page
          .locator('input[type="password"]')
          .fill(process.env.ACCOUNT_PASSWORD!);
        await page.getByRole("button", { name: /^(sign in|next)$/i }).click();
        await Promise.race([
          page.getByRole("button", { name: /^authorize$/i }).click(),
          page.waitForURL((url) => url.origin === origin),
        ]);
        await page.waitForURL(`${origin}/`);
      }
      await page.goto(`${origin}/${did}`);
      await expectPlayback(page);
      const missing = {
        did: "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa",
        handle: "missing.example.invalid",
      };
      // Only typeahead discovery is substituted; the selected account's
      // missing identity is resolved by the real node.
      await stubSearch(page, [missing]);
      const { promise: held, resolve: release } = Promise.withResolvers<void>();
      const { promise: lookupStarted, resolve: requested } =
        Promise.withResolvers<void>();
      await page.route(
        "**/xrpc/com.atproto.repo.describeRepo?*",
        async (route) => {
          if (
            new URL(route.request().url()).searchParams.get("repo") ===
            missing.handle
          ) {
            requested();
            await held;
          }
          await route.fallback();
        },
      );
      try {
        await page.getByRole("combobox", { name: /Search/ }).fill("missing");
        const result = beta
          ? page.getByRole("option").filter({ hasText: missing.handle })
          : page.getByTestId(`header-search-result-${missing.did}`);
        await result.click();
        await lookupStarted;
        await expect(page.locator("video")).toHaveCount(0);
        await expect(page.getByText("@user", { exact: true })).toHaveCount(0);
        release();
        await expect(
          page.getByRole("heading", { name: "User not found", exact: true }),
        ).toBeVisible();
        await stubSearch(page, [{ did, handle }]);
        await page.getByRole("combobox", { name: /Search/ }).fill(handle);
        await (
          beta
            ? page.getByRole("option").filter({ hasText: handle })
            : page.getByTestId(`header-search-result-${did}`)
        ).click();
        await expectPlayback(page);
      } finally {
        release();
      }
    });
  });
}
