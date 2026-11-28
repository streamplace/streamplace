import * as Moq from "@moq/net";

// Loaded after the page's inline first group: fetches the following groups
// over MoQ and switches up to the best rendition at the first keyframe it can.

type Frame = { ts: number; data: Uint8Array };
interface Engine {
  manifest: {
    moq: { url: string; fingerprint: string; path: string };
    gopUs: number;
    renditions: { name: string; groups: number }[];
  };
  transport?: WebTransport;
  seq: number;
  moq: string;
  push(rendition: number, seq: number, frames: Frame[]): void;
  mediaUs(): number;
}
declare global {
  interface Window {
    SP: Engine;
  }
}

const SP = window.SP;
const M = SP.manifest;
// Fetch ahead of playback by this much.
const AHEAD_US = 2 * M.gopUs;

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

// A group's frames, or null if the fetch failed or was abandoned.
async function readGroup(
  track: Moq.Track.Consumer,
  seq: number,
  signal: AbortSignal,
): Promise<Frame[] | null> {
  try {
    const group = await track.fetchGroup(seq, { signal });
    const frames: Frame[] = [];
    for (let f = await group.readFrame(); f; f = await group.readFrame()) {
      frames.push({ ts: f.timestamp.asMicros(), data: f.payload });
    }
    return frames;
  } catch (err) {
    if (!signal.aborted) console.warn(`group ${seq}:`, err);
    return null;
  }
}

async function connect(origin: Moq.Origin.Producer) {
  const props = {
    url: new URL(M.moq.url),
    consume: origin,
    websocket: { enabled: false },
    webtransport: { serverCertificateHashes: [{ value: M.moq.fingerprint }] },
  };
  try {
    // connect reads the negotiated protocol without waiting for ready; unready, it reads "".
    await SP.transport?.ready;
    return await Moq.Connection.connect({ ...props, transport: SP.transport });
  } catch {
    // The page's early session failed; open a fresh one.
    return await Moq.Connection.connect(props);
  }
}

async function play() {
  const origin = new Moq.Origin.Producer();
  await connect(origin);
  const request = origin.request(Moq.Path.from(M.moq.path));
  let broadcast = request.active.peek();
  while (!broadcast) {
    await request.active.changed();
    broadcast = request.active.peek();
  }
  const tracks = M.renditions.map((r) => broadcast.track(r.name));
  SP.moq = "connected";

  const starter = 0;
  const best = M.renditions.length - 1;
  let upgraded = false;
  for (let seq = SP.seq + 1; seq < M.renditions[0].groups; seq++) {
    while (seq * M.gopUs - SP.mediaUs() > AHEAD_US) await sleep(50);
    const abort = new AbortController();
    const hd = readGroup(tracks[best], seq, abort.signal);
    // Until the best rendition arrives in time, race it against playback
    // reaching this group, with the starter's copy as the fallback.
    const low = upgraded ? null : readGroup(tracks[starter], seq, abort.signal);
    const deadlineMs = (seq * M.gopUs - SP.mediaUs()) / 1000 - 100;
    const won = upgraded
      ? await hd
      : await Promise.race([hd, sleep(deadlineMs).then(() => null)]);
    if (won) {
      upgraded = true;
      SP.push(best, seq, won);
    } else {
      const frames =
        (await low) ?? (await readGroup(tracks[starter], seq, abort.signal));
      if (!frames) throw new Error(`group ${seq} failed`);
      SP.push(starter, seq, frames);
    }
    abort.abort();
  }
  SP.moq = "done";
}

play().catch((err) => {
  SP.moq = `failed: ${err}`;
  console.error(err);
});
