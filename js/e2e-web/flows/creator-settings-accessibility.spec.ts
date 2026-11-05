import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("settings inputs have their visible labels as accessible names", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard/stream`);
  const death = page.getByRole("checkbox", { name: "Death" });
  await expect(death).toHaveAccessibleDescription(
    /discussion or display of death/i,
  );
  await expect(page.getByRole("button", { name: "About Death" })).toHaveCount(
    0,
  );
  for (const name of [
    "Copyright Year",
    "Copyright Notice",
    "Credit Line",
    "Allowed Broadcasters",
    "Delete After",
    "Verifier DID or handle",
  ]) {
    await expect(
      page.getByRole("textbox", { name, exact: true }),
    ).toBeVisible();
  }
  const license = page.getByRole("button", { name: /License/ });
  await expect(license).toBeVisible();
  await license.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("menu")).toBeVisible();
  await page.keyboard.press("Escape");
  await page.getByText("Labelled by", { exact: true }).click();
  await expect(
    page.getByRole("textbox", { name: "Labeler DID or handle" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Label (trailing * matches a prefix)" }),
  ).toBeVisible();
});

test("keyboard users can read and dismiss the AI explanation", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard/stream`);
  const choice = page.getByRole("checkbox", {
    name: "Mark your content as available for generative AI training",
  });
  const description =
    "Your video's metadata signals whether you want your content used to train generative AI.";
  await expect(choice).toBeEnabled();
  await page.mouse.move(0, 0);
  await choice.focus();
  await expect(choice).toHaveAccessibleDescription(description);
  await expect(page.getByText(description, { exact: true })).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByText(description, { exact: true })).not.toBeVisible();
  await expect(choice).toBeFocused();
});

test.describe("touch explanations", () => {
  test.use({ hasTouch: true, viewport: { width: 390, height: 844 } });
  test("content warning explanations do not change a choice", async ({
    page,
  }) => {
    await loginThroughPds(page);
    await page.goto(`${url}/dashboard/stream`);
    const warning = page.getByRole("checkbox", { name: "Death" });
    const checked = await warning.isChecked();
    const help = page.getByRole("button", { name: "About Death" });
    await help.tap();
    const dialog = page.getByRole("dialog", { name: "About Death" });
    const anchor = (await help.boundingBox())!;
    const panel = (await dialog.boundingBox())!;
    expect(
      Math.min(
        Math.abs(panel.y - (anchor.y + anchor.height)),
        Math.abs(anchor.y - (panel.y + panel.height)),
      ),
    ).toBeLessThanOrEqual(16);
    expect(panel.x).toBeGreaterThanOrEqual(0);
    expect(panel.x + panel.width).toBeLessThanOrEqual(390);
    expect(panel.height).toBeLessThan(180);
    await expect(dialog).not.toHaveAttribute("aria-modal", "true");
    await expect(
      dialog.getByText(/discussion or display of death/i),
    ).toBeVisible();
    const close = dialog.getByRole("button", { name: "Close" });
    await expect(close).toBeVisible();
    const heading = dialog.getByText("Death", { exact: true });
    expect(
      Math.abs(
        (await close.boundingBox())!.y - (await heading.boundingBox())!.y,
      ),
    ).toBeLessThan(30);
    await close.tap();
    expect(await warning.isChecked()).toBe(checked);
    await help.focus();
    await page.keyboard.press("Enter");
    await expect(dialog).toBeVisible();
    await expect(help).toBeFocused();
    await expect(help).toHaveAccessibleDescription(
      /discussion or display of death/i,
    );
    await page.keyboard.press("Tab");
    await expect(dialog).not.toBeVisible();
    await help.focus();
    await page.keyboard.press("Enter");
    await expect(dialog).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(dialog).not.toBeVisible();
    await expect(help).toBeFocused();
    await help.tap();
    await expect(dialog).toBeVisible();
    await help.tap();
    await expect(dialog).not.toBeVisible();
    await help.tap();
    const language = page.getByRole("checkbox", { name: "Language" });
    const languageChecked = await language.isChecked();
    await language.tap();
    await expect(dialog).not.toBeVisible();
    expect(await language.isChecked()).toBe(!languageChecked);
    await language.tap();

    const lowerHelp = page.getByRole("button", { name: "About Violence" });
    await lowerHelp.evaluate((element) =>
      element.scrollIntoView({ block: "end" }),
    );
    await lowerHelp.tap();
    const lowerDialog = page.getByRole("dialog", { name: "About Violence" });
    const lowerPanel = (await lowerDialog.boundingBox())!;
    expect(lowerPanel.y).toBeGreaterThanOrEqual(0);
    expect(lowerPanel.y + lowerPanel.height).toBeLessThanOrEqual(844);
    await lowerDialog.getByRole("button", { name: "Close" }).tap();

    await page
      .getByRole("button", {
        name: "About Personally Identifiable Information",
      })
      .tap();
    const longDialog = page.getByRole("dialog", {
      name: "About Personally Identifiable Information",
    });
    const longPanel = (await longDialog.boundingBox())!;
    const longClose = (await longDialog
      .getByRole("button", { name: "Close" })
      .boundingBox())!;
    expect(longClose.x + longClose.width).toBeLessThanOrEqual(
      longPanel.x + longPanel.width,
    );
    expect(longPanel.x + longPanel.width).toBeLessThanOrEqual(390);
    await longDialog.getByRole("button", { name: "Close" }).tap();
    const ai = page.getByRole("checkbox", {
      name: "Mark your content as available for generative AI training",
    });
    const aiChecked = await ai.isChecked();
    await page.getByRole("button", { name: "About distribution" }).tap();
    const distributionHelp = page.getByRole("dialog", {
      name: "About distribution",
    });
    await expect(
      distributionHelp.getByText(
        "Your video's metadata signals whether you want your content used to train generative AI.",
      ),
    ).toBeVisible();
    await distributionHelp.getByRole("button", { name: "Close" }).tap();
    expect(await ai.isChecked()).toBe(aiChecked);
    await page.goto(`${url}/dashboard`);
    const widget = page.getByRole("region", { name: "Stream Info widget" });
    await widget.getByRole("button", { name: "Stream options" }).tap();
    await expect(
      widget.getByRole("textbox", { name: "Canonical URL" }),
    ).toBeVisible();
    await widget.getByRole("button", { name: "About stream options" }).tap();
    const optionsHelp = page.getByRole("dialog", {
      name: "About stream options",
    });
    await expect(
      optionsHelp.getByText("End livestream automatically", { exact: true }),
    ).toBeVisible();
    await optionsHelp.getByRole("button", { name: "Close" }).tap();
  });
});
