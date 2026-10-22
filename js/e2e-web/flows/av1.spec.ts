import { expect, test, type Page } from "@playwright/test";

// Under SP_E2E_VIDEO=av1 the harness streams test/fixtures/av1-h264-opus.mp4:
// an AV1 640x360 source track and an H.264 426x240 fallback track over fMP4
// ingest, the AV1 track alone over WHIP. Mobile: .maestro/logged-out/av1.yaml.
test.skip(
  process.env.SP_E2E_VIDEO !== "av1",
  "the harness isn't streaming AV1 (SP_E2E_VIDEO=av1)",
);
const HAS_FALLBACK = process.env.SP_E2E_INGEST === "fmp4";

// Keep the page's peer connections, to read what the playback one receives.
async function capturePeerConnections(page: Page) {
  await page.addInitScript(() => {
    const pcs: RTCPeerConnection[] = [];
    (window as any).__spPeerConnections = pcs;
    window.RTCPeerConnection = class extends window.RTCPeerConnection {
      constructor(config?: RTCConfiguration) {
        super(config);
        pcs.push(this);
      }
    };
  });
}

// The received video's codec and decoded frame count.
function inboundVideo(page: Page) {
  return page.evaluate(async () => {
    const pcs: RTCPeerConnection[] = (window as any).__spPeerConnections;
    for (const pc of pcs) {
      const byId = new Map<string, any>();
      (await pc.getStats()).forEach((s) => byId.set(s.id, s));
      for (const s of byId.values()) {
        if (s.type === "inbound-rtp" && s.kind === "video" && s.codecId) {
          return {
            mimeType: byId.get(s.codecId)?.mimeType as string | undefined,
            framesDecoded: (s.framesDecoded ?? 0) as number,
          };
        }
      }
    }
    return null;
  });
}

async function openStream(page: Page) {
  await page.goto("/");
  const card = page.getByTestId("home-stream-card").first();
  await expect(card).toBeVisible({ timeout: 30_000 });
  await card.click();
}

// The player's video shows frames, one of widths wide, and keeps going.
async function expectVideoPlays(page: Page, widths: number[]) {
  const video = page.locator("video").first();
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.videoWidth), {
      timeout: 30_000,
    })
    .toBeGreaterThan(0);
  expect(widths).toContain(
    await video.evaluate((v: HTMLVideoElement) => v.videoWidth),
  );
  const before = await video.evaluate((v: HTMLVideoElement) => v.currentTime);
  await expect
    .poll(() => video.evaluate((v: HTMLVideoElement) => v.currentTime))
    .toBeGreaterThan(before + 1);
}

async function expectWebRTCPlays(page: Page, mimeType: string, width: number) {
  await expect
    .poll(async () => (await inboundVideo(page))?.mimeType, {
      timeout: 30_000,
    })
    .toBe(mimeType);
  const { framesDecoded } = (await inboundVideo(page))!;
  await expect
    .poll(async () => (await inboundVideo(page))?.framesDecoded)
    .toBeGreaterThan(framesDecoded + 30);
  await expectVideoPlays(page, [width]);
}

test("av1: WebRTC plays the AV1 source", async ({ page }) => {
  await capturePeerConnections(page);
  await openStream(page);
  await expectWebRTCPlays(page, "video/AV1", 640);
});

test("av1: a viewer without AV1 gets the H.264 fallback over WebRTC", async ({
  page,
}) => {
  test.skip(!HAS_FALLBACK, "only the fMP4 push carries an H.264 track");
  await capturePeerConnections(page);
  // The playback connection offers every video codec but AV1.
  await page.addInitScript(() => {
    const addTransceiver = RTCPeerConnection.prototype.addTransceiver;
    RTCPeerConnection.prototype.addTransceiver = function (
      trackOrKind: MediaStreamTrack | string,
      init?: RTCRtpTransceiverInit,
    ) {
      const transceiver = addTransceiver.call(this, trackOrKind, init);
      if (trackOrKind === "video") {
        transceiver.setCodecPreferences(
          RTCRtpReceiver.getCapabilities("video")!.codecs.filter(
            (c) => c.mimeType !== "video/AV1",
          ),
        );
      }
      return transceiver;
    };
  });
  await openStream(page);
  await expectWebRTCPlays(page, "video/H264", 426);
});

test("av1: HLS lists every video track and plays", async ({ page }) => {
  const master = await (
    await page.request.get(
      `/xrpc/place.stream.playback.getLivePlaylist?streamer=${process.env.ACCOUNT_DID}`,
    )
  ).text();
  expect(master).toMatch(/CODECS="av01\.[^"]*",RESOLUTION=640x360/);
  if (HAS_FALLBACK) {
    expect(master).toMatch(/CODECS="avc1\.[^"]*",RESOLUTION=426x240/);
  }

  await page.addInitScript(() =>
    localStorage.setItem("player-protocol", "hls"),
  );
  await openStream(page);
  // hls.js picks a variant by bandwidth; either is a working playback.
  await expectVideoPlays(page, HAS_FALLBACK ? [640, 426] : [640]);
});
