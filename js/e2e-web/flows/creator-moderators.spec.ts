import { expect, request, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("the stream popout retains moderator management", async ({ page }) => {
  await loginThroughPds(page);
  await page.goto(`${url}/widgets/livestream`);
  const form = page.getByTestId("live-stream-form");
  await form.getByText("Moderation", { exact: true }).click();
  await expect(form.getByText("Moderators", { exact: true })).toBeVisible();
});

test("a creator can add and remove a moderator", async ({ page }) => {
  const pdsUrl = process.env.PDS_HTTPS_URL!;
  const pds = await request.newContext({ ignoreHTTPSErrors: true });
  const owner = await pds.post(
    `${pdsUrl}/xrpc/com.atproto.server.createSession`,
    {
      data: {
        identifier: process.env.ACCOUNT_HANDLE,
        password: process.env.ACCOUNT_PASSWORD,
      },
    },
  );
  expect(owner.ok()).toBe(true);
  const ownerToken = (await owner.json()).accessJwt;
  const handle = `mod${Date.now().toString(36)}.${new URL(pdsUrl).hostname}`;
  const account = await pds.post(
    `${pdsUrl}/xrpc/com.atproto.server.createAccount`,
    {
      data: {
        handle,
        email: `${handle}@example.com`,
        password: "moderator-password",
      },
    },
  );
  expect(account.ok()).toBe(true);
  const moderatorDid = (await account.json()).did;
  let createdUri: string | undefined;
  try {
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard`);
    const widget = page.getByTestId("creator-moderators");
    await expect(widget).toBeVisible();
    await widget.getByRole("button", { name: "Add" }).click();
    const add = page.getByRole("dialog", { name: "Add Moderator" });
    await add
      .getByPlaceholder("did:plc:... or @handle.bsky.social")
      .fill(moderatorDid);
    const hideMessages = add.getByRole("checkbox", { name: "Hide Messages" });
    await hideMessages.focus();
    await page.keyboard.press("Space");
    await expect(hideMessages).toBeChecked();
    const created = page.waitForResponse(
      (response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.createRecord") &&
        response.request().postData()?.includes(moderatorDid) === true,
    );
    await add
      .getByRole("button", { name: "Add Moderator", exact: true })
      .click();
    const createdResponse = await created;
    expect(createdResponse.ok()).toBe(true);
    createdUri = (await createdResponse.json()).uri;
    await expect(add).toBeHidden();
    const row = widget.getByTestId(`moderator-${moderatorDid}`);
    await expect(row.getByText("hide", { exact: true })).toBeVisible();
    await row.getByRole("button", { name: "Remove moderator" }).click();
    const remove = page.getByRole("dialog", { name: "Remove Moderator?" });
    await remove.getByRole("button", { name: "Cancel" }).click();
    await expect(row).toBeVisible();
    await row.getByRole("button", { name: "Remove moderator" }).click();
    await remove.getByRole("button", { name: "Remove", exact: true }).click();
    await expect(row).toBeHidden();
    const lookup = await pds.get(`${pdsUrl}/xrpc/com.atproto.repo.getRecord`, {
      params: {
        repo: process.env.ACCOUNT_DID!,
        collection: "place.stream.moderation.permission",
        rkey: createdUri!.split("/").pop()!,
      },
      headers: { Authorization: `Bearer ${ownerToken}` },
    });
    expect(lookup.status()).toBe(400);
    createdUri = undefined;
  } finally {
    if (createdUri) {
      const removed = await pds.post(
        `${pdsUrl}/xrpc/com.atproto.repo.deleteRecord`,
        {
          headers: { Authorization: `Bearer ${ownerToken}` },
          data: {
            repo: process.env.ACCOUNT_DID,
            collection: "place.stream.moderation.permission",
            rkey: createdUri.split("/").pop(),
          },
        },
      );
      expect(removed.ok()).toBe(true);
    }
    await pds.dispose();
  }
});
