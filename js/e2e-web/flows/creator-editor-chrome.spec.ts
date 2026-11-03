import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("the dashboard identifies its controls and widget panels", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await expect(
    page.getByRole("toolbar", { name: "Creator dashboard controls" }),
  ).toBeVisible();
  for (const label of [
    "Stream Monitor",
    "Stream Health",
    "Chat",
    "Stream Info",
    "Multistream",
    "Moderators",
  ]) {
    const panel = page.getByRole("region", { name: `${label} widget` });
    await expect(panel).toBeVisible();
    await expect(panel.getByText(label, { exact: true }).first()).toBeVisible();
    if (["Stream Health", "Chat", "Moderators"].includes(label)) {
      await expect(panel.getByText(label, { exact: true })).toHaveCount(1);
    }
  }
  const title = page.getByPlaceholder("Enter your stream title...");
  await title.fill("A title in progress");
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await expect(title).toHaveValue("A title in progress");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(title).toHaveValue("A title in progress");
});
