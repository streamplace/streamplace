import { expect, test } from "./fixtures";

// Embed tests: verify embed routes load and render minimal UI.
// These are used by OBS, info widgets, and danmu overlays.

test.describe("embeds", () => {
  test("stream embed (/embed/$user)", async ({ page }) => {
    await page.goto("/embed/test-user");
    await page.waitForLoadState("networkidle");
    // Embed should render a video player or poster.
    const playerOrVideo = page.locator("video, img");
    await expect(playerOrVideo.first()).toBeVisible({ timeout: 10000 });
  });

  test("VOD embed (/embed/$user/video/$tid)", async ({ page }) => {
    await page.goto("/embed/test-user/video/nonexistent");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("danmu OBS embed (/embed/danmu-obs/$user)", async ({ page }) => {
    await page.goto("/embed/danmu-obs/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  test("info widget embed (/embed/info-widget/$user)", async ({ page }) => {
    await page.goto("/embed/info-widget/test-user");
    await page.waitForLoadState("networkidle");
    await expect(page.locator("body")).toBeVisible();
  });

  // Overlays (/overlay/<name>) are the generic OBS widget host. The status
  // widget shows the node's build, which the page can only know from the
  // build manifest — so a version here proves the manifest reached it.
  test("status overlay (/overlay/status)", async ({ page }) => {
    await page.goto("/overlay/status?user=test-user");
    // Read the manifest from the page's own origin, so the assertion is about
    // this frontend reaching the node, not about any particular version.
    const version = await page.evaluate(async () => {
      const res = await fetch("/api/version");
      const manifest: unknown = await res.json();
      if (
        manifest &&
        typeof manifest === "object" &&
        "version" in manifest &&
        typeof manifest.version === "string"
      ) {
        return manifest.version;
      }
      throw new Error("node served no version manifest");
    });
    await expect(page.getByTestId("overlay-status")).toBeVisible();
    // An overlay composites over the operator's scene: nothing between the
    // widget and the page may paint a background, and the page must not wrap
    // it in the site's sidebar/header chrome.
    const painted = await page.evaluate(() => {
      const paintedAncestors: string[] = [];
      let el: Element | null = document.querySelector(
        '[data-testid="overlay-status"]',
      );
      while (el) {
        const style = getComputedStyle(el);
        if (
          style.backgroundColor !== "rgba(0, 0, 0, 0)" &&
          style.backgroundColor !== "transparent"
        ) {
          paintedAncestors.push(
            `${el.tagName} background-color ${style.backgroundColor}`,
          );
        }
        if (style.backgroundImage !== "none") {
          paintedAncestors.push(
            `${el.tagName} background-image ${style.backgroundImage}`,
          );
        }
        el = el.parentElement;
      }
      return paintedAncestors;
    });
    expect(painted).toEqual([]);
    await expect(page.getByTestId("overlay-version")).toHaveText(version);
  });

  test("status overlay reloads itself when the build manifest changes", async ({
    page,
  }) => {
    let version = "v-e2e-1";
    await page.route("**/api/version", (route) =>
      route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          version,
          buildTime: "2026-01-01T00:00:00Z",
          uuid: "00000000-0000-7000-8000-000000000000",
        }),
      }),
    );

    // ?versionPollMs is a string on the URL and a number after the router has
    // parsed the search; this only reloads if the route passes it through.
    await page.goto("/overlay/status?user=test-user&versionPollMs=500");
    await expect(page.getByTestId("overlay-version")).toHaveText("v-e2e-1");

    version = "v-e2e-2";
    await expect(page.getByTestId("overlay-version")).toHaveText("v-e2e-2", {
      timeout: 30_000,
    });
  });

  test("overlay diagnostics (/overlay/<name>)", async ({ page }) => {
    // A URL is an overlay's only interface, so a wrong name and a missing
    // ?user= both have to say so on screen.
    await page.goto("/overlay/not-a-widget?user=test-user");
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      'No overlay named "not-a-widget"',
    );

    // A name that exists on Object.prototype must not resolve to a widget.
    await page.goto("/overlay/__proto__?user=test-user");
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      'No overlay named "__proto__"',
    );

    await page.goto("/overlay/status");
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      "Add ?user=<your handle>",
    );
  });
});
