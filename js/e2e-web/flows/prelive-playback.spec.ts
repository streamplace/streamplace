import { expect, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// Native counterpart: .maestro/logged-out/prelive-hidden.yaml. The harness's
// second account streams with no livestream record (pre-live): only its own
// client may play it, not anonymous viewers or a leaked sid replayed off-site.
test("prelive: only the streamer's own client can play an unpublished stream", async ({
  page,
  request,
}) => {
  test.setTimeout(180_000);
  const httpsUrl = process.env.SERVER_HTTPS_URL;
  const pdsUrl = process.env.PDS_HTTPS_URL;
  const did = process.env.PRELIVE_ACCOUNT_DID;
  const handle = process.env.PRELIVE_ACCOUNT_HANDLE;
  const password = process.env.PRELIVE_ACCOUNT_PASSWORD;
  test.skip(
    !httpsUrl || !pdsUrl || !did || !handle || !password,
    "needs the harness's HTTPS hostnames and its pre-live account",
  );

  // The streamer's dashboard preview carries an owner sid in its playlist URL.
  await loginThroughPds(page, undefined, {
    handle: handle!,
    password: password!,
  });
  await page.evaluate(() => localStorage.setItem("player-protocol", "hls"));
  const sessionPlaylist = page.waitForRequest(
    (r) => /getLivePlaylist\?streamer=[^&]+&sid=/.test(r.url()),
    { timeout: 60_000 },
  );
  await page.goto(`${httpsUrl}/live`);
  const playlistUrl = (await sessionPlaylist).url();
  await expect
    .poll(
      () =>
        page
          .locator("video")
          .first()
          .evaluate(
            (v: HTMLVideoElement) =>
              !v.paused && v.readyState >= 2 && v.currentTime > 0,
          ),
      { timeout: 60_000 },
    )
    .toBe(true);

  // The stream has ingested by now, yet nobody without a session sees it.
  const anonymous = await request.get(
    `/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(did!)}`,
  );
  expect(anonymous.status()).toBe(404);

  // A pirate's page on a foreign origin (the PDS's host) replays the leaked
  // URL; same machine, so same client IP as the owner.
  await page.goto(`${pdsUrl}/xrpc/_health`);
  const foreign = await page.evaluate(async (url) => {
    const res = await fetch(url);
    return { status: res.status, body: await res.text() };
  }, playlistUrl);
  expect(foreign.status).toBe(404);
  expect(foreign.body).toContain("StreamNotLive");

  // Same sid without Origin/Referer (a native player) plays, so the 404 came
  // from the origin check. Plain HTTP: the HTTPS front pipes from loopback.
  const url = new URL(playlistUrl);
  const native = await request.get(
    `${process.env.SERVER_URL}${url.pathname}${url.search}`,
  );
  expect(native.status()).toBe(200);
  expect(await native.text()).toContain("#EXTM3U");
});
