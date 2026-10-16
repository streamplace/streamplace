import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";

// The harness runs a second node that hears the test stream's origin record
// on the firehose and pulls its segments from the first node over Media over
// QUIC (see pkg/cmd/e2e.go and docs/moq.md). Playback from that node is the
// whole transfer end to end: the origin advertising its MoQ URL, the peer
// dialling it with the pinned certificate, the MUXL segments arriving and
// validating, and the stream going live there.
//
// Web only: node-to-node transfer has no mobile surface.
test("syndication: stream plays from the node that pulls it over MoQ", async ({
  page,
}) => {
  const SERVER2_URL = process.env.SERVER2_URL;
  const did = process.env.ACCOUNT_DID;
  if (!SERVER2_URL || !did) {
    throw new Error(
      "SERVER2_URL / ACCOUNT_DID are not set — start the harness first",
    );
  }

  // The peer serves its own copy of the app at its own origin, so it has to
  // be pointed at itself like the first node was in global-setup.
  await pointAppAtNode(page, SERVER2_URL);

  // The stream is live on the peer once its first pulled segment validates.
  await expect
    .poll(
      async () => {
        const res = await fetch(
          `${SERVER2_URL}/xrpc/place.stream.live.getLiveUsers?limit=50`,
        );
        if (!res.ok) return 0;
        const body = (await res.json()) as { streams?: unknown[] };
        return body.streams?.length ?? 0;
      },
      { timeout: 90_000, intervals: [2_000] },
    )
    .toBeGreaterThan(0);

  await page.goto(`${SERVER2_URL}/${did}`);
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  const video = page.locator("video").first();
  await expect(video).toBeVisible();
  // Frames are actually arriving from the peer, not just a player mounted.
  await expect
    .poll(() => video.evaluate((v) => (v as HTMLVideoElement).currentTime), {
      timeout: 30_000,
    })
    .toBeGreaterThan(0);
});
