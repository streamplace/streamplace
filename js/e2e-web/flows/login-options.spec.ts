import { expect, test, type Page, type Response } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
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
  const reset = page.getByTestId("branding-reset-sign-in");
  try {
    await page.getByTestId("branding-input-loginEntryway").fill(PDS_URL!);
    await page
      .getByTestId("branding-input-walletConnectProjectId")
      .fill("e2e-project");
    await brandingWrites(page, 2, () =>
      page.getByTestId("branding-update-sign-in").click(),
    );
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
    await expect(visitor.getByTestId("login-walletconnect")).toBeVisible();
    await visitor.getByTestId("login-wallet-test.e2e").click();
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
  } finally {
    await visitor.close();
    await page.goto(`${HTTPS_URL}/settings/branding`);
    for (const m of METHODS) {
      const box = page.getByTestId(`branding-login-method-${m}`);
      if ((await box.getAttribute("aria-checked")) === "true") {
        await brandingWrites(page, 1, () => box.click());
        await expect(box).toHaveAttribute("aria-checked", "false");
      }
    }
    if (await reset.isEnabled()) await reset.click();
    await expect(reset).toBeDisabled();
  }
});
