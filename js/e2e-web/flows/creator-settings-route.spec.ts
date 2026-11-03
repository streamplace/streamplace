import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { completePdsLogin, loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("creator stream settings retain the dashboard shell and draft", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const title = page.getByPlaceholder("Enter your stream title...");
  await title.fill("Draft before opening settings");
  const nav = page.getByRole("navigation", { name: "Creator navigation" });
  await nav.getByRole("link", { name: "Stream Settings" }).click();
  await expect(page).toHaveURL(`${url}/dashboard/stream`);
  await expect(
    nav.getByRole("link", { name: "Stream Settings" }),
  ).toHaveAttribute("aria-current", "page");
  await expect(
    page.getByRole("link", { name: "Home", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByText("Content metadata", { exact: true }),
  ).toBeVisible();
  await expect
    .poll(async () => (await nav.boundingBox())?.x)
    .toBeLessThanOrEqual(2);
  await expect(page.getByText("Chat access", { exact: true })).toBeVisible();
  await page.goBack();
  await expect(title).toHaveValue("Draft before opening settings");
});

test("creator stream settings support direct entry and login return", async ({
  page,
}) => {
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/dashboard/stream`);
  await completePdsLogin(page);
  await expect(page).toHaveURL(`${url}/dashboard/stream`);
  await expect(
    page.getByRole("navigation", { name: "Creator navigation" }),
  ).toBeVisible();
  await expect(
    page.getByText("Content metadata", { exact: true }),
  ).toBeVisible();
});
