import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

const labels = [
  "Stream Monitor",
  "Stream Health",
  "Chat",
  "Stream Info",
  "Multistream",
  "Moderators",
];

test("widget titles align their icons in normal and customization modes", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  for (const editing of [false, true]) {
    if (editing)
      await page.getByRole("button", { name: "Customize dashboard" }).click();
    for (const label of labels) {
      const header = page
        .getByRole("region", { name: `${label} widget` })
        .locator(":scope > div")
        .first();
      const icon = header.locator(":scope > span svg");
      const title = editing
        ? header.getByRole("combobox")
        : header.locator("strong");
      await expect(title).toBeVisible();
      const iconBox = (await icon.boundingBox())!;
      const titleBox = (await title.boundingBox())!;
      expect(
        Math.abs(
          iconBox.y + iconBox.height / 2 - titleBox.y - titleBox.height / 2,
        ),
        label,
      ).toBeLessThanOrEqual(1);
    }
  }
});

test("widget panels join their headers without a second rounded top", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  for (const label of labels) {
    const widget = page.getByRole("region", { name: `${label} widget` });
    await expect(widget).toBeVisible();
    const body = widget.locator(":scope > div").nth(1);
    const panel =
      label === "Moderators"
        ? body.getByTestId("creator-moderators").locator(":scope > div").first()
        : body.locator(":scope > *").first();
    await expect(panel).toHaveCSS("border-top-left-radius", "0px");
    await expect(panel).toHaveCSS("border-top-right-radius", "0px");
    await expect(panel).not.toHaveCSS("border-bottom-left-radius", "0px");
    await expect(panel).not.toHaveCSS("border-bottom-right-radius", "0px");
  }
});
