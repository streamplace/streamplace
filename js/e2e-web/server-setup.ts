import { expect, type Page } from "@playwright/test";

// Point the app served at `url` back at that same node — the web counterpart
// of .maestro/00-server-setup.yaml. The app ships pointed at production, so
// open Settings -> Advanced (react-navigation's linking config exposes it at
// /settings/advanced), turn on "use custom node" and enter the node's URL.
// redux-persist keeps the choice in localStorage, which is per origin: do this
// once for every origin a flow visits.
export async function pointAppAtNode(page: Page, url: string) {
  await page.goto(`${url}/settings/advanced`);

  // toggle "use custom node" on, which reveals the URL field + save button
  const toggle = page.getByTestId("settings-use-custom-node");
  await expect(toggle).toBeVisible({ timeout: 60_000 });
  await toggle.click();

  // the react-native-web TextInput renders as an <input>; fill it
  const urlField = page
    .locator(
      '[data-testid="settings-custom-node-url"], [data-testid="settings-custom-node-url"] input',
    )
    .first();
  await expect(urlField).toBeVisible();
  await urlField.fill(url);

  await page.getByTestId("settings-save-node").click();

  // setURL writes storage asynchronously; don't navigate away before it lands
  await page.waitForFunction(
    (u) =>
      Object.values(localStorage).some(
        (v) => v === u || v === JSON.stringify(u),
      ),
    url,
  );
}
