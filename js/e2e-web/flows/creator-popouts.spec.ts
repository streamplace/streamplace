import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { completePdsLogin, loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("a signed-out viewer returns to the same chat popout after login", async ({
  page,
}) => {
  await pointAppAtNode(page, url!);
  const user = process.env.ACCOUNT_DID!;
  await page.goto(`${url}/chat-popout/${user}?reverse=true&hideAfter=30`);
  await page.getByText("Log in or sign up to chat").click();
  await completePdsLogin(page);
  await expect
    .poll(() => decodeURI(new URL(page.url()).pathname))
    .toBe(`/chat-popout/${user}`);
  await expect(page.getByPlaceholder("Type a message...")).toBeVisible();
  const returned = new URL(page.url());
  expect(returned.searchParams.get("reverse")).toBe("true");
  expect(returned.searchParams.get("hideAfter")).toBe("30");
});

test("a creator can open chat in a separate window without losing the dashboard", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const chat = page.getByPlaceholder("Type a message...");
  await chat.fill("Draft on the dashboard");
  const popupPromise = page.waitForEvent("popup");
  await page.getByTestId("chat-popout-button").click();
  const popup = await popupPromise;
  await expect
    .poll(() => decodeURI(new URL(popup.url()).pathname))
    .toBe(`/chat-popout/${process.env.ACCOUNT_DID}`);
  await expect(popup.getByPlaceholder("Type a message...")).toBeVisible();
  await popup.getByPlaceholder("Type a message...").fill("Draft in popout");
  await expect(page).toHaveURL(`${url}/dashboard`);
  await expect(chat).toHaveValue("Draft on the dashboard");
  await popup.close();
  await expect(chat).toHaveValue("Draft on the dashboard");
});

test("creator widget popouts remain usable in independent windows", async ({
  page,
  context,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const chat = page.getByPlaceholder("Type a message...");
  await chat.fill("Dashboard draft");
  const popup = await context.newPage();
  await popup.setViewportSize({ width: 480, height: 600 });
  await popup.goto(`${url}/widgets/stream-monitor`);
  await expect(popup.getByTestId("creator-stream-status")).toHaveText("Live");
  await expect(popup.locator("video")).toBeVisible();
  await popup.goto(`${url}/widgets/info`);
  const bitrate = popup.getByText("Bitrate", { exact: true }).first();
  await expect(bitrate).toBeVisible();
  await expect
    .poll(async () => (await bitrate.boundingBox())!.x)
    .toBeLessThan(60);
  await expect(popup.getByText(/\d+(\.\d+)? Mbps/).first()).toBeVisible({
    timeout: 30_000,
  });
  await popup.goto(`${url}/widgets/multistream`);
  await expect(popup.getByTestId("live-stream-destinations")).toBeVisible();
  await popup.goto(`${url}/widgets/livestream`);
  await expect(
    popup.getByPlaceholder("Enter your stream title..."),
  ).toBeVisible();
  await popup.close();
  await expect(page).toHaveURL(`${url}/dashboard`);
  await expect(chat).toHaveValue("Dashboard draft");
});

test("leaving theatre mode by browser history restores ordinary navigation", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/settings/streaming`);
  await expect(
    page.getByRole("link", { name: "Home", exact: true }),
  ).toBeVisible();
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Theatre mode" }).click();
  await page.goBack();
  await expect(page).toHaveURL(`${url}/settings/streaming`);
  await expect(
    page.getByRole("link", { name: "Home", exact: true }),
  ).toBeVisible();
});

test("public player and OBS overlay URLs connect to the requested stream", async ({
  page,
}) => {
  await pointAppAtNode(page, url!);
  const user = process.env.ACCOUNT_DID!;
  const sockets: string[] = [];
  page.on("websocket", (socket) => sockets.push(socket.url()));
  await page.goto(`${url}/embed/${user}`);
  await expect(page.locator("video")).toBeVisible();
  await expect
    .poll(
      () =>
        sockets.filter((socket) => socket.endsWith(`/api/websocket/${user}`))
          .length,
    )
    .toBeGreaterThan(0);
  const before = sockets.length;
  await page.goto(`${url}/widgets/${user}/danmu?opacity=80`);
  await expect
    .poll(
      () =>
        sockets
          .slice(before)
          .filter((socket) => socket.endsWith(`/api/websocket/${user}`)).length,
    )
    .toBeGreaterThan(0);
  await expect(page.getByTestId("login-submit")).toHaveCount(0);
});
