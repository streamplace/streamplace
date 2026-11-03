import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

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
  const dialog = page.getByRole("dialog", { name: "End livestream?" });
  await expect(dialog).toBeVisible();
  const confirmation = dialog.getByText(
    "This ends the announcement. Incoming video will continue.",
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
