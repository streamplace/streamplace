import { expect, type Page } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";

// Log in the way a user does: the app's atproto OAuth client talks to the
// node's OAuth proxy (oatproxy), which sends the browser to the PDS's own
// sign-in and consent pages and trades the result for a session. Every hop is
// real https — the harness serves the node and PDS on hostnames that resolve to
// 127.0.0.1, with a throwaway CA (see pkg/cmd/e2e_https.go) — so this runs the
// production OAuth code, not the http://127.0.0.1 development shortcut.
//
// Needs the harness's HTTPS mode (hack/e2e-web-local.sh turns it on); callers
// gate on SERVER_HTTPS_URL themselves and skip when it is unset.
export async function loginThroughPds(page: Page): Promise<void> {
  const httpsUrl = process.env.SERVER_HTTPS_URL!;
  const pdsUrl = process.env.PDS_HTTPS_URL!;
  const handle = process.env.ACCOUNT_HANDLE!;
  const password = process.env.ACCOUNT_PASSWORD!;
  const appOrigin = new URL(httpsUrl).origin;
  const pdsOrigin = new URL(pdsUrl).origin;

  // This origin is new to the browser, so point the app at the node again.
  await pointAppAtNode(page, httpsUrl);

  await page.goto(`${httpsUrl}/login`);
  const handleField = page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 30_000 });
  await handleField.fill(handle);
  await page.getByTestId("login-submit").click();

  // oatproxy hands the browser to the PDS's authorization UI; the handle comes
  // along as login_hint, so only the password is asked for
  await page.waitForURL((u) => u.origin === pdsOrigin, { timeout: 60_000 });
  const passwordField = page.locator('input[type="password"]');
  await expect(passwordField).toBeVisible({ timeout: 30_000 });
  await passwordField.fill(password);
  await page.getByRole("button", { name: /^(sign in|next)$/i }).click();

  // The PDS can skip consent on a retry after this OAuth client is already
  // authorized, redirecting straight back to the app.
  await Promise.race([
    page.getByRole("button", { name: /^authorize$/i }).click({
      timeout: 60_000,
    }),
    page.waitForURL((u) => u.origin === appOrigin, { timeout: 60_000 }),
  ]);

  // back through the node's /oauth/return to the app's /login, which lands a
  // logged-in user on their account settings
  await page.waitForURL((u) => u.origin === appOrigin, { timeout: 60_000 });
  await expect(
    page
      .getByText(`@${handle}`)
      .or(page.getByRole("link", { name: `Signed in as @${handle}` }))
      .first(),
  ).toBeVisible({ timeout: 30_000 });
}
