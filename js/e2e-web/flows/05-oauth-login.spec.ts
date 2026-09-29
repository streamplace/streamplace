import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// Log in the way a user does, then act as that user through chat. See
// flows/login.ts for the OAuth machinery; it needs the harness's HTTPS mode.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("05-oauth-login: log in through the PDS, then chat", async ({ page }) => {
  await loginThroughPds(page);

  // Now act as the user. A chat message is a record the node writes to the
  // user's PDS through oatproxy's upstream (DPoP-bound) session. Loading the
  // app afresh also restores the session from browser storage.
  await page.goto(`${HTTPS_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  // chat needs the streamer's profile, which arrives over the stream's
  // websocket along with this system message
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  const chatInput = page.getByPlaceholder("Type a message...");
  const message = `hello from 05-oauth-login ${Date.now()}`;
  const messageCreateResponse = page.waitForResponse((response) => {
    const request = response.request();
    return (
      request.method() === "POST" &&
      new URL(response.url()).pathname ===
        "/xrpc/com.atproto.repo.createRecord" &&
      request.postData()?.includes(message) === true
    );
  });
  // a send the page wasn't ready for clears the box and is dropped; resend
  await expect(async () => {
    await chatInput.fill(message);
    await chatInput.press("Enter");
    await expect(page.getByText(message).first()).toBeVisible({
      timeout: 10_000,
    });
  }).toPass({ timeout: 45_000 });
  const response = await messageCreateResponse;
  expect(
    response.ok(),
    `chat record creation returned ${response.status()}`,
  ).toBe(true);

  // The box shows sends optimistically, so reload: only the node's chat
  // history can put the message back, and it only has what the firehose
  // brought back from the PDS.
  await page.reload();
  await expect(page.getByText(message).first()).toBeVisible({
    timeout: 30_000,
  });
});
