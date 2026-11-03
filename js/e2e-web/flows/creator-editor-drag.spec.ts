import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("dragging a widget onto an edge rearranges the layout without losing drafts", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("A title in progress");
  await chat.fill("A message in progress");
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  await expect(drag).toBeVisible({ timeout: 5_000 });
  await expect(
    page.getByRole("button", { name: "Remove Moderators" }),
  ).toBeEnabled();
  const target = page.getByRole("region", { name: "Stream Monitor widget" });
  const from = (await drag.boundingBox())!;
  const to = (await target.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height * 0.15, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await expect
    .poll(
      async () =>
        (await page.getByTestId("creator-drop-preview").boundingBox())!.y,
    )
    .toBeLessThan(to.y + to.height * 0.25);
  await page.mouse.up();
  await expect(title).toHaveValue("A title in progress");
  await expect(chat).toHaveValue("A message in progress");
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await target.boundingBox())!.y,
  );
  await page.reload();
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await target.boundingBox())!.y,
  );
});

test("a keyboard user can move a widget beside another", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const move = page.getByRole("button", {
    name: "Move Moderators",
    exact: true,
  });
  await expect(move).toBeVisible({ timeout: 5_000 });
  await expect(move).toBeEnabled();
  await move.focus();
  await page.keyboard.press("Enter");
  await expect(
    page.getByText("Moving Moderators", { exact: true }),
  ).toBeVisible();
  await page
    .getByLabel("Move next to")
    .selectOption({ label: "Stream Monitor" });
  await page.getByRole("button", { name: "Move above" }).click();
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  const monitor = page.getByRole("region", { name: "Stream Monitor widget" });
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await monitor.boundingBox())!.y,
  );
  await expect(
    page.getByRole("button", { name: "Move Moderators", exact: true }),
  ).toBeFocused();
});

test("a split that would make widgets too small is rejected", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page
    .getByRole("button", { name: "Move Moderators", exact: true })
    .click();
  await page.getByLabel("Move next to").selectOption({ label: "Chat" });
  await page.getByRole("button", { name: "Move left" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "There is not enough room to place these widgets there.",
    { timeout: 5_000 },
  );
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  const chat = page.getByRole("region", { name: "Chat widget" });
  expect((await moderators.boundingBox())!.x).toBeGreaterThan(
    (await chat.boundingBox())!.x,
  );
});

test("a quick drop uses the side where the pointer is released", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1920, height: 1080 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  const target = page.getByRole("region", { name: "Stream Monitor widget" });
  const from = (await drag.boundingBox())!;
  const to = (await target.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width * 0.85, to.y + to.height / 2, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await page.mouse.move(to.x + to.width * 0.15, to.y + to.height / 2);
  await page.mouse.up();
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  await expect
    .poll(async () => (await moderators.boundingBox())!.x)
    .toBeLessThan((await target.boundingBox())!.x);
});

test("a creator can move a widget in the stacked phone layout", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page
    .getByRole("button", { name: "Move Moderators", exact: true })
    .click();
  await page
    .getByLabel("Move next to")
    .selectOption({ label: "Stream Monitor" });
  await page.getByRole("button", { name: "Move above" }).click();
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  const monitor = page.getByRole("region", { name: "Stream Monitor widget" });
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await monitor.boundingBox())!.y,
  );
  await page.reload();
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await monitor.boundingBox())!.y,
  );
});
