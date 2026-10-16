import { expect, test } from "@playwright/test";

// place.stream.live.getClip, called the way a third-party app does: with a
// service-auth token the user's PDS mints for exactly this method (the rpc
// scope), not the node's own OAuth session. The clip of the harness's live
// stream must be a playable MP4. Backend-only for now: there is no app surface
// yet, so there is no mobile counterpart.
test("live-clip: a service-auth caller clips the live stream to a playable MP4", async ({
  page,
}) => {
  const pdsUrl = process.env.PDS_HTTPS_URL;
  test.skip(!pdsUrl, "needs the harness's HTTPS mode for the PDS");
  const did = process.env.ACCOUNT_DID!;

  await page.goto("/");
  await expect(page.getByTestId("home-stream-card").first()).toBeVisible({
    timeout: 30_000,
  });

  // Everything runs in the page: the browser is what trusts the harness's
  // certificates, and a browser is where a third-party web app would call from.
  const result = await page.evaluate(
    async ({ pdsUrl, handle, password, did }) => {
      const nodeDid = (await (await fetch("/.well-known/did.json")).json()).id;
      const session = await (
        await fetch(`${pdsUrl}/xrpc/com.atproto.server.createSession`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ identifier: handle, password }),
        })
      ).json();
      const lxm = "place.stream.live.getClip";
      const { token } = await (
        await fetch(
          `${pdsUrl}/xrpc/com.atproto.server.getServiceAuth?aud=${encodeURIComponent(nodeDid)}&lxm=${lxm}`,
          { headers: { Authorization: `Bearer ${session.accessJwt}` } },
        )
      ).json();

      const end = new Date();
      const start = new Date(end.getTime() - 20_000);
      const url = `/xrpc/${lxm}?streamer=${encodeURIComponent(did)}&start=${start.toISOString()}&end=${end.toISOString()}`;
      const anonymous = (await fetch(url)).status;
      const res = await fetch(url, {
        headers: { Authorization: `Bearer ${token}` },
      });
      const bytes = new Uint8Array(await res.arrayBuffer());
      const contentLength = res.headers.get("content-length");

      // A plain <video> plays it from memory.
      let duration = 0;
      if (res.ok) {
        const video = document.createElement("video");
        video.muted = true;
        video.src = URL.createObjectURL(
          new Blob([bytes], { type: "video/mp4" }),
        );
        duration = await new Promise<number>((resolve, reject) => {
          video.onloadedmetadata = () => resolve(video.duration);
          video.onerror = () =>
            reject(new Error(`video error ${video.error?.code}`));
        });
      }
      return {
        anonymous,
        status: res.status,
        contentType: res.headers.get("content-type"),
        contentLength,
        size: bytes.length,
        firstBox: new TextDecoder().decode(bytes.slice(4, 8)),
        duration,
      };
    },
    {
      pdsUrl: pdsUrl!,
      handle: process.env.ACCOUNT_HANDLE!,
      password: process.env.ACCOUNT_PASSWORD!,
      did,
    },
  );

  expect(result.anonymous).toBe(401);
  expect(result.status).toBe(200);
  expect(result.contentType).toBe("video/mp4");
  expect(result.contentLength).toBe(`${result.size}`);
  expect(result.firstBox).toBe("ftyp");
  expect(result.duration).toBeGreaterThan(0);
});
