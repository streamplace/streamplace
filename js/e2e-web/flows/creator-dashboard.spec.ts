import {
  expect,
  test,
  type WebSocket as PlaywrightWebSocket,
} from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { completePdsLogin, loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("creator host: login return, restored session, and socket lifetime", async ({
  page,
}) => {
  const sockets = new Set<PlaywrightWebSocket>();
  page.on("websocket", (socket) => {
    if (!socket.url().endsWith(`/api/websocket/${process.env.ACCOUNT_DID}`))
      return;
    sockets.add(socket);
    socket.on("close", () => sockets.delete(socket));
  });
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/dashboard`);
  await completePdsLogin(page);
  const form = page.getByTestId("live-stream-form");
  await expect(form).toBeVisible();
  await expect(page).toHaveURL(`${url}/dashboard`);
  await expect.poll(() => sockets.size).toBe(1);

  const chat = page.getByPlaceholder("Type a message...");
  await chat.fill("Unsent creator draft");
  await form.getByText("Metadata", { exact: true }).click();
  await form.getByText("Create", { exact: true }).click();
  await expect(chat).toHaveValue("Unsent creator draft");
  expect(sockets.size).toBe(1);

  await page.getByRole("button", { name: "Open menu", exact: true }).click();
  await expect(
    page.getByRole("link", { name: "Live streaming", exact: true }),
  ).toHaveAttribute("aria-current", "page");
  await page.getByRole("link", { name: "Home", exact: true }).click();
  await expect.poll(() => sockets.size).toBe(0);
  await page.goBack();
  await expect(form).toBeVisible();
  await expect.poll(() => sockets.size).toBe(1);
  await expect(chat).toHaveValue("");

  await page.reload();
  await expect(form).toBeVisible();
  await expect(page.getByTestId("login-submit")).toBeHidden();
  await expect.poll(() => sockets.size).toBe(1);

  await page.goto(`${url}/live`);
  await expect(form).toBeVisible();
  await expect.poll(() => sockets.size).toBe(1);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: "Home", exact: true }).click();
  await expect(page).toHaveURL(`${url}/`);
  await expect.poll(() => sockets.size).toBe(0);
  await page.goBack();
  await expect(form).toBeVisible();
  await expect.poll(() => sockets.size).toBe(1);
});

test("creator status: reconnects without writes and ends only the announcement", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const NativeWebSocket = window.WebSocket;
    const state = window as unknown as {
      creatorSockets: WebSocket[];
      creatorDisconnected: boolean;
    };
    state.creatorSockets = [];
    state.creatorDisconnected = false;
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const live = String(url).includes("/api/websocket/");
        if (live && state.creatorDisconnected) {
          url = String(url).replace("/api/websocket/", "/api/no-websocket/");
        }
        super(url, protocols);
        if (live) state.creatorSockets.push(this);
      }
    };
  });
  let writes = 0;
  page.on("request", (request) => {
    if (
      request.method() === "POST" &&
      (request.url().endsWith("/xrpc/com.atproto.repo.putRecord") ||
        request.url().endsWith("/xrpc/place.stream.live.stopLivestream"))
    )
      writes++;
  });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  const announce = page.getByTestId("creator-announce");
  await expect(status).toHaveText("Live");
  await expect(announce).toBeEnabled();
  const before = writes;
  const attempts = await page.evaluate(
    () =>
      (window as unknown as { creatorSockets: WebSocket[] }).creatorSockets
        .length,
  );
  await page.evaluate(() => {
    const state = window as unknown as {
      creatorSockets: WebSocket[];
      creatorDisconnected: boolean;
    };
    state.creatorDisconnected = true;
    state.creatorSockets.forEach((socket) => socket.close());
  });
  await expect(status).toHaveText("Reconnecting");
  await expect(page.getByText("Readings stale", { exact: true })).toBeVisible();
  await expect(announce).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "End Livestream", exact: true }),
  ).toBeDisabled();
  await expect(page.locator("video")).toHaveCount(1);

  await expect
    .poll(
      () =>
        page.evaluate(
          () =>
            (window as unknown as { creatorSockets: WebSocket[] })
              .creatorSockets.length,
        ),
      { timeout: 35_000 },
    )
    .toBeGreaterThan(attempts + 20);

  await page.evaluate(() => {
    (
      window as unknown as { creatorDisconnected: boolean }
    ).creatorDisconnected = false;
  });
  await expect(status).toHaveText("Live");
  await expect(announce).toBeEnabled();
  expect(writes).toBe(before);

  await page
    .getByRole("button", { name: "End Livestream", exact: true })
    .click();
  await expect(status).toHaveText("Announcement ended");
  await expect(
    page.getByText(
      "This announcement has ended. Incoming video is independent.",
      {
        exact: true,
      },
    ),
  ).toHaveCount(1);
  await expect(announce).toHaveText("Announce stream");
  const time = await page.evaluate(
    () => document.querySelector("video")!.currentTime,
  );
  await page.waitForFunction(
    (time) => document.querySelector("video")!.currentTime > time + 1,
    time,
  );
  await announce.click();
  await expect(status).toHaveText("Live");
});
