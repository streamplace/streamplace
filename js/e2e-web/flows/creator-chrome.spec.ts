import { expect, request, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("a signed-out creator can dismiss login and return to the app", async ({
  page,
}) => {
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/dashboard`);
  await expect(page.getByTestId("login-submit")).toBeVisible();
  await page.getByRole("button", { name: "Close login" }).click();
  await expect(page.getByTestId("login-submit")).toBeHidden();
  await page.getByRole("button", { name: "Log in", exact: true }).click();
  await expect(page.getByTestId("login-submit")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("login-submit")).toBeHidden();
  await page.getByRole("link", { name: "Back to Streamplace" }).click();
  await expect(page).toHaveURL(`${url}/`);
});

test("creator navigation and drafts remain usable across screen sizes", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);

  const navigation = page.getByTestId("creator-navigation");
  const chat = page.getByPlaceholder("Type a message...");
  await expect(navigation).toBeVisible();
  await expect(
    navigation.getByRole("link", { name: /^control panel$/i }),
  ).toHaveAttribute("aria-current", "page");
  await expect(page.locator("video")).toBeVisible();
  let writes = 0;
  page.on("request", (request) => {
    if (
      request.method() === "POST" &&
      (request.url().endsWith("/xrpc/com.atproto.repo.putRecord") ||
        request.url().endsWith("/xrpc/place.stream.live.stopLivestream"))
    )
      writes++;
  });
  await page.getByRole("button", { name: "Hide preview" }).click();
  await expect(page.locator("video")).toHaveCount(0);
  await page.getByRole("button", { name: "Show preview" }).click();
  await expect(page.locator("video")).toBeVisible();
  expect(writes).toBe(0);
  await chat.fill("An unsent message");
  const title = page.getByPlaceholder("Enter your stream title...");
  await title.fill("An unsaved title");

  await page
    .getByRole("button", { name: "Collapse creator navigation" })
    .click();
  await expect(navigation.getByTitle("Stream Settings")).toBeVisible();
  await expect(chat).toHaveValue("An unsent message");
  await page.getByRole("button", { name: "Expand creator navigation" }).click();

  await page.setViewportSize({ width: 390, height: 844 });
  await expect(chat).toHaveValue("An unsent message");
  await expect(title).toHaveValue("An unsaved title");
  await expect(navigation).toBeHidden();
  await page.getByRole("button", { name: "Open creator navigation" }).click();
  await expect(navigation).toBeVisible();
  await expect(
    page.getByRole("dialog", { name: "Creator navigation" }),
  ).toBeVisible();
  await expect(
    navigation.getByRole("button", { name: "Close creator navigation" }),
  ).toBeFocused();
  await page.keyboard.press("Shift+Tab");
  await expect(
    navigation.getByRole("link", { name: "Webhooks" }),
  ).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(
    navigation.getByRole("button", { name: "Close creator navigation" }),
  ).toBeFocused();
  await page.keyboard.press("Escape");
  await expect(navigation).toBeHidden();
  await expect(
    page.getByRole("button", { name: "Open creator navigation" }),
  ).toBeFocused();
  await expect(chat).toHaveValue("An unsent message");
});

test("a creator can send, reply to, and moderate chat", async ({ page }) => {
  test.setTimeout(180_000);
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);

  const pdsUrl = process.env.PDS_HTTPS_URL!;
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const message = `viewer message ${Date.now()}`;
  const reply = `creator reply ${Date.now()}`;
  let messageUri: string | undefined;
  let viewerDid: string | undefined;
  let viewerToken: string | undefined;
  let ownerToken: string | undefined;
  const ownerRecords: string[] = [];
  let testError: unknown;
  const recordWrite = async (
    collection: string,
    marker: string,
    action: () => Promise<unknown>,
  ) => {
    const [response] = await Promise.all([
      page.waitForResponse(
        (response) =>
          response.request().method() === "POST" &&
          response.url().endsWith("/xrpc/com.atproto.repo.createRecord") &&
          response.request().postData()?.includes(collection) === true &&
          response.request().postData()?.includes(marker) === true,
      ),
      action(),
    ]);
    expect(response.ok(), `createRecord: ${response.status()}`).toBe(true);
    ownerRecords.push((await response.json()).uri);
  };
  try {
    const owner = await pds.post(
      `${pdsUrl}/xrpc/com.atproto.server.createSession`,
      {
        data: {
          identifier: process.env.ACCOUNT_HANDLE,
          password: process.env.ACCOUNT_PASSWORD,
        },
      },
    );
    expect(
      owner.ok(),
      `createSession: ${owner.status()} ${await owner.text()}`,
    ).toBe(true);
    ownerToken = (await owner.json()).accessJwt;
    const hostname = new URL(pdsUrl).hostname;
    const account = await pds.post(
      `${pdsUrl}/xrpc/com.atproto.server.createAccount`,
      {
        data: {
          handle: `v${Date.now().toString(36)}.${hostname}`,
          email: `viewer-${Date.now()}@example.com`,
          password: "viewer-password",
        },
      },
    );
    expect(
      account.ok(),
      `createAccount: ${account.status()} ${await account.text()}`,
    ).toBe(true);
    const { did, accessJwt } = await account.json();
    viewerDid = did;
    viewerToken = accessJwt;
    const created = await pds.post(
      `${pdsUrl}/xrpc/com.atproto.repo.createRecord`,
      {
        headers: { Authorization: `Bearer ${accessJwt}` },
        data: {
          repo: did,
          collection: "place.stream.chat.message",
          record: {
            $type: "place.stream.chat.message",
            text: message,
            createdAt: new Date().toISOString(),
            streamer: process.env.ACCOUNT_DID,
          },
        },
      },
    );
    expect(
      created.ok(),
      `createRecord: ${created.status()} ${await created.text()}`,
    ).toBe(true);
    messageUri = (await created.json()).uri;

    const row = page.getByTestId(`chat-message-${messageUri}`);
    await expect(row.getByText(message, { exact: true })).toBeVisible({
      timeout: 45_000,
    });
    const standalone = `creator message ${Date.now()}`;
    await page.getByPlaceholder("Type a message...").fill(standalone);
    await recordWrite("place.stream.chat.message", standalone, () =>
      page.getByPlaceholder("Type a message...").press("Enter"),
    );
    await expect(
      page.getByText(standalone, { exact: true }).first(),
    ).toBeVisible();
    await expect(row).toHaveAttribute("tabindex", "0");
    await row.focus();
    const replyAction = row.getByRole("button", { name: "Reply to message" });
    for (let tab = 0; tab < 8; tab++) {
      await page.keyboard.press("Tab");
      if (
        await replyAction.evaluate(
          (element) => element === document.activeElement,
        )
      )
        break;
    }
    await expect(replyAction).toBeFocused();
    await page.keyboard.press("Enter");
    await expect(page.getByTestId("chat-reply-preview")).toContainText(message);
    await page.getByPlaceholder("Type a message...").fill(reply);
    await recordWrite("place.stream.chat.message", reply, () =>
      page.getByPlaceholder("Type a message...").press("Enter"),
    );
    await expect(page.getByText(reply, { exact: true }).first()).toBeVisible();

    await row.focus();
    const actions = row.getByRole("button", { name: "Message actions" });
    for (let tab = 0; tab < 8; tab++) {
      await page.keyboard.press("Tab");
      if (
        await actions.evaluate((element) => element === document.activeElement)
      )
        break;
    }
    await expect(actions).toBeFocused();
    await page.keyboard.press("Enter");
    await page.getByText("Pin this message", { exact: true }).hover();
    await recordWrite("place.stream.chat.pinnedRecord", messageUri!, () =>
      page.getByText("Until stream end", { exact: true }).click(),
    );
    await expect(page.getByText("Comment pinned")).toBeVisible();
    await row.hover();
    await row.getByRole("button", { name: "Message actions" }).click();
    await recordWrite("place.stream.chat.gate", messageUri!, () =>
      page.getByText("Hide this message", { exact: true }).click(),
    );
    await expect(row).toBeHidden();
  } catch (error) {
    testError = error;
  }
  const cleanupErrors: unknown[] = [];
  const recordsToDelete = [
    ...ownerRecords.map((uri) => ({
      uri,
      repo: process.env.ACCOUNT_DID!,
      token: ownerToken!,
    })),
    ...(messageUri && viewerDid && viewerToken
      ? [{ uri: messageUri, repo: viewerDid, token: viewerToken }]
      : []),
  ];
  for (const { uri, repo, token } of recordsToDelete) {
    try {
      const parts = uri.split("/");
      const removed = await pds.post(
        `${pdsUrl}/xrpc/com.atproto.repo.deleteRecord`,
        {
          headers: { Authorization: `Bearer ${token}` },
          data: {
            repo,
            collection: parts[parts.length - 2],
            rkey: parts[parts.length - 1],
          },
        },
      );
      expect(
        removed.ok(),
        `deleteRecord: ${removed.status()} ${await removed.text()}`,
      ).toBe(true);
    } catch (error) {
      cleanupErrors.push(error);
    }
  }
  try {
    await pds.dispose();
  } catch (error) {
    cleanupErrors.push(error);
  }
  if (cleanupErrors.length) {
    throw new AggregateError(
      testError ? [testError, ...cleanupErrors] : cleanupErrors,
      "creator chat test cleanup failed",
    );
  }
  if (testError) throw testError;
});
