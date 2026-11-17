import { expect, test, type Page, type Response } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { sendChatMessage } from "./chat";
import { loginThroughPds } from "./login";

// Sign-in options the node turns on in its branding (loginEntryway,
// loginMethods, walletConnectProjectId), with the harness PDS as the
// entryway. Mobile counterpart: .maestro/logged-out/login-options.yaml.
// The provider and email paths go through the node's OAuth proxy, which
// passes provider= and screen= on to the entryway PDS.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;
const PDS_URL = process.env.PDS_HTTPS_URL;
const METHODS = ["apple", "google", "ethereum", "qr"];

// Branding edits need an admin session, so the OAuth/HTTPS harness.
test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

const handleField = (page: Page) =>
  page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();

async function openLogin(page: Page) {
  await page.goto(`${HTTPS_URL}/login`);
  await expect(handleField(page)).toBeVisible({ timeout: 30_000 });
}

// The node's own /oauth/authorize request a sign-in starts with, once the
// browser has gone on to the entryway.
async function authorizeParams(page: Page, start: () => Promise<void>) {
  const appOrigin = new URL(HTTPS_URL!).origin;
  const authorize = page.waitForRequest((r) => {
    const u = new URL(r.url());
    return u.origin === appOrigin && u.pathname === "/oauth/authorize";
  });
  await start();
  const params = new URL((await authorize).url()).searchParams;
  await page.waitForURL((u) => u.origin === new URL(PDS_URL!).origin, {
    timeout: 60_000,
  });
  return params;
}

// Runs action and waits for the n branding writes it makes to succeed.
async function brandingWrites(
  page: Page,
  n: number,
  action: () => Promise<void>,
) {
  let seen = 0;
  const done = new Promise<void>((resolve, reject) => {
    const onResponse = (r: Response) => {
      if (!/\/xrpc\/place\.stream\.branding\.\w+Blob$/.test(r.url())) return;
      if (!r.ok()) reject(new Error(`${r.url()}: HTTP ${r.status()}`));
      if (++seen < n) return;
      page.off("response", onResponse);
      resolve();
    };
    page.on("response", onResponse);
  });
  await action();
  await done;
}

// Turns every login method off and clears the sign-in keys again, from the
// branding admin page.
async function resetLoginOptions(page: Page) {
  await page.goto(`${HTTPS_URL}/settings/branding`);
  for (const m of METHODS) {
    const box = page.getByTestId(`branding-login-method-${m}`);
    if ((await box.getAttribute("aria-checked")) === "true") {
      await brandingWrites(page, 1, () => box.click());
      await expect(box).toHaveAttribute("aria-checked", "false");
    }
  }
  const reset = page.getByTestId("branding-reset-sign-in");
  if (await reset.isEnabled()) await reset.click();
  await expect(reset).toBeDisabled();
}

// A browser wallet that announces itself over EIP-6963, as MetaMask does,
// and records what it's asked to sign. Runs in the page.
const WALLET_ADDRESS = "0x8ba1f109551bD432803012645Ac136ddd64DBA72";
function fakeWallet(address: string) {
  const provider = {
    request: async ({ method, params }: { method: string; params?: any }) => {
      if (method === "eth_requestAccounts") return [address];
      if (method === "eth_chainId") return "0x1";
      if (method === "personal_sign") {
        (window as any).e2eSigned = params;
        return `0x${"11".repeat(65)}`;
      }
      throw new Error(`unexpected ${method}`);
    },
  };
  const info = { uuid: "e2e", name: "E2E Wallet", icon: "", rdns: "test.e2e" };
  const announce = () =>
    window.dispatchEvent(
      new CustomEvent("eip6963:announceProvider", {
        detail: Object.freeze({ info, provider }),
      }),
    );
  window.addEventListener("eip6963:requestProvider", announce);
  announce();
}

