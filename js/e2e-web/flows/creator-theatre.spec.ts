import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("enter and leave theatre without losing drafts", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("Title in progress");
  await chat.fill("Message in progress");
  const enter = page.getByRole("button", { name: "Theatre mode" });
  await expect(enter).toBeVisible({ timeout: 5_000 });
  await enter.click();
  await expect(
    page.getByRole("navigation", { name: "Creator navigation" }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("toolbar", { name: "Creator dashboard controls" }),
  ).toHaveCount(0);
  const exit = page.getByRole("button", { name: "Exit theatre mode" });
  await expect(exit).toBeFocused();
  await expect(title).toHaveValue("Title in progress");
  await expect(chat).toHaveValue("Message in progress");
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("navigation", { name: "Creator navigation" }),
  ).toBeVisible();
  await expect(enter).toBeFocused();
  await expect(title).toHaveValue("Title in progress");
  await expect(chat).toHaveValue("Message in progress");
  await enter.click();
  await exit.click();
  await expect(enter).toBeFocused();
});

test("theatre preference survives a reload", async ({ page }) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Theatre mode" }).click();
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Exit theatre mode" }),
  ).toBeVisible();
  await expect(
    page.getByRole("navigation", { name: "Creator navigation" }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Exit theatre mode" }).click();
  await page.reload();
  await expect(
    page.getByRole("button", { name: "Theatre mode" }),
  ).toBeVisible();
});

test("theatre hides phone navigation and can be exited", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await expect(
    page.getByRole("button", { name: "Open creator navigation" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Theatre mode" }).click();
  await expect(
    page.getByRole("button", { name: "Open creator navigation" }),
  ).toHaveCount(0);
  const exit = page.getByRole("button", { name: "Exit theatre mode" });
  expect((await exit.boundingBox())!.height).toBeLessThan(60);
  await exit.click();
  await expect(
    page.getByRole("button", { name: "Open creator navigation" }),
  ).toBeVisible();
});

test("a saved theatre preference can be exited on the login screen", async ({
  page,
}) => {
  await page.goto(`${url}/`);
  await page.evaluate(() =>
    localStorage.setItem("streamplace:theatre", "true"),
  );
  await page.goto(`${url}/dashboard`);
  const exit = page.getByRole("button", { name: "Exit theatre mode" });
  await expect(exit).toBeVisible();
  const closeLogin = page.getByRole("button", { name: "Close login" });
  await expect(closeLogin).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(closeLogin).toHaveCount(0);
  await expect(exit).toBeVisible();
  await exit.click();
  await expect(
    page.getByRole("navigation", { name: "Creator navigation" }),
  ).toBeVisible();
});
