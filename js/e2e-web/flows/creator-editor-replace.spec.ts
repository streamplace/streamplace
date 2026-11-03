import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("swap a widget in place while keeping other drafts", async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("Title in progress");
  await chat.fill("Message in progress");
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const before = (await page
    .getByRole("region", { name: "Moderators widget" })
    .boundingBox())!;
  await page
    .getByRole("combobox", { name: "Change Moderators widget" })
    .selectOption({ label: "Problems" });
  const problems = page.getByRole("region", { name: "Problems widget" });
  await expect(problems).toBeVisible();
  const after = (await problems.boundingBox())!;
  expect(after).toEqual(before);
  await expect(title).toHaveValue("Title in progress");
  await expect(chat).toHaveValue("Message in progress");
  await expect(
    page.getByRole("combobox", { name: "Change Problems widget" }),
  ).toBeFocused();
  await page.reload();
  await expect(problems).toBeVisible();
  await expect(
    page.getByRole("region", { name: "Moderators widget" }),
  ).toHaveCount(0);
});
