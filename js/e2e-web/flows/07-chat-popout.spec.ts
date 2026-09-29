import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// The chat popout button opens chat in its own sized browser window. It used
// to do that *and* navigate the tab it was clicked from (the Button rendered an
// <a href> wrapping the window.open handler), so one click both replaced the
// stream you were watching with the popout and spawned the popout window. It
// must open exactly one window and leave the stream page where it was.
//
// Log in first (chat's input row, which hosts the button, only renders for a
// logged-in user), the same way 05-oauth-login does.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

test("07-chat-popout: opens one window, leaves the stream page alone", async ({
  page,
}) => {
  await loginThroughPds(page);

  await page.goto(`${HTTPS_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  // Wait for the stream context the button needs (the streamer's profile and
  // the livestream record) the way 04-stream does, so the click can resolve the
  // streamer's DID rather than racing the stream's first websocket message.
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  await expect(page.locator("video").first()).toBeVisible();

  const popoutButton = page.getByTestId("chat-popout-button");
  await expect(popoutButton).toBeVisible({ timeout: 30_000 });

  const streamUrl = page.url();
  const popupPromise = page.waitForEvent("popup", { timeout: 20_000 });
  await popoutButton.click();
  const popup = await popupPromise;

  await popup.waitForLoadState("domcontentloaded");
  const popupPath = new URL(popup.url()).pathname;
  expect(popupPath).toMatch(/^\/chat-popout\/did:plc:/);
  await expect(popup.locator("body")).toBeVisible();

  // Regression: the click must not also navigate the page it was made from.
  await expect(page).toHaveURL(streamUrl, { timeout: 5_000 });

  await popup.close();
  expect(page.context().pages()).toHaveLength(1);
});
