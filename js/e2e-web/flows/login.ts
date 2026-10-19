import { expect, type Page } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";

// Real OAuth login: baseline uses the HTTPS node; upstream mode starts at the
// HTTP loopback node (or an explicit Desktop node URL), continues through the
// HTTPS station, and reaches the PDS's genuine sign-in and consent pages.
// The throwaway CA is scoped to this harness. No fake tokens or Public-mode
// production security relaxations are used for the chained loopback flow.
// Callers require HTTPS PDS metadata and gate their chosen topology themselves.
export async function loginThroughPds(
  page: Page,
  nodeURL?: string,
): Promise<void> {
  const httpsUrl =
    nodeURL ??
    (process.env.E2E_UPSTREAM_STATION
      ? process.env.SERVER_URL!
      : process.env.SERVER_HTTPS_URL!);
  const pdsUrl = process.env.PDS_HTTPS_URL!;
  const handle = process.env.ACCOUNT_HANDLE!;
  const password = process.env.ACCOUNT_PASSWORD!;
  const appOrigin = new URL(httpsUrl).origin;
  const pdsOrigin = new URL(pdsUrl).origin;

  // Idempotent on an already configured origin, including Desktop localhost.
  await pointAppAtNode(page, httpsUrl);

  await page.goto(`${httpsUrl}/login`);
  const handleField = page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 30_000 });
  await handleField.fill(handle);
  const stationAuthorization = process.env.E2E_UPSTREAM_STATION
    ? page.waitForRequest(
        (request) =>
          new URL(request.url()).origin ===
            new URL(process.env.UPSTREAM_STATION_URL!).origin &&
          new URL(request.url()).pathname === "/oauth/authorize",
        { timeout: 60_000 },
      )
    : undefined;
  // Handle both waits together so an early login failure cannot leave a
  // dangling request promise that hides the original error during teardown.
  await Promise.all([
    stationAuthorization,
    (async () => {
      await page.getByTestId("login-submit").click();
      // The handle travels as login_hint; only the password is asked for.
      await page.waitForURL((u) => u.origin === pdsOrigin, { timeout: 60_000 });
    })(),
  ]);
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
  await expect(page.getByText(`@${handle}`).first()).toBeVisible({
    timeout: 30_000,
  });
}
