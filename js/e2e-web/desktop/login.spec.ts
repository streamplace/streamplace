import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// Electron-specific runner; shared mobile/web OAuth coverage remains unchanged.
test("log in through the dummy PDS", async ({ desktop }, testInfo) => {
  const { page, nodeURL } = desktop;
  // The login survives Desktop reloading while the browser sign-in is open.
  await loginThroughSystemBrowser(desktop, { reloadMidLogin: true });
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
  // Offline, the preview points at OBS rather than the web go-live picker.
  await expect(page.getByTestId("desktop-offline")).toBeVisible();
  await expect(page.getByText("Camera", { exact: true })).toHaveCount(0);
  await page.screenshot({ path: testInfo.outputPath("logged-in.png") });
  // The 800x600 window scrolls down to the settings below the preview.
  const save = page.getByTestId("desktop-save");
  await expect(save).not.toBeInViewport();
  await page.mouse.move(400, 560);
  await page.mouse.wheel(0, 5000);
  await expect(save).toBeInViewport();
  await page.getByTestId("desktop-logout").click();
  await expect(page.getByTestId("login-handle").first()).toBeVisible();
});
