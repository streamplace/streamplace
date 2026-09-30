import { expect, test, type Locator } from "@playwright/test";
import { loginThroughPds } from "./login";

// Chat is an inverted list, which the web flips with scaleY(-1) and scrolls by
// hand on each wheel event. A trackpad sends many small, fractional deltas,
// and the browser snaps each scrollTop write to a whole pixel: the list used
// to lose every half-pixel step toward the latest message, so a two-finger
// scroll back down stalled while scrolling up into history worked. Both
// directions must travel the full distance the wheel asked for.
//
// Log in first so this flow can fill the chat with enough messages to scroll.
const HTTPS_URL = process.env.SERVER_HTTPS_URL;

test.skip(!HTTPS_URL, "harness started without its HTTPS hostnames");

const MESSAGES = 30;
const STEPS = 40;
const STEP_PX = 0.5;

const scrollTop = (list: Locator) => list.evaluate((el) => el.scrollTop);

test("09-chat-wheel-scroll: small wheel deltas scroll chat both ways", async ({
  page,
}) => {
  test.setTimeout(180_000);
  await loginThroughPds(page);

  await page.goto(`${HTTPS_URL}/`);
  await page.getByTestId("home-stream-card").first().click();
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });

  const chatInput = page.getByPlaceholder("Type a message...");
  const run = Date.now();
  // a send the page wasn't ready for clears the box and is dropped; resend
  await expect(async () => {
    await chatInput.fill(`wheel scroll ${run} 0`);
    await chatInput.press("Enter");
    await expect(page.getByText(`wheel scroll ${run} 0`).first()).toBeVisible({
      timeout: 10_000,
    });
  }).toPass({ timeout: 45_000 });
  for (let i = 1; i < MESSAGES; i++) {
    await chatInput.fill(`wheel scroll ${run} ${i}`);
    await chatInput.press("Enter");
    await expect(
      page.getByText(`wheel scroll ${run} ${i}`, { exact: true }).first(),
    ).toBeVisible({ timeout: 15_000 });
  }

  const list = page.getByTestId("chat-list").first();
  await expect
    .poll(() => list.evaluate((el) => el.scrollHeight - el.clientHeight))
    .toBeGreaterThan(200);

  const box = await list.boundingBox();
  expect(box).not.toBeNull();
  await page.mouse.move(box!.x + box!.width / 2, box!.y + box!.height / 2);

  // Into history with a mouse-wheel-sized step. In an inverted list the
  // latest message sits at scrollTop 0 and history is further up.
  await page.mouse.wheel(0, -150);
  await expect.poll(() => scrollTop(list)).toBeGreaterThan(100);
  const inHistory = await scrollTop(list);

  // Back down toward the latest message in trackpad-sized steps.
  for (let i = 0; i < STEPS; i++) {
    await page.mouse.wheel(0, STEP_PX);
  }
  const travel = STEPS * STEP_PX;
  await expect
    .poll(async () => inHistory - (await scrollTop(list)))
    .toBeGreaterThanOrEqual(travel - 1);
  const afterDown = await scrollTop(list);
  expect(inHistory - afterDown).toBeLessThanOrEqual(travel + 1);

  // And up again by the same amount.
  for (let i = 0; i < STEPS; i++) {
    await page.mouse.wheel(0, -STEP_PX);
  }
  await expect
    .poll(async () => (await scrollTop(list)) - afterDown)
    .toBeGreaterThanOrEqual(travel - 1);
  expect((await scrollTop(list)) - afterDown).toBeLessThanOrEqual(travel + 1);
});
