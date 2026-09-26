import { expect, test } from "@playwright/test";
import { logInWithOAuth } from "../oauth";

// Log in the way a user does: the app's atproto OAuth client talks to the
// node's OAuth proxy (oatproxy), which sends the browser to the PDS's own
// sign-in and consent pages and trades the result for a session. Every hop is
// real https — the harness serves the node and PDS on hostnames that resolve to
// 127.0.0.1, with a throwaway CA (see pkg/cmd/e2e_https.go) — so this runs the
// production OAuth code, not the http://127.0.0.1 development shortcut.
//
// Needs the harness's HTTPS mode (hack/e2e-web-local.sh turns it on).
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("05-oauth-login: log in through the PDS, then chat", async ({ page }) => {
  await logInWithOAuth(page);

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
  // a send the page wasn't ready for clears the box and is dropped; resend
  await expect(async () => {
    await chatInput.fill(message);
    await chatInput.press("Enter");
    await expect(page.getByText(message).first()).toBeVisible({
      timeout: 10_000,
    });
  }).toPass({ timeout: 45_000 });

  // The box shows sends optimistically, so reload: only the node's chat
  // history can put the message back, and it only has what the firehose
  // brought back from the PDS.
  await page.reload();
  await expect(page.getByText(message).first()).toBeVisible({
    timeout: 30_000,
  });
});
