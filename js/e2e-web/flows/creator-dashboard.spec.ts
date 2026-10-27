import { expect, test, type WebSocket } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { completePdsLogin } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("creator host: login return, restored session, and socket lifetime", async ({
  page,
}) => {
  const sockets = new Set<WebSocket>();
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
