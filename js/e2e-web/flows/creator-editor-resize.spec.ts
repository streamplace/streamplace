import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("keyboard resizing persists without losing drafts", async ({ page }) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("Title being edited");
  await chat.fill("Message being edited");
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const divider = page
    .getByRole("separator", { name: "Resize columns" })
    .first();
  await expect(divider).toBeVisible({ timeout: 5_000 });
  const monitor = page.getByRole("region", { name: "Stream Monitor widget" });
  const before = (await monitor.boundingBox())!.width;
  await divider.focus();
  await page.keyboard.press("ArrowRight");
  await expect
    .poll(async () => (await monitor.boundingBox())!.width)
    .toBeGreaterThan(before + 10);
  const after = (await monitor.boundingBox())!.width;
  await expect(title).toHaveValue("Title being edited");
  await expect(chat).toHaveValue("Message being edited");
  await page.reload();
  expect((await monitor.boundingBox())!.width).toBeCloseTo(after, 0);
});

test("dragging a divider resizes adjacent panels without crushing them", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const divider = page
    .getByRole("separator", { name: "Resize columns" })
    .first();
  await expect(divider).toBeVisible({ timeout: 5_000 });
  const monitor = page.getByRole("region", { name: "Stream Monitor widget" });
  const chat = page.getByRole("region", { name: "Chat widget" });
  const before = (await monitor.boundingBox())!.width;
  const box = (await divider.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2 + 100, box.y + box.height / 2, {
    steps: 10,
  });
  await page.mouse.up();
  await expect
    .poll(async () => (await monitor.boundingBox())!.width)
    .toBeGreaterThan(before + 30);
  await expect
    .poll(async () => (await chat.boundingBox())!.width)
    .toBeGreaterThanOrEqual(256);
  const after = (await monitor.boundingBox())!.width;
  await page.reload();
  expect((await monitor.boundingBox())!.width).toBeCloseTo(after, 0);
});
