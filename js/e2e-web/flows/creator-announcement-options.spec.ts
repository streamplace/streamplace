import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { completePdsLogin, loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("announcement options summarize changes without saving them", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  const announce = page.getByTestId("creator-announce");
  await expect(status).toHaveText("Live");
  await page
    .getByRole("button", { name: "End Livestream", exact: true })
    .click();
  await page
    .getByRole("dialog", { name: "End announcement?" })
    .getByRole("button", { name: "End announcement", exact: true })
    .click();
  await expect(status).toHaveText("Announcement ended");

  let writes = 0;
  page.on("request", (request) => {
    if (
      request.method() === "POST" &&
      /\/xrpc\/(com\.atproto\.repo\.(putRecord|createRecord|deleteRecord)|place\.stream\.live\.(startLivestream|stopLivestream))$/.test(
        request.url(),
      )
    )
      writes++;
  });
  let optionsChanged = false;
  try {
    const summary = page.getByTestId("creator-announcement-options");
    await expect(summary).toContainText("Push notification: on", {
      timeout: 3_000,
    });
    await expect(summary).toContainText("Automatic end: on (when inactive)");
    await expect(summary).toContainText("Bluesky post: off");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      page.getByRole("button", { name: "Open creator navigation" }),
    ).toBeVisible();
    await expect(summary).toBeVisible();
    await page
      .getByRole("button", { name: "Stream options", exact: true })
      .click();
    optionsChanged = true;
    await page
      .getByRole("checkbox", { name: "Send push notification" })
      .click();
    await page
      .getByRole("checkbox", { name: "End livestream automatically" })
      .click();
    const post = page.getByRole("checkbox", { name: "Create Bluesky post" });
    await post.click({ timeout: 10_000 });
    await expect(summary).toContainText("Bluesky post: on");
    await post.click({ timeout: 10_000 });
    await expect(summary).toContainText("Push notification: off");
    await expect(summary).toContainText("Automatic end: off");
    await page.getByRole("button", { name: "Hide stream options" }).click();
    expect(writes).toBe(0);
  } finally {
    if (optionsChanged) {
      const open = page.getByRole("button", {
        name: "Stream options",
        exact: true,
      });
      if (await open.isVisible()) await open.click();
      for (const name of [
        "Send push notification",
        "End livestream automatically",
      ]) {
        const checkbox = page.getByRole("checkbox", { name });
        if (!(await checkbox.isChecked())) await checkbox.click();
      }
    }
    await announce.click();
    await expect(status).toHaveText("Live");
  }
});

test("explains when Bluesky posting permission was not granted", async ({
  page,
}) => {
  await pointAppAtNode(page, url!);
  await page.goto(`${url}/login`);
  await page
    .getByRole("checkbox", { name: "Show when I'm live on Bluesky" })
    .click();
  await completePdsLogin(page);
  await expect(
    page.getByText(`@${process.env.ACCOUNT_HANDLE}`).first(),
  ).toBeVisible();
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  const announce = page.getByTestId("creator-announce");
  await expect(status).toHaveText("Live");
  await page
    .getByRole("button", { name: "End Livestream", exact: true })
    .click();
  await page
    .getByRole("dialog", { name: "End announcement?" })
    .getByRole("button", { name: "End announcement", exact: true })
    .click();
  await expect(status).toHaveText("Announcement ended");
  try {
    const summary = page.getByTestId("creator-announcement-options");
    await expect(summary).toContainText("Bluesky post: unavailable", {
      timeout: 3_000,
    });
    await page
      .getByRole("button", { name: "Stream options", exact: true })
      .click();
    const post = page.getByRole("checkbox", { name: "Create Bluesky post" });
    await expect(post).toBeDisabled();
    await post.scrollIntoViewIfNeeded();
    await expect(post).toBeVisible();
    await expect(post).toHaveAccessibleDescription(
      "You signed in without Bluesky posting permission. Sign out and back in to change this.",
    );
    const explanation = page.getByTestId("creator-post-permission");
    await explanation.scrollIntoViewIfNeeded({ timeout: 3_000 });
    const textBox = (await explanation.boundingBox())!;
    const widgetBox = (await page
      .getByRole("region", { name: "Stream Info widget" })
      .boundingBox())!;
    expect(textBox.x + textBox.width).toBeLessThanOrEqual(
      widgetBox.x + widgetBox.width,
    );
    await post.hover();
    await expect(
      page.getByText(
        "You signed in without Bluesky posting permission. Sign out and back in to change this.",
        { exact: true },
      ),
    ).toHaveCount(1);
  } finally {
    await announce.click();
    await expect(status).toHaveText("Live");
  }
});
