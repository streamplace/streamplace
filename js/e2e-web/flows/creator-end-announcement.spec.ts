import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";
import { loginThroughPds } from "./login";

const url = process.env.SERVER_HTTPS_URL;
test.skip(!url, "harness started without its HTTPS hostnames");

test("ending an announcement leaves an existing viewer receiving video", async ({
  page,
  browser,
}) => {
  const viewerContext = await browser.newContext({
    ignoreHTTPSErrors: true,
    ...(process.env.E2E_PROXY_URL
      ? {
          proxy: {
            server: process.env.E2E_PROXY_URL,
            bypass: "127.0.0.1,localhost",
          },
        }
      : {}),
  });
  const viewer = await viewerContext.newPage();
  await pointAppAtNode(viewer, url!);
  await viewer.goto(`${url}/`);
  await viewer.getByTestId("home-stream-card").first().click();
  const video = viewer.locator("video").first();
  await expect(video).toBeVisible();
  await expect
    .poll(() => video.evaluate((el: HTMLVideoElement) => el.currentTime))
    .toBeGreaterThan(1);

  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  const announce = page.getByTestId("creator-announce");
  await expect(status).toHaveText("Live");
  try {
    await page
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    await expect(
      dialog.getByText(
        "Ending the announcement does not stop video from your streaming app. To stop sending video, stop the stream in OBS or your streaming app.",
      ),
    ).toBeVisible();
    await dialog
      .getByRole("button", { name: "End announcement", exact: true })
      .click();
    await expect(status).toHaveText("Announcement ended");
    await expect(
      page.getByText(
        "Video is still arriving. To stop sending video, stop the stream in OBS or your streaming app.",
      ),
    ).toBeVisible();
    const before = await video.evaluate(
      (el: HTMLVideoElement) => el.currentTime,
    );
    await expect
      .poll(() => video.evaluate((el: HTMLVideoElement) => el.currentTime))
      .toBeGreaterThan(before + 2);
  } finally {
    if (await announce.getByText("Announce stream").isVisible()) {
      await announce.click();
      await expect(status).toHaveText("Live");
    }
    await viewerContext.close();
  }
});
