import { expect, test, type Page } from "@playwright/test";
import { loginThroughPds } from "./login";

// Automatic VOD publishing is the autoPublishVods field of the account's
// place.stream.server.settings record for this node. Flip it in Privacy &
// Security and back, reloading each time so only the saved record can set the
// toggle. The harness node has no beta-invite issuer, so the test account is
// in the VOD beta that gates the toggle. It records no streams (no S3), so
// publishing itself is covered by the Go tests in pkg/statedb and
// pkg/atproto. Mobile: .maestro/logged-in/auto-publish-vods.yaml.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

async function setAutoPublishVods(page: Page, on: boolean) {
  const toggle = page.getByTestId("settings-auto-publish-vods");
  const saved = page.waitForResponse((response) =>
    new URL(response.url()).pathname.endsWith(
      "/xrpc/com.atproto.repo.putRecord",
    ),
  );
  await toggle.click();
  const response = await saved;
  expect(response.ok(), `putRecord returned ${response.status()}`).toBe(true);
  await expect(toggle).toHaveAttribute("aria-checked", String(on));

  await page.reload();
  await expect(page.getByTestId("settings-auto-publish-vods")).toHaveAttribute(
    "aria-checked",
    String(on),
    { timeout: 30_000 },
  );
}

test("auto-publish-vods: the setting is saved to the settings record", async ({
  page,
}) => {
  await loginThroughPds(page);

  await page.goto(`${HTTPS_URL}/settings/privacy`);
  const toggle = page.getByTestId("settings-auto-publish-vods");
  await expect(toggle).toBeVisible({ timeout: 30_000 });
  await expect(toggle).toHaveAttribute("aria-checked", /^(true|false)$/);
  const initial = (await toggle.getAttribute("aria-checked")) === "true";

  await setAutoPublishVods(page, !initial);
  await setAutoPublishVods(page, initial);
});
