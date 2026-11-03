import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("dragging works without a separate move panel", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  const target = page.getByRole("region", { name: "Stream Monitor widget" });
  const from = (await drag.boundingBox())!;
  const to = (await target.boundingBox())!;
  await expect(
    page.getByRole("button", { name: "Move Moderators", exact: true }),
  ).toHaveCount(0);
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width / 2, to.y + to.height * 0.15, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await page.mouse.up();
  const moderators = page.getByRole("region", { name: "Moderators widget" });
  expect((await moderators.boundingBox())!.y).toBeLessThan(
    (await target.boundingBox())!.y,
  );
  await expect(
    page.getByRole("group", { name: "Move Moderators" }),
  ).toHaveCount(0);
});

test("switching to layouts clears a rejected move", async ({ page }) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const drag = page.getByRole("button", { name: "Drag Moderators" });
  const target = page.getByRole("region", { name: "Chat widget" });
  const from = (await drag.boundingBox())!;
  const to = (await target.boundingBox())!;
  await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2);
  await page.mouse.down();
  await page.mouse.move(to.x + to.width * 0.15, to.y + to.height / 2, {
    steps: 20,
  });
  await expect(page.getByTestId("creator-drop-preview")).toBeVisible();
  await page.mouse.up();
  await expect(page.getByRole("alert")).toBeVisible();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(
    page.getByRole("group", { name: "Move Moderators" }),
  ).toHaveCount(0);
});

test("save, load, delete and reset layouts without losing other drafts", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  await page.getByRole("button", { name: "Save layout" }).click();
  await page.getByLabel("Layout name").fill("Studio");
  await expect(
    page.getByText("Saving will replace the existing layout."),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.reload();
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("Draft title");
  await chat.fill("Draft chat");
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Remove Moderators" }).click();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  await page.getByLabel("Load layout").selectOption({ label: "Studio" });
  await expect(page.getByTestId("creator-moderators")).toBeVisible();
  await expect(title).toHaveValue("Draft title");
  await expect(chat).toHaveValue("Draft chat");
  await page.getByRole("button", { name: "Remove Moderators" }).click();
  await page.getByRole("button", { name: "Save layout" }).click();
  await page.getByLabel("Layout name").fill("Studio");
  await expect(
    page.getByText("Saving will replace the existing layout."),
  ).toBeVisible();
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.getByRole("button", { name: "Reset layout" }).click();
  await expect(page.getByTestId("creator-moderators")).toBeVisible();
  await page.getByLabel("Load layout").selectOption({ label: "Studio" });
  await expect(page.getByTestId("creator-moderators")).toHaveCount(0);
  await page.getByRole("button", { name: "Delete Studio" }).click();
  await expect(page.getByLabel("Load layout")).toHaveCount(0);
  await page.getByRole("button", { name: "Reset layout" }).click();
  await expect(page.getByTestId("creator-moderators")).toBeVisible();
  await expect(title).toHaveValue("Draft title");
  await expect(chat).toHaveValue("Draft chat");
  await page.reload();
  await expect(page.getByTestId("creator-moderators")).toBeVisible();
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  await expect(page.getByLabel("Load layout")).toHaveCount(0);
});

test("show an error when a saved layout cannot be written", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const original = Storage.prototype.setItem;
    Storage.prototype.setItem = function (key, value) {
      if (key === "dashboard:presets")
        throw new DOMException("Storage unavailable", "SecurityError");
      return original.call(this, key, value);
    };
  });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  await page.getByRole("button", { name: "Save layout" }).click();
  await page.getByLabel("Layout name").fill("Studio");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText(
    "Could not save your layouts",
  );
  await expect(page.getByText("Studio saved", { exact: true })).toHaveCount(0);
  await expect(
    page.getByText("Studio kept for this session", { exact: true }),
  ).toHaveCount(1);
  await expect(page.getByPlaceholder("Type a message...")).toBeVisible();
});

test("preserve unreadable saved layouts and warn the creator", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.evaluate(() =>
    localStorage.setItem("dashboard:presets", "{broken"),
  );
  await page.goto(`${url}/dashboard`);
  await expect(page.getByRole("alert")).toContainText(
    "Could not read all saved layouts",
  );
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Layouts", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Save layout" }),
  ).toBeDisabled();
  expect(
    await page.evaluate(() => localStorage.getItem("dashboard:presets")),
  ).toBe("{broken");
});
