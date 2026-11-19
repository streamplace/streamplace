import { expect, test } from "@playwright/test";

// The VOD proof of concept in js/vod-poc, served by hack/vod-poc-local.sh: the
// page paints its first frame from its own bytes, then switches up over MoQ.
const URLS = process.env.VOD_POC_URLS?.split(",");

type Engine = {
  firstFrame?: number;
  bestAt?: number;
  height?: number;
  mediaUs(): number;
};
declare global {
  interface Window {
    SP: Engine;
  }
}

test.skip(!URLS, "VOD_POC_URLS is not set: run hack/vod-poc-local.sh");

test("vod-poc: the first frame needs no request but the page", async ({
  page,
}) => {
  await page.route("**/*", (route) =>
    route.request().resourceType() === "document"
      ? route.continue()
      : route.abort(),
  );
  await page.goto(URLS![0]);
  await page.waitForFunction(() => window.SP.firstFrame !== undefined);
  expect(await page.evaluate(() => window.SP.height)).toBe(240);
});

test("vod-poc: playback switches up to 1080p and keeps going", async ({
  page,
}) => {
  await page.goto(URLS![0]);
  await expect
    .poll(() => page.evaluate(() => window.SP.height), { timeout: 30_000 })
    .toBe(1080);
  await expect
    .poll(() => page.evaluate(() => window.SP.mediaUs()), { timeout: 30_000 })
    .toBeGreaterThan(5_000_000);
  const [first, best] = await page.evaluate(() => [
    window.SP.firstFrame,
    window.SP.bestAt,
  ]);
  console.log(`vod-poc: first frame at ${first} ms, 1080p at ${best} ms`);
});
