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
  const title = page.getByPlaceholder("Enter your stream title...");
  const originalTitle = await title.inputValue();
  await title.fill("Unfinished title");
  try {
    await page
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    await expect(
      dialog.getByText(
        "Ending the announcement doesn't stop video from your streaming app. People watching may still receive it. To stop sending video, stop streaming in OBS or your streaming app.",
      ),
    ).toBeVisible();
    await dialog
      .getByRole("button", { name: "End announcement", exact: true })
      .click();
    await expect(status).toHaveText("Announcement ended");
    await expect(title).toHaveValue("Unfinished title");
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
      await title.fill(originalTitle);
      await announce.click();
      await expect(status).toHaveText("Live");
    }
    await viewerContext.close();
  }
});

test("shows progress while the real end request is pending", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  await expect(status).toHaveText("Live");

  let releaseRequest!: () => void;
  const held = new Promise<void>((resolve) => {
    releaseRequest = resolve;
  });
  let requestStarted!: () => void;
  const started = new Promise<void>((resolve) => {
    requestStarted = resolve;
  });
  await page.route(
    "**/xrpc/place.stream.live.stopLivestream",
    async (route) => {
      requestStarted();
      await held;
      await route.continue();
    },
  );
  try {
    await page
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    await dialog
      .getByRole("button", { name: "End announcement", exact: true })
      .click();
    await started;
    await expect(
      dialog.getByRole("button", { name: "Ending..." }),
    ).toBeDisabled({ timeout: 3_000 });
    await expect(dialog.getByRole("button", { name: "Cancel" })).toBeDisabled();
    await dialog.getByRole("button", { name: "Close" }).click();
    await expect(page.getByTestId("creator-announce")).toBeDisabled({
      timeout: 3_000,
    });
    releaseRequest();
    await expect(status).toHaveText("Announcement ended");
  } finally {
    releaseRequest();
    await page.unrouteAll({ behavior: "wait" });
    const announce = page.getByTestId("creator-announce");
    if (await announce.getByText("Announce stream").isVisible()) {
      await announce.click();
      await expect(status).toHaveText("Live");
    }
  }
});

test("explains an uncertain end request and allows a retry after reconnecting", async ({
  page,
}) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  const status = page.getByTestId("creator-stream-status");
  await expect(status).toHaveText("Live");
  let releaseRequest!: () => void;
  const held = new Promise<void>((resolve) => {
    releaseRequest = resolve;
  });
  let requestStarted!: () => void;
  const started = new Promise<void>((resolve) => {
    requestStarted = resolve;
  });
  await page.route(
    "**/xrpc/place.stream.live.stopLivestream",
    async (route) => {
      requestStarted();
      await held;
      await route.continue();
    },
  );
  try {
    await page
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    await dialog
      .getByRole("button", { name: "End announcement", exact: true })
      .click();
    await started;
    await page.context().setOffline(true);
    releaseRequest();
    await expect(
      dialog.getByText(
        "Couldn't confirm whether the announcement ended. Check your connection and status before trying again.",
      ),
    ).toBeVisible();
    await page.context().setOffline(false);
    await page.unrouteAll({ behavior: "wait" });
    await expect(status).toHaveText("Live");
    await dialog
      .getByRole("button", { name: "End announcement", exact: true })
      .click();
    await expect(status).toHaveText("Announcement ended");
  } finally {
    releaseRequest();
    await page.context().setOffline(false);
    await page.unrouteAll({ behavior: "wait" });
    const announce = page.getByTestId("creator-announce");
    if (await announce.getByText("Announce stream").isVisible()) {
      await announce.click();
      await expect(status).toHaveText("Live");
    }
  }
});

test("warns when ending can publish a recording", async ({ page }) => {
  await loginThroughPds(page);
  await page.goto(`${url}/dashboard`);
  await expect(page.getByTestId("creator-stream-status")).toHaveText("Live");
  await page
    .getByRole("button", { name: "Stream options", exact: true })
    .click();
  const toggle = page.getByTestId("live-auto-publish-vods");
  await expect(toggle).toBeEnabled();
  const initial = (await toggle.getAttribute("aria-checked")) === "true";
  try {
    if (!initial) {
      const saved = page.waitForResponse((response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord"),
      );
      await toggle.click();
      expect((await saved).ok()).toBe(true);
      await expect(toggle).toHaveAttribute("aria-checked", "true");
    }
    await page
      .getByRole("button", { name: "End Livestream", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    await expect(
      dialog.getByText(
        "If automatic VOD publishing is on, ending may publish a recording.",
      ),
    ).toBeVisible({ timeout: 3_000 });
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
  } finally {
    const dialog = page.getByRole("dialog", { name: "End announcement?" });
    if (await dialog.isVisible()) {
      await dialog.getByRole("button", { name: "Cancel" }).click();
    }
    if (!initial && (await toggle.getAttribute("aria-checked")) === "true") {
      const saved = page.waitForResponse((response) =>
        response.url().endsWith("/xrpc/com.atproto.repo.putRecord"),
      );
      await toggle.click();
      expect((await saved).ok()).toBe(true);
    }
  }
});
