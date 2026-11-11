import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// Real Desktop lifecycle coverage, separate from the full-node web suite.
test("login and writes survive closing Desktop", async ({ desktop }) => {
  const { nodeURL } = desktop;
  const localHost = new URL(nodeURL).host;
  let page = desktop.page;
  await loginThroughSystemBrowser(desktop);
  await page.goto(nodeURL + "/settings/privacy");
  const toggleName = `Allow ${localHost} to record your livestream for debugging and improving the service`;
  const toggle = page.getByRole("switch", { name: toggleName });
  await expect(toggle).toBeVisible();
  const initial = (await toggle.getAttribute("aria-checked")) === "true";
  const saved = page.waitForResponse(
    (response) =>
      new URL(response.url()).origin === nodeURL &&
      new URL(response.url()).pathname === "/xrpc/com.atproto.repo.putRecord",
  );
  await toggle.click();
  expect((await saved).ok()).toBe(true);
  await expect(toggle).toHaveAttribute("aria-checked", String(!initial));
  // Read the actual PDS record: an optimistic toggle alone proves no write.
  const recordURL = new URL(
    "/xrpc/com.atproto.repo.getRecord",
    process.env.PDS_HTTPS_URL!,
  );
  recordURL.search = new URLSearchParams({
    repo: process.env.ACCOUNT_DID!,
    collection: "place.stream.server.settings",
    rkey: localHost,
  }).toString();
  const persisted = await page.request.get(recordURL.toString(), {
    ignoreHTTPSErrors: true,
  });
  expect(persisted.ok()).toBe(true);
  expect((await persisted.json()).value.debugRecording).toBe(!initial);
  page = await desktop.restart();
  await page.goto(nodeURL + "/settings/privacy");
  await expect(page.getByRole("switch", { name: toggleName })).toHaveAttribute(
    "aria-checked",
    String(!initial),
  );
  const restoredWrite = page.waitForResponse(
    (response) =>
      new URL(response.url()).origin === nodeURL &&
      new URL(response.url()).pathname === "/xrpc/com.atproto.repo.putRecord",
  );
  await page.getByRole("switch", { name: toggleName }).click();
  expect((await restoredWrite).ok()).toBe(true);
  await expect(page.getByRole("switch", { name: toggleName })).toHaveAttribute(
    "aria-checked",
    String(initial),
  );
});
