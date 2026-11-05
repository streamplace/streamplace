import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("destination management reports connection failures and recovers", async ({
  page,
}) => {
  test.setTimeout(120_000);
  await loginThroughPds(page);
  await page.goto(`${url}/settings/streaming/multistream`);
  await expect(
    page.getByRole("button", { name: "Refresh", exact: true }),
  ).toBeEnabled();
  await page
    .getByText("Multistream Targets", { exact: true })
    .locator("../..")
    .getByRole("button", { name: "Create", exact: true })
    .click();
  const create = page.getByRole("dialog", { name: "Create Target" });
  await create
    .getByPlaceholder("My Multistream Target")
    .fill("Connection test destination");
  await create
    .getByPlaceholder("rtmps://example.com:443/live/foo")
    .fill("rtmp://127.0.0.1:65500/live/test-key");
  await create.getByRole("switch").uncheck();
  await create.getByRole("button", { name: "Create", exact: true }).click();
  const name = page.getByText("Connection test destination", { exact: true });
  await expect(name).toBeVisible();
  const row = page
    .locator("div")
    .filter({ has: name })
    .filter({
      has: page.getByRole("button", { name: "Edit multistream target" }),
    })
    .last()
    .locator("..");
  const toggle = row.getByRole("switch");
  let failedRequests = 0;
  page.on("requestfailed", (request) => {
    if (request.url().includes("place.stream.multistream.")) failedRequests++;
  });
  try {
    await expect(toggle).not.toBeChecked();
    await page.context().setOffline(true);
    await toggle.click();
    await expect(
      row.getByText("Failed to toggle multistream target. Please try again."),
    ).toBeVisible();
    await expect(toggle).not.toBeChecked();

    await page.getByRole("button", { name: "Refresh", exact: true }).click();
    await expect(
      page.getByText("Failed to load multistream targets. Please try again."),
    ).toBeVisible();
    await expect(name).toBeVisible();
    await expect(page.getByText("No multistream targets yet")).toHaveCount(0);
    expect(failedRequests).toBeGreaterThanOrEqual(2);

    await page.context().setOffline(false);
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await expect(
      page.getByText("Failed to load multistream targets. Please try again."),
    ).toHaveCount(0);
    const updated = page.waitForResponse((response) =>
      response.url().includes("place.stream.multistream.putTarget"),
    );
    await toggle.click();
    expect((await updated).ok()).toBe(true);
    await expect(toggle).toBeChecked();
    await page.reload();
    await expect(name).toBeVisible();
    await expect(toggle).toBeChecked();
  } finally {
    await page.context().setOffline(false);
    const retry = page.getByRole("button", { name: "Retry", exact: true });
    if (await retry.isVisible()) {
      await retry.click();
      await expect(retry).toHaveCount(0);
    }
    if (await name.isVisible()) {
      await row
        .getByRole("button", { name: "Delete multistream target" })
        .click();
      const dialog = page.getByRole("dialog", { name: "Delete Target" });
      await dialog.getByRole("button", { name: "Delete", exact: true }).click();
      await expect(name).toHaveCount(0);
    }
  }
});

test("an initial connection failure is not presented as an empty list", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const listUrl = "**/xrpc/place.stream.multistream.listTargets**";
  await page.route(listUrl, async (route) => {
    await page.context().setOffline(true);
    await route.continue();
  });
  try {
    await page.getByRole("link", { name: "Multistreaming" }).click();
    await expect(
      page.getByText("Failed to load multistream targets. Please try again."),
    ).toBeVisible();
    await expect(page.getByText("No multistream targets yet")).toHaveCount(0);
    await page.unroute(listUrl);
    await page.context().setOffline(false);
    await page.getByRole("button", { name: "Retry", exact: true }).click();
    await expect(
      page.getByText("Failed to load multistream targets. Please try again."),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Refresh", exact: true }),
    ).toBeEnabled();
  } finally {
    await page.unroute(listUrl);
    await page.context().setOffline(false);
  }
});
