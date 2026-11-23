// Shared by the inline boot script and the engine; see docs/vod-instant-start.md.

export type Track = {
  id: string;
  type: "video" | "audio";
  codec: string;
  width?: number;
  height?: number;
  bandwidth: number;
  timescale: number;
  init: string;
  url: string;
  first: number;
  offsets: number[];
  sizes: number[];
  durations: number[];
};

export type MoqSource = {
  type: "moq";
  url: string;
  fingerprint?: string;
  broadcast: string;
};

export type Manifest = {
  uri: string;
  sid: string;
  durationMs: number;
  tracks: Track[];
  sources: (MoqSource | { type: "http" })[];
};

export type SourceBuffers = { video: SourceBuffer; audio?: SourceBuffer };

// What the boot script hands the engine once its first frame is on screen.
export type BootState = {
  video: HTMLVideoElement;
  mediaSource: MediaSource;
  sourceBuffers: SourceBuffers;
  manifest: Manifest;
  // Leading segments fully appended, by track id.
  appended: Record<string, number>;
};

// Every video track (lowest bandwidth first) and the default audio track.
export function playable(m: Manifest) {
  return {
    video: m.tracks.filter((t) => t.type === "video"),
    audio: m.tracks.find((t) => t.type === "audio"),
  };
}

// MediaSource where the browser has it; iOS Safari has only ManagedMediaSource.
export function mediaSourceClass(): typeof MediaSource | undefined {
  return (
    window.MediaSource ??
    (window as { ManagedMediaSource?: typeof MediaSource }).ManagedMediaSource
  );
}

// Attaches a new MediaSource to `video` and resolves once it is open.
export async function openMediaSource(
  MS: typeof MediaSource,
  video: HTMLVideoElement,
): Promise<MediaSource> {
  const ms = new MS();
  // ManagedMediaSource plays only with remote playback off (or an AirPlay source).
  if (MS !== window.MediaSource) video.disableRemotePlayback = true;
  video.src = URL.createObjectURL(ms);
  await new Promise((r) =>
    ms.addEventListener("sourceopen", r, { once: true }),
  );
  URL.revokeObjectURL(video.src);
  return ms;
}

export const mime = (t: Track) => `${t.type}/mp4; codecs="${t.codec}"`;

export const bytes = (b64: string) =>
  Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));

// Runs one SourceBuffer operation and resolves when it ends.
export function update(sb: SourceBuffer, run: () => void): Promise<void> {
  return new Promise((resolve, reject) => {
    const ac = new AbortController();
    const done = (err?: unknown) => {
      ac.abort();
      if (err) reject(err);
      else resolve();
    };
    sb.addEventListener("updateend", () => done(), { signal: ac.signal });
    sb.addEventListener("error", () => done(new Error("SourceBuffer error")), {
      signal: ac.signal,
    });
    try {
      run();
    } catch (err) {
      done(err);
    }
  });
}

// Appends a media fragment so its first sample lands at `start` seconds.
export async function appendAt(
  sb: SourceBuffer,
  data: Uint8Array<ArrayBuffer>,
  start: number,
  timescale: number,
) {
  const offset = start - decodeTime(data) / timescale;
  if (Math.abs(sb.timestampOffset - offset) > 1e-6) sb.timestampOffset = offset;
  await update(sb, () => sb.appendBuffer(data));
}

// The first fragment's baseMediaDecodeTime (moof/traf/tfdt), in timescale ticks.
export function decodeTime(b: Uint8Array): number {
  const v = new DataView(b.buffer, b.byteOffset, b.byteLength);
  const walk = (p: number, end: number): number => {
    while (p + 8 <= end) {
      const size = v.getUint32(p) || end - p;
      const type = String.fromCharCode(b[p + 4], b[p + 5], b[p + 6], b[p + 7]);
      if (type === "tfdt") {
        return b[p + 8] ? Number(v.getBigUint64(p + 12)) : v.getUint32(p + 12);
      }
      if (type === "moof" || type === "traf") return walk(p + 8, p + size);
      if (size < 8) break;
      p += size;
    }
    throw new Error("fragment has no tfdt");
  };
  return walk(0, b.length);
}
