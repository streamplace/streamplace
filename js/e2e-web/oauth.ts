import { expect, type Page } from "@playwright/test";
import { pointAppAtNode } from "./server-setup";

// Log in the way a user does: the app's atproto OAuth client talks to the
// node's OAuth proxy (oatproxy), which sends the browser to the PDS's own
// sign-in and consent pages and trades the result for a session. Needs the
// harness's HTTPS mode (SERVER_HTTPS_URL); see 05-oauth-login.
export async function logInWithOAuth(page: Page) {
  const httpsURL = process.env.SERVER_HTTPS_URL!;
  const handle = process.env.ACCOUNT_HANDLE!;
  const appOrigin = new URL(httpsURL).origin;
  const pdsOrigin = new URL(process.env.PDS_HTTPS_URL!).origin;

  // This origin is new to the browser, so point the app at the node again.
  await pointAppAtNode(page, httpsURL);

  await page.goto(`${httpsURL}/login`);
  const handleField = page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 30_000 });
  await handleField.fill(handle);
  await page.getByTestId("login-submit").click();

  // oatproxy hands the browser to the PDS's authorization UI; the handle comes
  // along as login_hint, so only the password is asked for
  await page.waitForURL((u) => u.origin === pdsOrigin, { timeout: 60_000 });
  const password = page.locator('input[type="password"]');
  await expect(password).toBeVisible({ timeout: 30_000 });
  await password.fill(process.env.ACCOUNT_PASSWORD!);
  await page.getByRole("button", { name: /^(sign in|next)$/i }).click();

  // The consent screen shows once per account and client: a second login
  // (a later flow) goes straight back to the app.
  const authorize = page.getByRole("button", { name: /^authorize$/i });
  const backInApp = page.waitForURL((u) => u.origin === appOrigin, {
    timeout: 60_000,
  });
  const first = await Promise.race([
    authorize.waitFor({ timeout: 60_000 }).then(() => "consent" as const),
    backInApp.then(() => "app" as const),
  ]);
  if (first === "consent") await authorize.click();

  // back through the node's /oauth/return to the app's /login, which lands a
  // logged-in user on their account settings
  await backInApp;
  await expect(page.getByText(`@${handle}`).first()).toBeVisible({
    timeout: 30_000,
  });
}
