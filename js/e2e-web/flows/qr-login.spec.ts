import { expect, test, type Page } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { sendChatMessage } from "./chat";
import { loginThroughPds } from "./login";

const HTTPS_URL = process.env.SERVER_HTTPS_URL;

// Branding edits need an admin session, so the OAuth/HTTPS harness.
test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

const handleField = (page: Page) =>
  page
    .locator('[data-testid="login-handle"], [data-testid="login-handle"] input')
    .first();

// QR login: the browser shows a code; a signed-in device opens what it links
// to and approves; the browser is then signed in, sharing that device's
// session with the PDS. The harness node has an app bundle id, which is what
// makes it offer QR login. A logged-out browser keeps an attempt ready, so
// the login form shows it at once. Mobile counterpart:
// .maestro/logged-in/qr-login.yaml.
test("QR login signs a browser in from a signed-in device", async ({
  page,
  browser,
}) => {
  test.setTimeout(4 * 60_000);
  await loginThroughPds(page);
  const visitor = await browser.newPage();
  await pointAppAtNode(visitor, HTTPS_URL!);
  await page.goto(`${HTTPS_URL}/settings/branding`);
  const toggle = page.getByTestId("branding-login-method-qr");
  try {
    await visitor.goto(`${HTTPS_URL}/login`);
    await expect(handleField(visitor)).toBeVisible();
    await expect(visitor.getByTestId("login-qr")).toHaveCount(0);
    await expect(toggle).toHaveAttribute("aria-checked", "false");
    await toggle.click();
    await expect(toggle).toHaveAttribute("aria-checked", "true");
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
    await page.goto(`${HTTPS_URL}/settings/branding`);
    if ((await toggle.getAttribute("aria-checked")) === "true") {
      await toggle.click();
      await expect(toggle).toHaveAttribute("aria-checked", "false");
    }
  }
});
