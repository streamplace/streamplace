import { expect, test, type Page } from "@playwright/test";

const SERVER_URL = process.env.SERVER_URL;
const ACCOUNT_HANDLE = process.env.ACCOUNT_HANDLE;

test.skip(!SERVER_URL || !ACCOUNT_HANDLE, "needs the e2e harness node");

/**
 * Every element between the status widget and the page, listing any that paint
 * a background. An overlay has to composite over the operator's scene, so this
 * must be empty.
 */
async function opaqueAncestors(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const painted: string[] = [];
    let el: Element | null = document.querySelector(
      '[data-testid="overlay-status"]',
    );
    while (el) {
      const style = getComputedStyle(el);
      if (
        style.backgroundColor !== "rgba(0, 0, 0, 0)" &&
        style.backgroundColor !== "transparent"
      ) {
        painted.push(`${el.tagName} background-color ${style.backgroundColor}`);
      }
      if (style.backgroundImage !== "none") {
        painted.push(`${el.tagName} background-image ${style.backgroundImage}`);
      }
      el = el.parentElement;
    }
    return painted;
  });
}

// Overlays (/overlay/<name>) are OBS browser sources. These flows cover the
// shell every widget sits in: the ?user= identity, the upstream websocket,
// the build manifest the shell reloads on, and the diagnostics an operator
// gets when the URL is wrong. No login: an overlay is an anonymous page.
test.describe("overlays", () => {
  test("status overlay shows the node build, the streamer, and a connected socket", async ({
    page,
  }) => {
    // What the node says its build is, so the assertion is about the manifest
    // actually reaching the widget — not about any particular version string.
    const manifest = await fetch(`${SERVER_URL}/api/version`);
    expect(manifest.ok).toBe(true);
    const body: unknown = await manifest.json();
    if (
      !body ||
      typeof body !== "object" ||
      !("version" in body) ||
      typeof body.version !== "string"
    ) {
      throw new Error("node served no version manifest");
    }
    const version = body.version;

    await page.goto(`/overlay/status?user=${ACCOUNT_HANDLE}`);

    await expect(page.getByTestId("overlay-status")).toBeVisible({
      timeout: 30_000,
    });
    // An overlay composites over the operator's scene: nothing between the
    // widget and the page may paint a background (OBS's default custom CSS
    // only clears <body>, and the app paints <html> too).
    const painted = await opaqueAncestors(page);
    expect(painted).toEqual([]);
    await expect(page.getByTestId("overlay-streamer")).toContainText(
      ACCOUNT_HANDLE as string,
    );
    await expect(page.getByTestId("overlay-version")).toHaveText(version);
    // The websocket reports itself connected once the node's messages (the
    // looping test stream) start arriving.
    await expect(page.getByTestId("overlay-upstream")).toHaveText("Connected", {
      timeout: 30_000,
    });
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

    await page.goto(`/overlay/status?user=${ACCOUNT_HANDLE}&versionPollMs=500`);
    await expect(page.getByTestId("overlay-version")).toHaveText("v-e2e-1");

    // A deploy replaces the build behind this URL. The next poll must reload,
    // so the page comes back up on the new manifest — the displayed version
    // only ever changes on a fresh boot, which is what proves the reload.
    version = "v-e2e-2";
    await expect(page.getByTestId("overlay-version")).toHaveText("v-e2e-2", {
      timeout: 30_000,
    });
  });

  test("unknown overlay name lists the available ones", async ({ page }) => {
    await page.goto(`/overlay/not-a-widget?user=${ACCOUNT_HANDLE}`);

    await expect(page.getByTestId("overlay-diagnostic")).toBeVisible();
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      'No overlay named "not-a-widget"',
    );
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      "Available overlays: status",
    );

    // A name that exists on Object.prototype must not resolve to a widget.
    await page.goto(`/overlay/__proto__?user=${ACCOUNT_HANDLE}`);
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      'No overlay named "__proto__"',
    );
  });

  test("missing ?user= explains what the URL needs", async ({ page }) => {
    await page.goto("/overlay/status");

    await expect(page.getByTestId("overlay-diagnostic")).toBeVisible();
    await expect(page.getByTestId("overlay-diagnostic")).toContainText(
      "Add ?user=<your handle>",
    );
  });
});
