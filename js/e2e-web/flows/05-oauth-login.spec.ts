import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";

// Log in the way a user does: the app's atproto OAuth client talks to the
// node's OAuth proxy (oatproxy), which sends the browser to the PDS's own
// sign-in and consent pages and trades the result for a session. Every hop is
// real https — the harness serves the node and PDS on hostnames that resolve to
// 127.0.0.1, with a throwaway CA (see pkg/cmd/e2e_https.go) — so this runs the
// production OAuth code, not the http://127.0.0.1 development shortcut.
//
// Needs the harness's HTTPS mode (hack/e2e-web-local.sh turns it on).
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const PDS_URL = process.env.PDS_HTTPS_URL;
const HANDLE = process.env.ACCOUNT_HANDLE;
const PASSWORD = process.env.ACCOUNT_PASSWORD;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("05-oauth-login: log in through the PDS, then chat", async ({ page }) => {
  const appOrigin = new URL(HTTPS_URL!).origin;
  const pdsOrigin = new URL(PDS_URL!).origin;

  // This origin is new to the browser, so point the app at the node again.
  await pointAppAtNode(page, HTTPS_URL!);

  await page.goto(`${HTTPS_URL}/login`);
  const handleField = page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 30_000 });
  await handleField.fill(HANDLE!);
  await page.getByTestId("login-submit").click();

  // oatproxy hands the browser to the PDS's authorization UI; the handle comes
  // along as login_hint, so only the password is asked for
  await page.waitForURL((u) => u.origin === pdsOrigin, { timeout: 60_000 });
  const password = page.locator('input[type="password"]');
  await expect(password).toBeVisible({ timeout: 30_000 });
  await password.fill(PASSWORD!);
  await page.getByRole("button", { name: /^(sign in|next)$/i }).click();

  await page.getByRole("button", { name: /^authorize$/i }).click();

  // back through the node's /oauth/return to the app's /login, which lands a
  // logged-in user on their account settings
  await page.waitForURL((u) => u.origin === appOrigin, { timeout: 60_000 });
  await expect(page.getByText(`@${HANDLE}`).first()).toBeVisible({
    timeout: 30_000,
  });

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
