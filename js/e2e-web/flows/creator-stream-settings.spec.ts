import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { completePdsLogin, loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("stream settings require sign-in and return to the settings page", async ({
  page,
}) => {
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/settings/streaming`);
  await expect(page.getByRole("checkbox", { name: "Language" })).toHaveCount(0);
  await page
    .getByRole("button", { name: "Log in to manage stream settings" })
    .click();
  await completePdsLogin(page);
  await expect(page).toHaveURL(`${url}/settings/streaming`);
  await expect(page.getByRole("checkbox", { name: "Language" })).toBeVisible();
});

test("a creator can save warnings before setting up metadata", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const widget = page.getByRole("region", { name: "Stream Info widget" });
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  const warning = widget.getByRole("checkbox", { name: "Language" });
  await warning.click();
  await widget
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await page.reload();
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  await expect(warning).toBeChecked();
  await warning.click();
  await widget
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await page.reload();
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  await expect(warning).not.toBeChecked();
});

test("stream settings keep content metadata and chat access available", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/settings/streaming`);
  await expect(
    page.getByText("Content metadata", { exact: true }),
  ).toBeVisible();
  await expect(page.getByText("Chat access", { exact: true })).toBeVisible();
  await page.getByText("Rights", { exact: true }).click();
  const notice = page.getByPlaceholder("Enter your copyright notice...");
  await notice.fill("A creator's copyright notice");
  const save = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      /\/xrpc\/com\.atproto\.repo\.(createRecord|putRecord)$/.test(
        response.url(),
      ) &&
      response.request().postDataJSON()?.collection ===
        "place.stream.metadata.configuration",
  );
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await save).ok()).toBe(true);
  await page.reload();
  await page.getByText("Rights", { exact: true }).click();
  await expect(notice).toHaveValue("A creator's copyright notice");
  await notice.fill("");
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await page.reload();
  await page.getByText("Rights", { exact: true }).click();
  await expect(notice).toHaveValue("");
});

test("dashboard links to stream settings without losing a draft", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const widget = page.getByRole("region", { name: "Stream Info widget" });
  const title = widget.getByPlaceholder("Enter your stream title...");
  await title.fill("A title still in progress");
  await expect(widget.getByText("Moderation", { exact: true })).toHaveCount(0);
  const settings = widget.getByRole("link", { name: /stream settings/i });
  const popupPromise = page.waitForEvent("popup");
  await settings.click();
  const popup = await popupPromise;
  await expect(popup).toHaveURL(`${url}/dashboard/stream`);
  await expect(
    popup.getByText("Content metadata", { exact: true }),
  ).toBeVisible();
  await expect(title).toHaveValue("A title still in progress");
});

test("quick content warnings preserve the rest of the saved metadata", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/settings/streaming`);
  await page.getByText("Rights", { exact: true }).click();
  const notice = page.getByPlaceholder("Enter your copyright notice...");
  await notice.fill("Retain this notice");
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await page.goto(`${url}/dashboard`);
  const widget = page.getByRole("region", { name: "Stream Info widget" });
  const title = widget.getByPlaceholder("Enter your stream title...");
  await title.fill("Keep this draft");
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  await expect(
    widget.getByRole("checkbox", { name: "Death" }),
  ).toBeInViewport();
  const warning = widget.getByRole("checkbox", { name: "Death" });
  if (!(await warning.isChecked())) await warning.click();
  await widget.getByRole("button", { name: "Back to Stream Info" }).click();
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  await expect(warning).toBeChecked();
  const settings = await page.context().newPage();
  await settings.goto(`${url}/settings/streaming`);
  await settings.getByText("Rights", { exact: true }).click();
  await settings
    .getByPlaceholder("Enter your copyright notice...")
    .fill("Updated while editing warnings");
  await settings
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    settings.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await widget
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await widget.getByRole("button", { name: "Back to Stream Info" }).click();
  await expect(title).toHaveValue("Keep this draft");
  await page.goto(`${url}/settings/streaming`);
  await page.getByText("Rights", { exact: true }).click();
  await expect(notice).toHaveValue("Updated while editing warnings");
  await page.getByText("Warnings", { exact: true }).click();
  await expect(page.getByRole("checkbox", { name: "Death" })).toBeChecked();
});

test("saving metadata preserves warnings changed in another tab", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/settings/streaming`);
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await page.getByText("Rights", { exact: true }).click();
  await page
    .getByPlaceholder("Enter your copyright notice...")
    .fill("New notice");
  const dashboard = await page.context().newPage();
  await dashboard.goto(`${url}/dashboard`);
  const widget = dashboard.getByRole("region", { name: "Stream Info widget" });
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  const warning = widget.getByRole("checkbox", { name: "Flashing Lights" });
  if (!(await warning.isChecked())) await warning.click();
  const warningSaved = dashboard.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
      response
        .request()
        .postDataJSON()
        ?.record?.contentWarnings?.warnings?.includes(
          "place.stream.metadata.contentWarnings#flashingLights",
        ),
  );
  await widget
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await warningSaved).ok()).toBe(true);
  const noticeSaved = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
      response.request().postDataJSON()?.record?.contentRights
        ?.copyrightNotice === "New notice",
  );
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await noticeSaved).ok()).toBe(true);
  await page.reload();
  await expect(
    page.getByRole("checkbox", { name: "Flashing Lights" }),
  ).toBeChecked();
});

test("a concurrent metadata save cannot silently overwrite a newer record", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/settings/streaming`);
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await page.getByText("Rights", { exact: true }).click();
  const notice = page.getByPlaceholder("Enter your copyright notice...");
  await notice.fill("Unsent notice");

  let release!: () => void;
  let intercepted!: () => void;
  const held = new Promise<void>((resolve) => (intercepted = resolve));
  const resume = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/xrpc/com.atproto.repo.putRecord", async (route) => {
    if (
      route.request().postDataJSON()?.collection ===
      "place.stream.metadata.configuration"
    ) {
      intercepted();
      await resume;
    }
    await route.continue();
  });
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await held;

  const dashboard = await page.context().newPage();
  await dashboard.goto(`${url}/dashboard`);
  const widget = dashboard.getByRole("region", { name: "Stream Info widget" });
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  const warning = widget.getByRole("checkbox", {
    name: "Violence",
    exact: true,
  });
  if (!(await warning.isChecked())) await warning.click();
  const saved = dashboard.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      response.url().endsWith("/xrpc/com.atproto.repo.putRecord") &&
      response
        .request()
        .postDataJSON()
        ?.record?.contentWarnings?.warnings?.includes(
          "place.stream.metadata.contentWarnings#violence",
        ),
  );
  await widget
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  expect((await saved).ok()).toBe(true);
  release();
  await expect(page.getByText("Settings changed elsewhere")).toBeVisible();
  await expect(notice).toHaveValue("Unsent notice");
  await page.unrouteAll({ behavior: "wait" });
  await page
    .getByRole("button", { name: /save metadata|update metadata/i })
    .click();
  await expect(
    page.getByText(/content metadata (created|updated)/i),
  ).toBeVisible();
  await dashboard.reload();
  await widget.getByRole("button", { name: "Edit content warnings" }).click();
  await expect(warning).toBeChecked();
});
