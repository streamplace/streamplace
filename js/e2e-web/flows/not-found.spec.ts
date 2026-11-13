import { expect, test } from "@playwright/test";

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
        if (request.url().includes("actor=does-not-exist")) {
          lookups.push(request.url());
        }
      });
      for (const path of [
        "/does-not-exist",
        "/does-not-exist/?q=example.com:123",
      ]) {
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
      }
      await test.info().attach("404 page", {
        body: await page.screenshot({
          path: test.info().outputPath("404.png"),
        }),
        contentType: "image/png",
      });
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

    test("missing handles and DIDs keep their stream-specific state", async ({
      page,
    }) => {
      // Public AppView is outside the local harness; only its missing-profile
      // response is substituted. Routing and the stream websocket remain real.
      await page.route(
        "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?*",
        (route) =>
          route.fulfill({
            status: 400,
            headers: { "Access-Control-Allow-Origin": "*" },
            json: { error: "InvalidRequest", message: "Profile not found" },
          }),
      );
      for (const user of [
        "missing.example.invalid",
        "did:plc:aaaaaaaaaaaaaaaaaaaaaaaa",
      ]) {
        await page.goto(`/${user}`);
        await expect(
          page.getByText(beta ? "Creator not found" : "Connecting...", {
            exact: true,
          }),
        ).toBeVisible();
        await expect(page.getByText("404 — Page not found")).toHaveCount(0);
      }
    });

    test("handles and DIDs still open a playable stream", async ({ page }) => {
      const did = process.env.ACCOUNT_DID;
      const handle = process.env.ACCOUNT_HANDLE;
      if (!did || !handle) throw new Error("harness account required");
      // The local harness account isn't indexed by the public AppView.
      await page.route(
        "https://public.api.bsky.app/xrpc/app.bsky.actor.getProfile?*",
        (route) =>
          route.fulfill({
            headers: { "Access-Control-Allow-Origin": "*" },
            json: { did, handle },
          }),
      );
      for (const user of [handle, did]) {
        await page.goto(`/${user}`);
        const video = page.locator("video").first();
        await expect(video).toBeVisible();
        await expect
          .poll(() =>
            video.evaluate((element: HTMLVideoElement) => element.currentTime),
          )
          .toBeGreaterThan(0);
      }
    });
  });
}
