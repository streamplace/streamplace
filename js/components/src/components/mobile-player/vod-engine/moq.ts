import type { MoqSource } from "./mse";

export type MoqFetcher = {
  // Group `group` of track `track`: one frame, the segment's bytes.
  fetch(
    track: string,
    group: number,
    signal: AbortSignal,
  ): Promise<Uint8Array<ArrayBuffer>>;
  close(): void;
};

// Connects to the node's MoQ listener and resolves the video's broadcast.
// The session, origin and request stay held until close: dropping them ends the broadcast.
export async function connectMoq(src: MoqSource): Promise<MoqFetcher> {
  const Moq = await import("@moq/net");
  const origin = new Moq.Origin.Producer();
  const session = await Moq.Connection.connect({
    url: new URL(src.url),
    consume: origin,
    websocket: { enabled: false },
    webtransport: src.fingerprint
      ? { serverCertificateHashes: [{ value: src.fingerprint }] }
      : undefined,
  });
  const request = origin.request(Moq.Path.from(src.broadcast));
  const close = () => {
    request.close();
    session.close();
    origin.close();
  };
  let broadcast = request.active.peek();
  try {
    const refused = Promise.resolve(request.closed).then((err) => {
      throw err ?? new Error("MoQ request closed");
    });
    refused.catch(() => {});
    while (!broadcast) {
      await Promise.race([request.active.changed(), refused]);
      broadcast = request.active.peek();
    }
  } catch (err) {
    close();
    throw err;
  }
  const active = broadcast;
  return {
    async fetch(track, group, signal) {
      const g = await active.track(track).fetchGroup(group, { signal });
      const abort = () => g.close();
      signal.addEventListener("abort", abort);
      try {
        const frame = await g.readFrame();
        if (!frame) throw new Error(`MoQ group ${group} of ${track} is empty`);
        // Read off the wire, so backed by a plain ArrayBuffer.
        return frame.payload as Uint8Array<ArrayBuffer>;
      } finally {
        signal.removeEventListener("abort", abort);
        g.close();
      }
    },
    close,
  };
}
