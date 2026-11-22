import { expect, type Page, test } from "@playwright/test";
import { loginThroughPds } from "./login";

// The harness's second account streams with no livestream record (pre-live).
function preliveAccount() {
  const httpsUrl = process.env.SERVER_HTTPS_URL;
  const pdsUrl = process.env.PDS_HTTPS_URL;
  const did = process.env.PRELIVE_ACCOUNT_DID;
  const handle = process.env.PRELIVE_ACCOUNT_HANDLE;
  const password = process.env.PRELIVE_ACCOUNT_PASSWORD;
  test.skip(
    !httpsUrl || !pdsUrl || !did || !handle || !password,
    "needs the harness's HTTPS hostnames and its pre-live account",
  );
  return {
    httpsUrl: httpsUrl!,
    pdsUrl: pdsUrl!,
    did: did!,
    handle: handle!,
    password: password!,
  };
}

// The page's video is playing, not just holding a decoded frame.
async function expectPlaying(page: Page) {
  const video = page.locator("video").first();
  await expect
    .poll(
      () =>
        video.evaluate(
          (v: HTMLVideoElement) =>
            new Promise<boolean>((resolve) => {
              const start = v.currentTime;
              setTimeout(
                () =>
                  resolve(
                    !v.paused && v.readyState >= 2 && v.currentTime > start,
                  ),
                1500,
              );
            }),
        ),
      { timeout: 60_000 },
    )
    .toBe(true);
}

// Native counterpart: .maestro/logged-out/prelive-hidden.yaml. The harness's
// second account streams with no livestream record (pre-live): only its own
// client may play it, not anonymous viewers or a leaked sid replayed off-site.
test("prelive: only the streamer's own client can play an unpublished stream", async ({
  page,
  request,
}) => {
  test.setTimeout(180_000);
  const { httpsUrl, pdsUrl, did, handle, password } = preliveAccount();

  // The streamer's dashboard preview carries an owner sid in its playlist URL.
  await loginThroughPds(page, undefined, { handle, password });
  await page.evaluate(() => localStorage.setItem("player-protocol", "hls"));
  const sessionPlaylist = page.waitForRequest(
    (r) => /getLivePlaylist\?streamer=[^&]+&sid=/.test(r.url()),
    { timeout: 60_000 },
  );
  await page.goto(`${httpsUrl}/live`);
  const playlistUrl = (await sessionPlaylist).url();
  await expectPlaying(page);

  // The stream has ingested by now, yet nobody without a session sees it.
  const anonymous = await request.get(
    `/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(did)}`,
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

// Native counterpart: .maestro/logged-in/prelive-elsewhere.yaml. A pre-live
// stream plays to one owner session at a time; the newest one wins and the
// evicted player has to ask before it takes the stream back.
test("prelive: a newer session evicts the older, which only reclaims on request", async ({
  page,
  request,
}) => {
  test.setTimeout(240_000);
  const { httpsUrl, handle, password } = preliveAccount();
  await loginThroughPds(page, undefined, { handle, password });

  // Player A: HLS.
  await page.evaluate(() => localStorage.setItem("player-protocol", "hls"));
  const sessionPlaylist = page.waitForRequest(
    (r) => /getLivePlaylist\?streamer=[^&]+&sid=/.test(r.url()),
    { timeout: 60_000 },
  );
  await page.goto(`${httpsUrl}/live`);
  const staleUrl = new URL((await sessionPlaylist).url());
  await expectPlaying(page);

  // Player B (same account, same browser): the default protocol, WebRTC.
  await page.evaluate(() => localStorage.removeItem("player-protocol"));
  const pageB = await page.context().newPage();
  await pageB.goto(`${httpsUrl}/live`);
  await expectPlaying(pageB);

  const elsewhereA = page.getByTestId("playback-elsewhere");
  await expect(elsewhereA).toBeVisible({ timeout: 30_000 });
  await expect(pageB.getByTestId("playback-elsewhere")).toBeHidden();

  // The evicted session is refused outright, not just by the UI.
  const stale = await request.get(
    `${process.env.SERVER_URL}${staleUrl.pathname}${staleUrl.search}`,
  );
  expect(stale.status()).toBe(409);
  expect(await stale.text()).toContain("PlaybackStartedElsewhere");

  // Play here: A takes over, B is the evicted one now.
  await page.getByTestId("playback-elsewhere-play-here").click();
  await expect(elsewhereA).toBeHidden({ timeout: 30_000 });
  await expectPlaying(page);
  const elsewhereB = pageB.getByTestId("playback-elsewhere");
  await expect(elsewhereB).toBeVisible({ timeout: 30_000 });

  // Neither player takes it back unasked, however long it waits (the player
  // re-mints a session at most every 15s).
  await page.waitForTimeout(20_000);
  await expect(elsewhereB).toBeVisible();
  await expect(elsewhereA).toBeHidden();
  await expectPlaying(page);
});