test("login-options: branding gates the sign-in options", async ({
  page,
  browser,
}) => {
  test.setTimeout(4 * 60_000);
  const visitor = await browser.newPage();
  await visitor.addInitScript(fakeWallet, WALLET_ADDRESS);
  await pointAppAtNode(visitor, HTTPS_URL!);

  // Unset, the form is the handle field alone.
  await openLogin(visitor);
  await expect(visitor.getByText("Handle", { exact: true })).toBeVisible();
  for (const m of METHODS) {
    await expect(visitor.getByTestId(`login-${m}`)).toHaveCount(0);
  }

  await loginThroughPds(page);
  await page.goto(`${HTTPS_URL}/settings/branding`);
  try {
    // No WalletConnect project id: with one, the Ethereum panel pairs with
    // WalletConnect's relay as it opens, which needs a real id.
    const entryway = page.getByTestId("branding-input-loginEntryway");
    await entryway.fill(PDS_URL!);
    await brandingWrites(page, 1, () =>
      page.getByTestId("branding-update-sign-in").click(),
    );
    // what's saved stays in the field
    await expect(entryway).toHaveValue(PDS_URL!);
    // One after another as each write lands, the way a person ticks a list:
    // every toggle has to start from the ones before it.
    for (const m of METHODS) {
      await brandingWrites(page, 1, () =>
        page.getByTestId(`branding-login-method-${m}`).click(),
      );
    }
    for (const m of METHODS) {
      await expect(
        page.getByTestId(`branding-login-method-${m}`),
      ).toHaveAttribute("aria-checked", "true");
    }

    await openLogin(visitor);
    await expect(visitor.getByText("Handle or email")).toBeVisible();
    for (const m of METHODS) {
      await expect(visitor.getByTestId(`login-${m}`)).toBeVisible();
    }
    const screenshot = test.info().outputPath("login-options.png");
    await visitor.screenshot({ path: screenshot });
    await test.info().attach("login-options", { path: screenshot });

    // Sign in with Ethereum through the browser wallet gets as far as a SIWE
    // signature for this site (the backend that takes it is still to come),
    // without asking any wallet vendor's servers for anything.
    const vendorRequests: string[] = [];
    visitor.on("request", (r) => {
      if (/walletconnect|reown|coinbase/.test(new URL(r.url()).hostname)) {
        vendorRequests.push(r.url());
      }
    });
    await visitor.getByTestId("login-ethereum").click();
    await expect(visitor.getByTestId("login-walletconnect-qr")).toHaveCount(0);
    await visitor.getByText("Connect E2E Wallet").click();
    await expect(visitor.getByTestId("login-error")).toHaveText(
      "Sign in with Ethereum isn't available on this server yet.",
    );
    const [hexMessage, signer] = await visitor.evaluate(
      () => (window as any).e2eSigned,
    );
    expect(Buffer.from(hexMessage.slice(2), "hex").toString()).toContain(
      `${new URL(HTTPS_URL!).host} wants you to sign in with your Ethereum account:\n${WALLET_ADDRESS}`,
    );
    expect(signer).toBe(WALLET_ADDRESS);
    expect(vendorRequests).toEqual([]);
    await visitor.getByTestId("login-ethereum-back").click();

    const apple = await authorizeParams(visitor, () =>
      visitor.getByTestId("login-apple").click(),
    );
    expect(apple.get("provider")).toBe("apple");

    await openLogin(visitor);
    await handleField(visitor).fill("nobody@example.com");
    const email = await authorizeParams(visitor, () =>
      visitor.getByTestId("login-submit").click(),
    );
    expect(email.get("screen")).toBe("sign-in");
    expect(email.get("provider")).toBeNull();

    // clearing the field and updating removes the setting
    await entryway.fill("");
    await brandingWrites(page, 1, () =>
      page.getByTestId("branding-update-sign-in").click(),
    );
    await expect(page.getByTestId("branding-reset-sign-in")).toBeDisabled();
  } finally {
    await visitor.close();
    await resetLoginOptions(page);
  }
});

// QR login: the browser shows a code; a signed-in device opens what it links
// to and approves; the browser is then signed in, sharing that device's
// session with the PDS. The harness node has an app bundle id, which is what
// makes it offer QR login. A logged-out browser keeps an attempt ready, so
// the login form shows it at once. Mobile counterpart:
// .maestro/logged-in/qr-login.yaml.
test("login-options: QR login signs a browser in from a signed-in device", async ({
  page,
  browser,
}) => {
  test.setTimeout(4 * 60_000);
  await loginThroughPds(page);
  const visitor = await browser.newPage();
  await pointAppAtNode(visitor, HTTPS_URL!);
  try {
    await page.goto(`${HTTPS_URL}/settings/branding`);
    await page.getByTestId("branding-input-loginEntryway").fill(PDS_URL!);
    await brandingWrites(page, 1, () =>
      page.getByTestId("branding-update-sign-in").click(),
    );
    await brandingWrites(page, 1, () =>
      page.getByTestId("branding-login-method-qr").click(),
    );
    // a signed-in browser never prepares one, even as a page loads
    const signedInAttempts: string[] = [];
    page.on("request", (r) => {
      if (r.url().includes("/xrpc/place.stream.login.createQRLogin")) {
        signedInAttempts.push(r.url());
      }
    });

    // made in the background, before any login form is open
    const created = visitor.waitForResponse((r) =>
      r.url().includes("/xrpc/place.stream.login.createQRLogin"),
    );
    await visitor.goto(HTTPS_URL!);
    const { url } = await (await created).json();
    await expect(handleField(visitor)).toHaveCount(0);
    await visitor.getByRole("button", { name: "Log in" }).first().click();
    await expect(handleField(visitor)).toBeVisible();
    // the code is up as soon as the form is, not a spinner (count() doesn't
    // wait; the spinner draws no rect)
    expect(
      await visitor.getByTestId("login-qr").locator("svg rect").count(),
    ).toBe(1);

    await page.goto(url);
    await expect(page.getByText("Sign in on another device?")).toBeVisible({
      timeout: 30_000,
    });
    // the requesting browser, as the approver sees it
    await expect(page.getByText(/Chrome/).first()).toBeVisible();
    const screenshot = test.info().outputPath("qr-login-request.png");
    await page.screenshot({ path: screenshot });
    await test.info().attach("qr-login-request", { path: screenshot });
    await page.getByTestId("qr-login-approve").click();
    await expect(page.getByTestId("qr-login-result")).toHaveText(
      "Signed in on your browser",
    );
    await page.getByTestId("qr-login-done").click();
    await expect(page.getByTestId("home-stream-card").first()).toBeVisible();
    expect(signedInAttempts).toEqual([]);

    // the browser picks the approval up and signs in, and its session can
    // write to the PDS through the approver's upstream session
    await expect(handleField(visitor)).toHaveCount(0, { timeout: 30_000 });
    await sendChatMessage(
      visitor,
      HTTPS_URL!,
      `hello from qr-login ${Date.now()}`,
    );
  } finally {
    await visitor.close();
    await resetLoginOptions(page);
  }
});
