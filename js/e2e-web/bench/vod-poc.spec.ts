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

// A cached player can run before the page's early WebTransport session is
// ready, as on any real network; handing it over then stalled MoQ for 10 s.
test("vod-poc: a slow early handshake still switches up promptly", async ({
  page,
}) => {
  // A 500 ms handshake: no negotiated protocol until it is ready.
  await page.addInitScript(() => {
    const Native = window.WebTransport;
    const protocol = Object.getOwnPropertyDescriptor(
      Native.prototype,
      "protocol",
    )!;
    window.WebTransport = function (
      url: string,
      options?: WebTransportOptions,
    ) {
      const transport = new Native(url, options);
      let done = false;
      const ready = transport.ready
        .then(() => new Promise((r) => setTimeout(r, 500)))
        .then(() => {
          done = true;
        });
      Object.defineProperty(transport, "ready", { get: () => ready });
      Object.defineProperty(transport, "protocol", {
        get: () => (done ? protocol.get!.call(transport) : ""),
      });
      return transport;
    } as unknown as typeof WebTransport;
  });
  await page.goto(URLS![0]);
  await page.waitForFunction(() => window.SP.bestAt !== undefined, null, {
    timeout: 15_000,
  });
  expect(await page.evaluate(() => window.SP.bestAt)).toBeLessThan(4_000);
});

// The MoQ library reclaims groups nobody has read for 30 s, on sweeps every
// 15 s that date reads late: up to 60 s idle. A fetch after that must still play.
test("vod-poc: groups still play after the server's idle window", async ({
  page,
}) => {
  test.setTimeout(120_000);
  const playsOverMoq = async () => {
    await page.goto(URLS![1]);
    await expect
      .poll(() => page.evaluate(() => window.SP.mediaUs()), { timeout: 15_000 })
      .toBeGreaterThan(2_500_000);
  };
  await playsOverMoq();
  await page.waitForTimeout(65_000);
  await playsOverMoq();
});
