import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("the default layout gives chat more room while keeping multistream usable", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 1000 });
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const destinations = page.getByTestId("live-stream-destinations");
  await expect(
    destinations.getByText("No multistream destinations yet"),
  ).toBeVisible();
  const chat = page.getByRole("region", { name: "Chat widget" });
  const chatBox = (await chat.boundingBox())!;
  expect(chatBox.y).toBeLessThan(280);

  await page.setViewportSize({ width: 1280, height: 720 });
  const widget = page.getByRole("region", { name: "Multistream widget" });
  const widgetBox = (await widget.boundingBox())!;
  const addBox = (await destinations
    .getByRole("button", { name: "Add destination" })
    .boundingBox())!;
  expect(addBox.y + addBox.height).toBeLessThanOrEqual(
    widgetBox.y + widgetBox.height,
  );
});

test("stream settings use the widget title without losing their controls", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const widget = page.getByRole("region", { name: "Stream Info widget" });
  await expect(
    widget.getByPlaceholder("Enter your stream title..."),
  ).toBeVisible();
  await expect(
    widget.getByText("Stream Settings", { exact: true }),
  ).toHaveCount(1);
  await expect(
    widget.getByRole("link", { name: "Stream Settings" }),
  ).toHaveCount(1);
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  await widget.getByRole("button", { name: "Back to Stream Info" }).click();
  await expect(
    widget.getByPlaceholder("Enter your stream title..."),
  ).toBeVisible();
});
