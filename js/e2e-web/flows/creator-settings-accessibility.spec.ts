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
    await page.getByRole("button", { name: "About Death" }).tap();
    const dialog = page.getByRole("dialog");
    await expect(
      dialog.getByText(/discussion or display of death/i),
    ).toBeVisible();
    const close = dialog.getByRole("button", { name: "Close" });
    await expect(close).toBeVisible();
    const heading = dialog.getByText("About Death", { exact: true });
    expect(
      Math.abs(
        (await close.boundingBox())!.y - (await heading.boundingBox())!.y,
      ),
    ).toBeLessThan(30);
    await close.tap();
    expect(await warning.isChecked()).toBe(checked);
    const ai = page.getByRole("checkbox", {
      name: "Mark your content as available for generative AI training",
    });
    const aiChecked = await ai.isChecked();
    await page.getByRole("button", { name: "About distribution" }).tap();
    await expect(
      dialog.getByText(
        "Your video's metadata signals whether you want your content used to train generative AI.",
      ),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Close" }).tap();
    expect(await ai.isChecked()).toBe(aiChecked);
    await page.goto(`${url}/dashboard`);
    const widget = page.getByRole("region", { name: "Stream Info widget" });
    await widget.getByRole("button", { name: "Stream options" }).tap();
    await expect(
      widget.getByRole("textbox", { name: "Canonical URL" }),
    ).toBeVisible();
    await widget.getByRole("button", { name: "About stream options" }).tap();
    await expect(
      dialog.getByText("End livestream automatically", { exact: true }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "Close" }).tap();
  });
});
