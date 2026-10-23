import { expect, test } from "@playwright/test";
import { pointAppAtNode } from "../server-setup";

// The harness runs a third node that syndicates nothing itself: the first
// node is configured to push its live streams to it (--peers, see
// pkg/cmd/e2e.go and docs/moq.md), the way a node beside a streamer's encoder
// hands a stream off to a server it cannot be reached from. Playback from
// that node is the push end to end: the pusher looking up the peer's MoQ URL,
// dialling it as a publisher, announcing the stream, the peer subscribing,
// and the MUXL segments validating and going live there.
//
// Web only: node-to-node transfer has no mobile surface.
test("push: stream plays from the node it is pushed to over MoQ", async ({
  page,
}) => {
  const SERVER3_URL = process.env.SERVER3_URL;
  const did = process.env.ACCOUNT_DID;
  if (!SERVER3_URL || !did) {
    throw new Error(
      "SERVER3_URL / ACCOUNT_DID are not set — start the harness first",
    );
  }

  await pointAppAtNode(page, SERVER3_URL);

  // The stream is live on the node once its first pushed segment validates.
  await expect
    .poll(
      async () => {
        const res = await fetch(
          `${SERVER3_URL}/xrpc/place.stream.live.getLiveUsers?limit=50`,
        );
        if (!res.ok) return 0;
        const body = (await res.json()) as { streams?: unknown[] };
        return body.streams?.length ?? 0;
      },
      { timeout: 90_000, intervals: [2_000] },
    )
    .toBeGreaterThan(0);

  await page.goto(`${SERVER3_URL}/${did}`);
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
  const video = page.locator("video").first();
  await expect(video).toBeVisible();
  // Frames are actually arriving over the push, not just a player mounted.
  await expect
    .poll(() => video.evaluate((v) => (v as HTMLVideoElement).currentTime), {
      timeout: 30_000,
    })
    .toBeGreaterThan(0);
});
