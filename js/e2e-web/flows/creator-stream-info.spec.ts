import { expect, request, test } from "@playwright/test";
import { completePdsLogin, loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("an unsaved title survives reloads, including an intentional clear", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  await expect(title).toHaveValue("e2e test stream");
  let writes = 0;
  page.on("request", (request) => {
    if (
      request.method() === "POST" &&
      /\/xrpc\/(com\.atproto\.repo\.putRecord|place\.stream\.live\.startLivestream)$/.test(
        request.url(),
      )
    )
      writes++;
  });
  await title.fill("An unfinished title");
  await page.reload();
  await expect(page.getByTestId("creator-stream-status")).toHaveText("Live");
  await expect(title).toHaveValue("An unfinished title");
  await title.fill("");
  await page.reload();
  await expect(page.getByTestId("creator-stream-status")).toHaveText("Live");
  await expect(title).toHaveValue("");
  expect(writes).toBe(0);
});

test("title drafts stay with the signed-in creator", async ({ page }) => {
  test.setTimeout(180_000);
  const pdsUrl = process.env.PDS_HTTPS_URL!;
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const handle = `draft${Date.now().toString(36)}.${new URL(pdsUrl).hostname}`;
  const password = "title-draft-test-password";
  const created = await pds.post(
    `${pdsUrl}/xrpc/com.atproto.server.createAccount`,
    {
      data: { handle, email: `${handle}@example.com`, password },
    },
  );
  expect(created.ok()).toBe(true);
  await pds.dispose();

  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  await expect(title).toHaveValue("e2e test stream");
  await title.fill("First creator's draft");
  await page.goto(`${url}/settings/account`);
  await page.getByText("Log out", { exact: true }).click();
  await expect(page).toHaveURL(`${url}/settings`);
  await page.context().clearCookies({ domain: new URL(pdsUrl).hostname });
  await page.goto(`${url}/login`);
  await completePdsLogin(page, { handle, password });
  await expect(page.getByText(`@${handle}`).first()).toBeVisible();
  await page.goto(`${url}/dashboard`);
  await expect(title).toBeVisible();
  await expect(title).not.toHaveValue("First creator's draft");
  await title.fill("Second creator's draft");
  await page.reload();
  await expect(title).toHaveValue("Second creator's draft");

  await page.goto(`${url}/settings/account`);
  await page.getByText("Log out", { exact: true }).click();
  await expect(page).toHaveURL(`${url}/settings`);
  await page.context().clearCookies({ domain: new URL(pdsUrl).hostname });
  await page.goto(`${url}/login`);
  await completePdsLogin(page);
  await expect(
    page.getByText(`@${process.env.ACCOUNT_HANDLE}`).first(),
  ).toBeVisible();
  await page.goto(`${url}/dashboard`);
  await expect(title).toHaveValue("First creator's draft");
});

test("saving a title does not discard a newer draft in another tab", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  await expect(title).toHaveValue("e2e test stream");
  const originalTitle = await title.inputValue();
  const other = await page.context().newPage();
  await other.goto(`${url}/dashboard`);
  const otherTitle = other.getByPlaceholder("Enter your stream title...");
  await expect(otherTitle).toHaveValue(originalTitle);
  try {
    await title.fill("Title to save");
    await otherTitle.fill("Newer title draft");
    const saved = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.record?.title === "Title to save",
    );
    await page.getByTestId("creator-announce").click();
    expect((await saved).ok()).toBe(true);
    await page.reload();
    await expect(title).toHaveValue("Newer title draft");
  } finally {
    await otherTitle.fill(originalTitle);
    const restored = other.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.record?.title === originalTitle,
    );
    await other.getByTestId("creator-announce").click();
    expect((await restored).ok()).toBe(true);
  }
});

