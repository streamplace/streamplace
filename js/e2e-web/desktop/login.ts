import { chromium, expect } from "@playwright/test";
import { completePdsSignIn } from "../flows/login";
import type { Desktop } from "./fixture";

declare global {
  // Written by the main-process stub installed below.
  var __streamplaceOpenExternalURLs: string[] | undefined;
}

// Desktop sends OAuth to the system browser and gets the result back on a
// custom scheme. This drives both halves: it intercepts the URL the app asks
// the OS to open, signs in there in a real Chromium, then emits the deep link
// the OS would hand back to Electron.
export async function loginThroughSystemBrowser(
  desktop: Desktop,
): Promise<void> {
  const app = desktop.electronApp();
  const page = desktop.page;
  const handle = process.env.ACCOUNT_HANDLE!;
  const password = process.env.ACCOUNT_PASSWORD!;
  const nodeOrigin = new URL(desktop.nodeURL).origin;

  await app.evaluate(({ shell }) => {
    const opened: string[] = [];
    globalThis.__streamplaceOpenExternalURLs = opened;
    shell.openExternal = async (url: string) => {
      opened.push(url);
    };
  });

  const handleField = page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();
  await expect(handleField).toBeVisible({ timeout: 60_000 });
  await handleField.fill(handle);
  await page.getByTestId("login-submit").click();

  await expect
    .poll(
      async () =>
        app.evaluate(
          () => globalThis.__streamplaceOpenExternalURLs?.length ?? 0,
        ),
      { timeout: 60_000 },
    )
    .toBe(1);
  const authorizeURL = await app.evaluate(() => {
    const url = globalThis.__streamplaceOpenExternalURLs?.[0];
    if (!url) {
      throw new Error("the app did not open a login URL");
    }
    return url;
  });
  // The login left this window; only the system browser navigates from here.
  expect(new URL(authorizeURL).origin).toBe(nodeOrigin);
  await expect(page).toHaveURL(`${desktop.nodeURL}/desktop`);

  const browser = await chromium.launch({
    headless: true,
    args: [
      "--disable-dev-shm-usage",
      ...(process.getuid?.() === 0 ? ["--no-sandbox"] : []),
      ...(process.env.E2E_TLS_SPKI
        ? [`--ignore-certificate-errors-spki-list=${process.env.E2E_TLS_SPKI}`]
        : []),
    ],
    ...(process.env.E2E_PROXY_URL && {
      proxy: {
        server: process.env.E2E_PROXY_URL,
        bypass: "127.0.0.1,localhost",
      },
    }),
  });
  try {
    const browserPage = await browser.newPage();
    await browserPage.goto(authorizeURL);
    await completePdsSignIn(browserPage, nodeOrigin, password);
    await browserPage.waitForURL(
      (u) =>
        u.origin === nodeOrigin &&
        u.pathname.startsWith("/app-return/desktop/"),
      { timeout: 60_000 },
    );
    // The node holds the response for the app to poll; no OS handler needed.
    await expect(browserPage.getByText("You're logged in")).toBeVisible();
  } finally {
    await browser.close();
  }

  // The renderer polls the node and completes the login.
  await expect(page.getByText("Streamplace Desktop").first()).toBeVisible({
    timeout: 60_000,
  });
}
