import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("add and remove widgets without losing stream or chat drafts", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  const chat = page.getByPlaceholder("Type a message...");
  await title.fill("An unsaved stream title");
  await chat.fill("An unsent chat message");

  await expect(
    page.getByRole("button", { name: "Customize dashboard" }),
  ).toBeVisible({ timeout: 5_000 });
  expect(
    (await page
      .getByRole("button", { name: "Customize dashboard" })
      .boundingBox())!.height,
  ).toBeLessThan(60);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const healthHeight = (await page.getByTestId("creator-health").boundingBox())!
    .height;
  const remove = page.getByRole("button", { name: "Remove Moderators" });
  await remove.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByTestId("creator-moderators")).toHaveCount(0);
  await expect(title).toHaveValue("An unsaved stream title");
  await expect(chat).toHaveValue("An unsent chat message");

  const add = page.getByRole("button", { name: "Add widget" });
  await expect(add).toBeFocused();
  await add.click();
  await page.getByRole("button", { name: "Add Moderators" }).click();
  await expect(page.getByTestId("creator-moderators")).toBeVisible();
  expect(
    (await page.getByTestId("creator-health").boundingBox())!.height,
  ).toBeGreaterThanOrEqual(healthHeight - 2);
  await expect(title).toHaveValue("An unsaved stream title");
  await expect(chat).toHaveValue("An unsent chat message");
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(title).toHaveValue("An unsaved stream title");
  await expect(chat).toHaveValue("An unsent chat message");
  await page.getByRole("button", { name: "Done customizing" }).click();
  await page.reload();
  await expect(page.getByTestId("creator-moderators")).toBeVisible();
});

test("a legacy nested layout and unknown widget survive until explicitly removed", async ({
  page,
}) => {
  await loginThroughPds(page);
  const saved = {
    type: "split",
    direction: "horizontal",
    ratio: [2, 1],
    children: [
      {
        type: "split",
        direction: "vertical",
        ratio: [1, 1],
        children: [
          {
            type: "leaf",
            widget: "stream-monitor",
            config: { example: "keep" },
          },
          { type: "leaf", widget: "chat" },
        ],
      },
      { type: "leaf", widget: "future-widget", config: { version: 2 } },
    ],
  };
  await page.evaluate(
    (layout) =>
      localStorage.setItem("dashboard:layout", JSON.stringify(layout)),
    saved,
  );
  await page.goto(`${url}/dashboard`);
  await expect(page.getByPlaceholder("Type a message...")).toBeVisible();
  await expect(page.getByText("Unknown widget: future-widget")).toBeVisible();
  expect(
    await page.evaluate(() =>
      JSON.parse(localStorage.getItem("dashboard:layout")!),
    ),
  ).toEqual(saved);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Remove future-widget" }).click();
  await page.getByRole("button", { name: "Add widget" }).click();
  await page.getByRole("button", { name: "Add Stream Health" }).click();
  await expect(page.getByRole("alert")).toHaveText(
    "There is not enough room to place these widgets there.",
  );
  await page.setViewportSize({ width: 1920, height: 1800 });
  await page.getByRole("button", { name: "Add Stream Health" }).click();
  await expect(page.getByTestId("creator-health")).toBeVisible();
  await page.setViewportSize({ width: 1280, height: 720 });
  const stored = await page.evaluate(() =>
    localStorage.getItem("dashboard:layout"),
  );
  expect(stored).toContain('"example":"keep"');
  await page.reload();
  await expect(page.getByTestId("creator-health")).toBeVisible();
  await expect(page.getByText("Unknown widget: future-widget")).toHaveCount(0);
});

test("a storage failure leaves the current dashboard usable and visible", async ({
  page,
}) => {
  await page.addInitScript(() => {
    const original = Storage.prototype.setItem;
    Storage.prototype.setItem = function (key, value) {
      if (key === "dashboard:layout")
        throw new DOMException("Storage unavailable", "SecurityError");
      return original.call(this, key, value);
    };
  });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Remove Moderators" }).click();
  await expect(
    page.getByText(
      "Could not save your dashboard layout. Changes will last until you leave this page.",
    ),
  ).toBeVisible();
  await expect(page.getByTestId("creator-moderators")).toHaveCount(0);
  await expect(page.getByPlaceholder("Type a message...")).toBeVisible();
});

test("a zero-sized saved widget can be removed and the last widget can be replaced", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.evaluate(() =>
    localStorage.setItem(
      "dashboard:layout",
      JSON.stringify({
        type: "split",
        direction: "horizontal",
        ratio: [1, 0],
        children: [
          { type: "leaf", widget: "chat" },
          { type: "leaf", widget: "future-widget" },
        ],
      }),
    ),
  );
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page
    .getByRole("button", { name: "Remove future-widget" })
    .first()
    .click();
  await page.getByRole("button", { name: "Remove Chat" }).click();
  await expect(page.getByText("Add a widget to your dashboard.")).toBeVisible();
  await page.getByRole("button", { name: "Add widget" }).click();
  await page.getByRole("button", { name: "Add Chat" }).click();
  await expect(page.getByPlaceholder("Type a message...")).toBeVisible();
});

test("the add menu can be dismissed with the keyboard", async ({ page }) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  const add = page.getByRole("button", { name: "Add widget" });
  await add.click();
  await page.getByRole("button", { name: "Add Problems" }).focus();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("group", { name: "Available widgets" }),
  ).toHaveCount(0);
  await expect(add).toBeFocused();
  await add.click();
  await page.getByRole("button", { name: "Add Problems" }).focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("alert")).toHaveText(
    "There is not enough room to place these widgets there.",
  );
  await expect(
    page.getByRole("button", { name: "Add Problems" }),
  ).toBeFocused();
  await page.setViewportSize({ width: 1920, height: 1080 });
  await page.keyboard.press("Enter");
  await expect(page.getByText("No current problems")).toBeVisible();
  await page.getByRole("button", { name: "Remove Problems" }).click();
});

test("a saved widget named like an object property remains recoverable", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.evaluate(() =>
    localStorage.setItem(
      "dashboard:layout",
      JSON.stringify({
        type: "leaf",
        widget: "constructor",
        config: { version: 1 },
      }),
    ),
  );
  await page.goto(`${url}/dashboard`);
  await expect(page.getByText("Unknown widget: constructor")).toBeVisible();
  await page.getByRole("button", { name: "Customize dashboard" }).click();
  await page.getByRole("button", { name: "Remove constructor" }).click();
  await expect(page.getByText("Add a widget to your dashboard.")).toBeVisible();
});
