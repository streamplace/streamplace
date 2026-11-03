import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("drag an available widget into a chosen place and keep drafts", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("Title in progress");
  await chat.fill("Message in progress");
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Add widget" }).click();
  const source = page.getByRole("button", { name: "Drag Problems to add" });
  const target = page.getByRole("region", { name: "Stream Monitor widget" });
  const from = (await source.boundingBox())!;
  const to = (await target.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height * 0.15, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await page.mouse.up();
  const problems = page.getByRole("region", { name: "Problems widget" });
  await expect(problems).toBeVisible();
  expect((await problems.boundingBox())!.y).toBeLessThan(
    (await target.boundingBox())!.y,
  );
  await expect(title).toHaveValue("Title in progress");
  await expect(chat).toHaveValue("Message in progress");
  await page.reload();
  expect((await problems.boundingBox())!.y).toBeLessThan(
    (await target.boundingBox())!.y,
  );
});

test("do not add a widget where it would make its neighbor too small", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Add widget" }).click();
  const source = page.getByRole("button", { name: "Drag Problems to add" });
  const target = page.getByRole("region", { name: "Chat widget" });
  const from = (await source.boundingBox())!;
  const to = (await target.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width * 0.15, to.y + to.height / 2, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await page.mouse.up();
  await expect(page.getByRole("alert")).toHaveText(
    "There is not enough room to place these widgets there.",
  );
  await expect(
    page.getByRole("region", { name: "Problems widget" }),
  ).toHaveCount(0);
});

test("selecting a widget does not squeeze the last panel below its minimum", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 700 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Add widget" }).click();
  await page.getByRole("button", { name: "Add Problems" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "There is not enough room to place these widgets there.",
  );
  await expect(
    page.getByRole("region", { name: "Problems widget" }),
  ).toHaveCount(0);
});

test("selecting a widget uses another panel when the last one is too small", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 900 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Add widget" }).click();
  const originalHeight = (await moderators.boundingBox())!.height;
  await page.getByRole("button", { name: "Add Problems" }).click();
  await expect(
    page.getByRole("region", { name: "Problems widget" }),
  ).toBeVisible();
  expect((await moderators.boundingBox())!.height).toBeGreaterThanOrEqual(
    originalHeight - 1,
  );
});
