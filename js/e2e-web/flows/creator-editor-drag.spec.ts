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
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  const monitor = page.getByRole("region", { name: "Stream Monitor widget" });
  const from = (await drag.boundingBox())!;
  const target = (await monitor.boundingBox())!;
  const dx = target.x + target.width / 2 - (from.x + from.width / 2);
  const dy = target.y + target.height * 0.15 - (from.y + from.height / 2);
  await expect(
    page.getByRole("button", { name: "Move Moderators", exact: true }),
  ).toHaveCount(0, { timeout: 5_000 });
  await drag.focus();
  await page.keyboard.press("Space");
  await expect(
    page.getByRole("status").filter({ hasText: "Picked up Moderators" }),
  ).toHaveCount(1);
  for (let i = 0; i < Math.round(Math.abs(dx) / 50); i++)
    await page.keyboard.press(dx < 0 ? "Shift+ArrowLeft" : "Shift+ArrowRight");
  for (let i = 0; i < Math.round(Math.abs(dy) / 50); i++)
    await page.keyboard.press(dy < 0 ? "Shift+ArrowUp" : "Shift+ArrowDown");
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await expect(
    page
      .getByRole("status")
      .filter({ hasText: "Moving Moderators above Stream Monitor" }),
  ).toHaveCount(1);
  await page.keyboard.press("Space");
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  await expect
    .poll(
      async () =>
        (await moderators.boundingBox())!.y < (await monitor.boundingBox())!.y,
    )
    .toBe(true);
  await expect(drag).toBeFocused();
  await page.keyboard.press("Space");
  await expect(
    page.getByRole("status").filter({ hasText: "Picked up Moderators" }),
  ).toHaveCount(1);
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("status").filter({ hasText: "Move canceled" }),
  ).toHaveCount(1);
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await monitor.boundingBox())!.y,
  );
});

test("a split that would make widgets too small is rejected", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  const chat = page.getByRole("region", { name: "Chat widget" });
  const from = (await drag.boundingBox())!;
  const to = (await chat.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width * 0.15, to.y + to.height / 2, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await page.mouse.up();
  await expect(page.getByRole("alert")).toHaveText(
    "There is not enough room to place these widgets there.",
    { timeout: 5_000 },
  );
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  expect((await moderators.boundingBox())!.x).toBeGreaterThan(
    (await chat.boundingBox())!.x,
  );
});

test("keyboard users are told when a widget is dropped without a target", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  await drag.focus();
  await page.keyboard.press("Space");
  await expect(
    page.getByRole("status").filter({ hasText: "Picked up Moderators" }),
  ).toHaveCount(1);
  await page.keyboard.press("Space");
  await expect(
    page.getByRole("status").filter({ hasText: "No drop target" }),
  ).toHaveCount(1, { timeout: 5_000 });
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
  context,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const drag = page.getByRole("button", { name: "Drag Stream Health" });
  const health = page.getByRole("region", { name: "Stream Health widget" });
  const monitor = page.getByRole("region", { name: "Stream Monitor widget" });
  const from = (await drag.boundingBox())!;
  const target = (await monitor.boundingBox())!;
  const start = { x: from.x + from.width / 2, y: from.y + from.height / 2 };
  const end = {
    x: target.x + target.width / 2,
    y: target.y + target.height * 0.15,
  };
  const cdp = await context.newCDPSession(page);
  await cdp.send("Emulation.setTouchEmulationEnabled", { enabled: true });
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchStart",
    touchPoints: [start],
  });
  // Touch dragging starts after a short hold so scrolling remains possible.
  await page.waitForTimeout(300);
  for (let i = 1; i <= 20; i++) {
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [
        {
          x: start.x + ((end.x - start.x) * i) / 20,
          y: start.y + ((end.y - start.y) * i) / 20,
        },
      ],
    });
  }
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await cdp.send("Input.dispatchTouchEvent", {
    type: "touchEnd",
    touchPoints: [],
  });
  expect((await health.boundingBox())!.y).toBeLessThan(
    (await monitor.boundingBox())!.y,
  );
  await page.reload();
  expect((await health.boundingBox())!.y).toBeLessThan(
    (await monitor.boundingBox())!.y,
  );
});
