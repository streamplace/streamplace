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
    // An overlay composites over the operator's scene: neither <html> nor
    // <body> may paint a background of its own.
    const backgrounds = await page.evaluate(() => [
      getComputedStyle(document.documentElement).backgroundColor,
      getComputedStyle(document.body).backgroundColor,
    ]);
    expect(backgrounds).toEqual(["rgba(0, 0, 0, 0)", "rgba(0, 0, 0, 0)"]);
    await expect(page.getByTestId("overlay-version")).toHaveText(version);
  });

  test("overlay diagnostics (/overlay/<name>)", async ({ page }) => {
    // A URL is an overlay's only interface, so a wrong name and a missing
    // ?user= both have to say so on screen.
    await page.goto("/overlay/not-a-widget?user=test-user");
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      'No overlay named "not-a-widget"',
    );
    await page.goto("/overlay/status");
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      "Add ?user=<your handle>",
    );
  });
});
