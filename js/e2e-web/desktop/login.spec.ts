import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// Electron-specific runner; shared mobile/web OAuth coverage remains unchanged.
test("log in through the dummy PDS", async ({ desktop }, testInfo) => {
  const { page, nodeURL } = desktop;
  await loginThroughSystemBrowser(desktop);
  const signedIn = page.getByText(
    `Signed in as @${process.env.ACCOUNT_HANDLE}`,
  );
  // Desktop is the only screen: other app routes land back on it.
  for (const path of ["/", "/settings/account"]) {
    await page.goto(nodeURL + path);
    await expect(page).toHaveURL(`${nodeURL}/desktop`);
    await expect(signedIn).toBeVisible();
  }
  await page.reload();
  await expect(signedIn).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("logged-in.png") });
  await page.getByTestId("desktop-logout").click();
  await expect(page.getByTestId("login-handle").first()).toBeVisible();
});
