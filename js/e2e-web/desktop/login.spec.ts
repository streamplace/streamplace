import { expect, test } from "./fixture";
import { loginThroughSystemBrowser } from "./login";

// Electron-specific runner; shared mobile/web OAuth coverage remains unchanged.
test("log in through the dummy PDS", async ({ desktop }, testInfo) => {
  const { page, nodeURL } = desktop;
  await loginThroughSystemBrowser(desktop);
  await page.goto(`${nodeURL}/settings/account`);
  await expect(
    page.getByText(`@${process.env.ACCOUNT_HANDLE}`, { exact: true }).first(),
  ).toBeVisible();
  await expect(page.getByText("Log out", { exact: true }).last()).toBeVisible();
  await page.reload();
  await expect(
    page.getByText(`@${process.env.ACCOUNT_HANDLE}`, { exact: true }).first(),
  ).toBeVisible();
  await expect(page.getByText("Log out", { exact: true }).last()).toBeVisible();
  await page.screenshot({ path: testInfo.outputPath("logged-in.png") });
});
