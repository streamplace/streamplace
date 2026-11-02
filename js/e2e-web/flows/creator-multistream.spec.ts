import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("a failed destination list is not presented as an empty list", async ({
  page,
}) => {
  await loginThroughPds(page);
  let blocked = 0;
  await page.route("**/xrpc/place.stream.multistream.listTargets*", (route) => {
    blocked++;
    return route.abort();
  });
  await page.goto(`${url}/dashboard`);
  const widget = page.getByTestId("live-stream-destinations");
  await expect.poll(() => blocked).toBeGreaterThan(0);
  await expect(widget.getByText("Could not load destinations")).toBeVisible();
  await expect(
    widget.getByText("No multistream destinations yet"),
  ).toBeHidden();
  await page.unrouteAll({ behavior: "wait" });
  await widget.getByRole("button", { name: "Retry" }).click();
  await expect(
    widget.getByText("No multistream destinations yet"),
  ).toBeVisible();
});

test("destination toggles retain confirmed state on failure and refresh after management", async ({
  page,
}) => {
  test.setTimeout(180_000);
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const widget = page.getByTestId("live-stream-destinations");
  await widget.getByRole("button", { name: "Add destination" }).click();
  await page.getByRole("button", { name: "Create Multistream Target" }).click();
  const create = page.getByRole("dialog", { name: "Create Target" });
  await create
    .getByPlaceholder("My Multistream Target")
    .fill("Local test destination");
  await create
    .getByPlaceholder("rtmps://example.com:443/live/foo")
    .fill("rtmp://127.0.0.1:65500/live/test-private-key");
  await create.getByRole("switch").uncheck();
  await create.getByRole("button", { name: "Create", exact: true }).click();
  await expect(
    page.getByText("Local test destination", { exact: true }),
  ).toBeVisible();
  await page.goBack();
  await expect(widget.getByText("Local test destination")).toBeVisible();
  await expect(widget).not.toContainText("test-private-key");
  const toggle = widget.getByRole("switch", { name: "Local test destination" });
  const host = widget.getByText("127.0.0.1", { exact: true });
  const hostBox = await host.boundingBox();
  const switchBox = await toggle.boundingBox();
  expect(hostBox).not.toBeNull();
  expect(switchBox).not.toBeNull();
  expect(hostBox!.x + hostBox!.width).toBeLessThanOrEqual(switchBox!.x - 8);
  await expect(toggle).not.toBeChecked();
  await page.route("**/xrpc/place.stream.multistream.putTarget", (route) =>
    route.abort(),
  );
  await toggle.click();
  await expect(widget.getByText("Could not update destination")).toBeVisible();
  await expect(toggle).not.toBeChecked();
  await page.unrouteAll({ behavior: "wait" });
  await toggle.click();
  await expect(toggle).toBeChecked();
  await page.getByRole("link", { name: "Multistreaming" }).click();
  await expect(
    page.getByText("Local test destination", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Edit multistream target" }).click();
  const edit = page.getByRole("dialog", { name: "Edit Target" });
  await edit
    .getByPlaceholder("My Multistream Target")
    .fill("Renamed destination");
  await edit.getByRole("button", { name: "Update", exact: true }).click();
  await expect(
    page.getByText("Renamed destination", { exact: true }),
  ).toBeVisible();
  await page.goBack();
  await expect(
    widget.getByRole("switch", { name: "Renamed destination" }),
  ).toBeChecked();

  await page.getByRole("link", { name: "Multistreaming" }).click();
  await page
    .getByText("Multistream Targets", { exact: true })
    .locator("../..")
    .getByRole("button", { name: "Create", exact: true })
    .click();
  const second = page.getByRole("dialog", { name: "Create Target" });
  await second
    .getByPlaceholder("My Multistream Target")
    .fill("Second destination");
  await second
    .getByPlaceholder("rtmps://example.com:443/live/foo")
    .fill("rtmp://127.0.0.1:65501/live/other-private-key");
  await second.getByRole("switch").uncheck();
  await second.getByRole("button", { name: "Create", exact: true }).click();
  await expect(
    page.getByText("Second destination", { exact: true }),
  ).toBeVisible();
  await page.goBack();
  const firstToggle = widget.getByRole("switch", {
    name: "Renamed destination",
  });
  const secondToggle = widget.getByRole("switch", {
    name: "Second destination",
  });
  await expect(firstToggle).toBeChecked();
  await expect(secondToggle).not.toBeChecked();
  let release!: () => void;
  let started = false;
  const held = new Promise<void>((resolve) => {
    release = resolve;
  });
  await page.route(
    "**/xrpc/place.stream.multistream.putTarget",
    async (route) => {
      if (
        route.request().postDataJSON().multistreamTarget.name ===
        "Renamed destination"
      ) {
        started = true;
        await held;
      }
      await route.continue();
    },
  );
  try {
    await firstToggle.click();
    await expect.poll(() => started).toBe(true);
    await expect(firstToggle).toBeDisabled();
    await expect(secondToggle).toBeEnabled();
    await secondToggle.click();
    await expect(secondToggle).toBeChecked();
    release();
    await expect(firstToggle).not.toBeChecked();
    await expect(widget.getByText("Inactive", { exact: true })).toBeVisible();
  } finally {
    release();
    await page.unrouteAll({ behavior: "wait" });
  }
});
