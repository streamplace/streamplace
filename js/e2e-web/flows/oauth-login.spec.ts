import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// Login counterpart to .maestro/setup/oauth-login.yaml, with chat/profile
// coverage corresponding to .maestro/logged-in/chat-profile.yaml.
// Log in the way a user does, then act as that user through chat. See
// flows/login.ts for the OAuth machinery; it needs the harness's HTTPS mode.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const HANDLE = process.env.ACCOUNT_HANDLE;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("oauth-login: log in, chat, and reopen a profile", async ({ page }) => {
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
  const message = `hello from oauth-login ${Date.now()}`;
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

  // Exercise the real card rendered outside the chat row. Checking its own
  // handle avoids accidentally matching the account settings or chat username.
  const profileTrigger = page
    .getByTestId(`chat-profile-trigger-${HANDLE}`)
    .first();
  const profileCard = page.getByTestId("chat-profile-card");
  for (let opening = 0; opening < 2; opening++) {
    await profileTrigger.click();
    await expect(profileCard).toBeVisible();
    await expect(profileCard.getByTestId("chat-profile-handle")).toHaveText(
      `@${HANDLE}`,
    );
    await page
      .getByTestId("chat-profile-backdrop")
      .click({ position: { x: 10, y: 10 } });
    await expect(profileCard).toBeHidden();
  }

  // Do not reload or retry the send here: closing the profile must leave the
  // existing chat session usable. Reload only afterward to prove persistence.
  const afterProfileMessage = `chat after profile ${Date.now()}`;
  await chatInput.fill(afterProfileMessage);
  await chatInput.press("Enter");
  await expect(page.getByText(afterProfileMessage).first()).toBeVisible({
    timeout: 15_000,
  });
  await page.reload();
  await expect(page.getByText(afterProfileMessage).first()).toBeVisible({
    timeout: 30_000,
  });
});
