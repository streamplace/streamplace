import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const HTTPS_URL = process.env.SERVER_HTTPS_URL;
// HTML bootstrap is web-only. Real branding edits require the OAuth/HTTPS harness.
test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("bootstrap: persisted branding colors paint before application scripts", async ({
  page,
  context,
}) => {
  test.setTimeout(4 * 60_000);
  await loginThroughPds(page);
  await page.goto(`${HTTPS_URL}/settings/branding`);

  // Restore the whole original bundle through the same admin UI in finally.
  // The suite uses one worker, so no other flow edits this node concurrently.
  const downloading = page.waitForEvent("download");
  await page
    .getByRole("button", { name: "Download bundle", exact: true })
    .click();
  const original = test.info().outputPath("original-branding.zip");
  await (await downloading).saveAs(original);
  const preview = await context.newPage();
  let release = () => {};
  try {
    for (const [key, color] of [
      ["primaryColor", "#22d3ee"],
      ["backgroundColor", "#122334"],
    ]) {
      await page.getByTestId(`branding-input-${key}`).fill(color);
      const updated = page.waitForResponse((response) =>
        response.url().endsWith("/xrpc/place.stream.branding.updateBlob"),
      );
      await page.getByTestId(`branding-update-${key}`).click();
      expect((await updated).ok()).toBe(true);
    }

    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    await preview.route(/\/__expo-metro-runtime-[^/]+\.js$/, async (route) => {
      await held;
      await route.continue();
    });
    await preview.goto(HTTPS_URL!, { waitUntil: "domcontentloaded" });
    await expect(
      preview.getByRole("progressbar", { name: "Loading" }),
    ).toBeVisible();
    await expect(preview.locator("#bootstrap-spinner")).toHaveCSS(
      "color",
      "rgb(34, 211, 238)",
    );
    for (const selector of ["html", "body", "#web-bootstrap"]) {
      await expect(preview.locator(selector)).toHaveCSS(
        "background-color",
        "rgb(18, 35, 52)",
      );
    }
    await expect(preview.locator("#web-bootstrap")).toHaveJSProperty(
      "innerText",
      "",
    );
    await expect(preview.getByTestId("home-stream-card")).toHaveCount(0);
    const screenshot = test.info().outputPath("branded-bootstrap.png");
    await preview.screenshot({ path: screenshot });
    await test.info().attach("branded-bootstrap", { path: screenshot });
    release();
    await expect(preview.getByTestId("home-stream-card").first()).toBeVisible();
    await expect(preview.locator("#web-bootstrap")).toHaveCount(0);
  } finally {
    release();
    await preview.close();
    const choosing = page.waitForEvent("filechooser");
    await page
      .getByRole("button", { name: "Import bundle…", exact: true })
      .click();
    const dryRun = page.waitForResponse((response) =>
      response.url().includes("/xrpc/place.stream.branding.importBundle"),
    );
    await (await choosing).setFiles(original);
    expect((await dryRun).ok()).toBe(true);
    const applying = page.waitForResponse((response) =>
      response.url().includes("/xrpc/place.stream.branding.importBundle"),
    );
    await page.getByRole("button", { name: "Apply", exact: true }).click();
    expect((await applying).ok()).toBe(true);
  }
});
