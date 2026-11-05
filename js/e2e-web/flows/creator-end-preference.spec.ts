import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("the end confirmation can be disabled and restored on this browser", async ({
  page,
  browser,
}) => {
  test.setTimeout(180_000);
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  await expect(status).toHaveText("Live");
  const otherTab = await page.context().newPage();
  await otherTab.goto(`${url}/dashboard`);
  const otherStatus = otherTab.getByTestId("creator-stream-status");
  await expect(otherStatus).toHaveText("Live");

  const isolated = await browser.newContext({
    ignoreHTTPSErrors: true,
    ...(process.env.E2E_PROXY_URL
      ? {
          proxy: {
            server: process.env.E2E_PROXY_URL,
            bypass: "127.0.0.1,localhost",
          },
        }
      : {}),
  });
  const separateBrowser = await isolated.newPage();
  let stops = 0;
  let writes = 0;
  for (const tab of [page, otherTab]) {
    tab.on("request", (request) => {
      if (
        request.method() === "POST" &&
        request.url().endsWith("/xrpc/place.stream.live.stopLivestream")
      )
        stops++;
      if (
        request.method() === "POST" &&
        /\/xrpc\/(com\.atproto\.repo\.(putRecord|createRecord|deleteRecord)|place\.stream\.live\.(startLivestream|stopLivestream))$/.test(
          request.url(),
        )
      )
        writes++;
    });
  }
  try {
    await page.setViewportSize({ width: 390, height: 844 });
    await page
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    await expect(dialog).toBeVisible();
    await dialog
      .getByRole("button", { name: "Don't show this again on this device" })
      .click({ timeout: 3_000 });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByText("This confirmation is off in this browser."),
    ).toBeVisible();
    expect(stops).toBe(0);
    expect(writes).toBe(0);
    await dialog
      .getByRole("button", { name: "Show this confirmation on this device" })
      .click();
    await expect(
      dialog.getByText("This confirmation is off in this browser."),
    ).toBeHidden();
    await dialog
      .getByRole("button", { name: "Don't show this again on this device" })
      .click();
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
    await page.reload();
    await expect(status).toHaveText("Live");

    await loginThroughPds(separateBrowser);
    await separateBrowser.goto(`${url}/dashboard`);
    await expect(
      separateBrowser.getByTestId("creator-stream-status"),
    ).toHaveText("Live");
    await separateBrowser
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const separateDialog = separateBrowser.getByRole("dialog", {
      name: "End announcement?",
    });
    await expect(separateDialog).toBeVisible();
    await separateDialog.getByRole("button", { name: "Cancel" }).click();

    await otherTab
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    await expect(otherStatus).toHaveText("Announcement ended");
    await expect(
      otherTab.getByRole("dialog", { name: "End announcement?" }),
    ).toBeHidden();
    expect(stops).toBe(1);
    await otherTab.getByTestId("creator-announce").click();
    await expect(otherStatus).toHaveText("Live");
    await expect(status).toHaveText("Live");

    await page
      .getByRole("button", { name: "Stream options", exact: true })
      .click();
    const confirm = page.getByRole("checkbox", {
      name: "Show end confirmation on this device",
    });
    await expect(confirm).not.toBeChecked();
    const writesBeforeSetting = writes;
    await confirm.click();
    await expect(confirm).toBeChecked();
    expect(writes).toBe(writesBeforeSetting);
    await otherTab
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const restoredDialog = otherTab.getByRole("dialog", {
      name: "End announcement?",
    });
    await expect(restoredDialog).toBeVisible();
    await restoredDialog.getByRole("button", { name: "Cancel" }).click();
    expect(stops).toBe(1);
  } finally {
    await isolated.close();
    const announce = otherTab.getByTestId("creator-announce");
    if (await announce.getByText("Announce stream").isVisible()) {
      await announce.click();
      await expect(otherStatus).toHaveText("Live");
    }
    await otherTab.close();
  }
});