test("a saved draft does not replace a later title from another browser", async ({
  page,
  browser,
}) => {
  test.setTimeout(120_000);
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  await expect(title).toHaveValue("e2e test stream");
  const originalTitle = await title.inputValue();
  const otherContext = await browser.newContext({
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
  const other = await otherContext.newPage();
  try {
    await title.fill("Title saved in first browser");
    await page.reload();
    await expect(title).toHaveValue("Title saved in first browser");
    const firstSave = page.waitForResponse(
      (response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.record?.title ===
          "Title saved in first browser",
    );
    await page.getByTestId("creator-announce").click();
    expect((await firstSave).ok()).toBe(true);

    await loginThroughPds(other);
    await other.goto(`${url}/dashboard`);
    const otherTitle = other.getByPlaceholder("Enter your stream title...");
    await expect(otherTitle).toHaveValue("Title saved in first browser");
    await otherTitle.fill("Title changed in another browser");
    const secondSave = other.waitForResponse(
      (response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
        response.request().postDataJSON()?.record?.title ===
          "Title changed in another browser",
    );
    await other.getByTestId("creator-announce").click();
    expect((await secondSave).ok()).toBe(true);
    await page.reload();
    await expect(title).toHaveValue("Title changed in another browser");
  } finally {
    const otherTitle = other.getByPlaceholder("Enter your stream title...");
    const restorePage = (await otherTitle.isVisible()) ? other : page;
    const restoreTitle = restorePage.getByPlaceholder(
      "Enter your stream title...",
    );
    if (await restoreTitle.isVisible()) {
      await restoreTitle.fill(originalTitle);
      const restored = restorePage.waitForResponse(
        (response) =>
          response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
          response.request().postDataJSON()?.record?.title === originalTitle,
      );
      await restorePage.getByTestId("creator-announce").click();
      expect((await restored).ok()).toBe(true);
    }
    await otherContext.close();
  }
});

test("a draft survives the initial stream record arriving late", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const NativeWebSocket = window.WebSocket;
    const state = window as unknown as { holdCreatorStream: boolean };
    state.holdCreatorStream = true;
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        if (
          state.holdCreatorStream &&
          String(url).includes("/api/websocket/")
        ) {
          url = String(url).replace("/api/websocket/", "/api/no-websocket/");
        }
        super(url, protocols);
      }
    };
  });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  await title.fill("Draft before connection");
  await page.evaluate(() => {
    (window as unknown as { holdCreatorStream: boolean }).holdCreatorStream =
      false;
  });
  await expect(page.getByTestId("creator-stream-status")).toHaveText("Live", {
    timeout: 30_000,
  });
  await expect(title).toHaveValue("Draft before connection");
});

test("updating stream details saves the existing announcement", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);

  const title = page.getByPlaceholder("Enter your stream title...");
  const update = page.getByTestId("creator-announce");
  await expect(update).toHaveText("Update Livestream!");
  const originalTitle = await title.inputValue();
  const editedTitle = `Edited stream ${Date.now()}`;
  let announcements = 0;
  page.on("request", (request) => {
    if (request.url().endsWith("/xrpc/place.stream.live.startLivestream")) {
      announcements++;
    }
  });

  await title.fill(editedTitle);
  const form = page.getByTestId("live-stream-form");
  await form.getByRole("button", { name: "Edit content warnings" }).click();
  await form.getByRole("button", { name: "Back to Stream Info" }).click();
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(title).toHaveValue(editedTitle);
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(title).toHaveValue(editedTitle);
  await page.route("**/xrpc/com.atproto.repo.putRecord", (route) =>
    route.abort(),
  );
  await update.click();
  await expect(page.getByText("Error updating livestream")).toBeVisible();
  await expect(title).toHaveValue(editedTitle);
  await page.unrouteAll({ behavior: "wait" });

  const saved = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
      response.request().postDataJSON()?.record?.title === editedTitle,
    { timeout: 15_000 },
  );
  await update.click();
  const response = await saved;
  expect(response.ok()).toBe(true);
  expect(announcements).toBe(0);
  const savedRecord = response.request().postDataJSON();
  expect(savedRecord.collection).toBe("place.stream.livestream");
  expect(savedRecord.rkey).toBeTruthy();

  await page.reload();
  await expect(title).toHaveValue(editedTitle);
  await title.fill(originalTitle);
  await update.click();
  await expect(page.getByText("Livestream updated")).toBeVisible();
});

test("ending an announcement can be cancelled before the stream is changed", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  await expect(status).toHaveText("Live");
  await page.setViewportSize({ width: 390, height: 844 });
  const end = page.getByRole("button", { name: "End Livestream", exact: true });
  await end.click();
  const dialog = page.getByRole("dialog", { name: "End announcement?" });
  await expect(dialog).toBeVisible();
  const confirmation = dialog.getByText(
    "Ending the announcement doesn't stop video from your streaming app. People watching may still receive it. To stop sending video, stop streaming in OBS or your streaming app.",
  );
  const bounds = await confirmation.boundingBox();
  expect(bounds).not.toBeNull();
  expect(bounds!.x).toBeGreaterThanOrEqual(0);
  expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toBeHidden();
  await expect(status).toHaveText("Live");
  await expect(end).toBeVisible();
});
