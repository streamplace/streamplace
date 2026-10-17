import { expect, test } from "@playwright/test";

// A livestream socket whose handshake fails before any message (node
// restarting, IP over the websocket limit) must be retried. Regression: the app
// read that failure as "this stream doesn't exist", stopped reconnecting, and
// surfaced it as a user_not_found problem on the streamer's dashboard.
//
// Web only: Maestro has no way to fail a native app's socket handshake.
test("stream: recovers when the first livestream socket fails", async ({
  page,
}) => {
  const did = process.env.ACCOUNT_DID;
  if (!did) {
    throw new Error("ACCOUNT_DID is not set — start the harness first");
  }

  await page.addInitScript(() => {
    const NativeWebSocket = window.WebSocket;
    window.WebSocket = class extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        const w = window as unknown as { failedLivestreamSocket?: boolean };
        if (!w.failedLivestreamSocket && `${url}`.includes("/api/websocket/")) {
          w.failedLivestreamSocket = true;
          // No such route: the server answers without upgrading.
          url = `${url}`.replace("/api/websocket/", "/api/no-websocket/");
        }
        super(url, protocols);
      }
    };
  });

  await page.goto(`/${did}`);

  await expect
    .poll(() =>
      page.evaluate(
        () =>
          (window as unknown as { failedLivestreamSocket?: boolean })
            .failedLivestreamSocket,
      ),
    )
    .toBe(true);
  await expect(
    page.getByText("Now streaming - e2e test stream").first(),
  ).toBeVisible({ timeout: 30_000 });
});
