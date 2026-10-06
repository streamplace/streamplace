import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// A branding admin can upload separate favicons for light and dark color
// schemes beside the generic one. The tab shows the variant for the current
// scheme, follows a scheme change without a reload, and falls back to the
// generic favicon (then the bundled one) when a variant is removed. Web-only:
// native apps have no favicon, and branding uploads are web-only.
//
// The harness account is the node's branding admin (SP_ADMIN_DIDS in
// pkg/cmd/e2e.go); uploads go through the app's own file chooser and the
// node's authenticated XRPC, so this needs the harness's HTTPS mode.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

const ICONS: Record<string, string> = {
  favicon: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><rect width="32" height="32" fill="#808080"/></svg>`,
  faviconLight: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><circle cx="16" cy="16" r="12" fill="#ffd000"/></svg>`,
  faviconDark: `<svg xmlns="http://www.w3.org/2000/svg" width="32" height="32"><circle cx="16" cy="16" r="12" fill="#2040ff"/></svg>`,
};

type Icon = { type: string; body: string };
const icon = (key: string): Icon => ({
  type: "image/svg+xml",
  body: ICONS[key],
});

const xrpc = (method: string) => (response: { url(): string }) =>
  new URL(response.url()).pathname === `/xrpc/place.stream.branding.${method}`;

// The icon a browser shows for the current color scheme: the last
// <link rel="icon"> in <head> whose media applies (Chromium and Firefox both
// prefer the last one declared), read from the hydrated page or from its HTML
// as the node serves it. SVG bodies come back as text, anything else base64;
// a link typed differently from what its href serves reports both types, and
// a failed read reports its error, so a poll retries it.
async function shownIcon(page: Page, source: "dom" | "html"): Promise<Icon> {
  const read = page.evaluate(async (source) => {
    let head = document.head;
    if (source === "html") {
      const html = await fetch(location.href, { cache: "no-store" });
      head = new DOMParser().parseFromString(
        await html.text(),
        "text/html",
      ).head;
    }
    const links = Array.from(
      head.querySelectorAll<HTMLLinkElement>(':scope > link[rel~="icon"]'),
    ).filter((link) => !link.media || matchMedia(link.media).matches);
    const link = links[links.length - 1];
    if (!link) return { type: "no applicable icon link", body: "" };
    const href = new URL(link.getAttribute("href")!, location.href).href;
    // Hydrated links use the browser's normal cache. Static HTML checks ask
    // the server directly, since its public icon routes cache for five minutes.
    const res = await fetch(
      href,
      source === "html" ? { cache: "no-store" } : undefined,
    );
    const served = res.headers.get("content-type")?.split(";")[0] ?? "";
    const type =
      link.type && link.type !== served
        ? `${link.type} (served ${served})`
        : served;
    if (served === "image/svg+xml") return { type, body: await res.text() };
    let binary = "";
    for (const byte of new Uint8Array(await res.arrayBuffer())) {
      binary += String.fromCharCode(byte);
    }
    return { type, body: btoa(binary) };
  }, source);
  return read.catch((e: Error) => ({ type: `error: ${e.message}`, body: "" }));
}

async function expectShown(
  page: Page,
  scheme: "light" | "dark",
  expected: Icon,
  source: "dom" | "html" = "dom",
) {
  await page.emulateMedia({ colorScheme: scheme });
  await expect
    .poll(() => shownIcon(page, source), {
      message: `${scheme} ${source} favicon`,
    })
    .toEqual(expected);
}

async function uploadIcon(page: Page, key: string) {
  const updated = page.waitForResponse(xrpc("updateBlob"));
  const chooser = page.waitForEvent("filechooser");
  await page.getByTestId(`branding-upload-${key}`).click();
  await (
    await chooser
  ).setFiles({
    name: `${key}.svg`,
    mimeType: "image/svg+xml",
    buffer: Buffer.from(ICONS[key]),
  });
  expect((await updated).ok()).toBe(true);
  await expect(page.getByTestId(`branding-preview-${key}`)).toBeVisible();
}

async function removeIcon(page: Page, key: string) {
  const deleted = page.waitForResponse(xrpc("deleteBlob"));
  await page.getByTestId(`branding-delete-${key}`).click();
  expect((await deleted).ok()).toBe(true);
  await expect(page.getByTestId(`branding-preview-${key}`)).toBeHidden();
}

// The page can hydrate branding from server metadata without a getBranding
// request. Clean up only slots with a preview, avoiding unset-key 404s.
async function openBrandingWithoutIcons(page: Page) {
  await page.goto(`${HTTPS_URL}/settings/branding`);
  await expect(page.getByTestId("branding-upload-favicon")).toBeVisible();
  for (const key of Object.keys(ICONS)) {
    if (await page.getByTestId(`branding-preview-${key}`).isVisible()) {
      await removeIcon(page, key);
    }
  }
}

test.afterEach(async ({ page }) => {
  await openBrandingWithoutIcons(page);
});

test("branding-favicons: light and dark favicons follow the color scheme", async ({
  page,
}) => {
  // OAuth login plus several uploads and reloads outlast the default 90s.
  test.setTimeout(4 * 60_000);

  await loginThroughPds(page);
  await openBrandingWithoutIcons(page);
  // With no favicon branded, the tab shows the app's bundled icon.
  await page.emulateMedia({ colorScheme: "light" });
  await expect
    .poll(async () => (await shownIcon(page, "dom")).type)
    .toBe("image/png");
  const bundled = await shownIcon(page, "dom");

  // The background-backed default works in either scheme on its own.
  await uploadIcon(page, "favicon");
  await expectShown(page, "light", icon("favicon"));
  await expectShown(page, "dark", icon("favicon"));

  // Transparent variants override it independently without altering the files.
  for (const key of ["faviconLight", "faviconDark"])
    await uploadIcon(page, key);

  // Each scheme gets its own icon, live: no reload between the switches.
  await expectShown(page, "light", icon("faviconLight"));
  await expectShown(page, "dark", icon("faviconDark"));

  const screenshot = test.info().outputPath("branding-favicons.png");
  await page
    .getByTestId("branding-preview-faviconDark")
    .scrollIntoViewIfNeeded();
  await page.screenshot({ path: screenshot });
  await test.info().attach("branding-favicons", { path: screenshot });

  // Persisted on the node: a fresh page load, both hydrated and as served.
  await page.reload();
  for (const source of ["dom", "html"] as const) {
    await expectShown(page, "dark", icon("faviconDark"), source);
    await expectShown(page, "light", icon("faviconLight"), source);
  }

  // Prime the same HTTP cache used by the initial HTML's favicon links.
  // Deleting uploads must not bring these cached images back into the tab.
  await page.evaluate(async () => {
    await fetch("/favicon.png");
    await fetch("/favicon.png?scheme=dark");
  });

  // Without its own variant, a scheme falls back to the generic favicon...
  await removeIcon(page, "faviconDark");
  await expectShown(page, "dark", icon("favicon"));
  await expectShown(page, "dark", icon("favicon"), "html");
  await expectShown(page, "light", icon("faviconLight"));

  // ...and without that, to the bundled one.
  await removeIcon(page, "favicon");
  await expectShown(page, "dark", bundled);
  await expectShown(page, "light", icon("faviconLight"));
});
