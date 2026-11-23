import { fetchRange } from "./http";
import { connectMoq, type MoqFetcher } from "./moq";
import {
  appendAt,
  type BootState,
  bytes,
  type Manifest,
  mediaSourceClass,
  mime,
  openMediaSource,
  playable,
  type Track,
  update,
} from "./mse";

// The boot page's engine, and the boot script's pending handover of it. Cast
// rather than augment Window: the app type-checks this file under two paths.
export type VodGlobals = {
  __spVod?: VodEngine;
  __spVodBoot?: Promise<VodEngine | undefined>;
};
const globals = () => window as Window & VodGlobals;

const AHEAD = 30; // seconds buffered ahead of playback
const BEHIND = 30; // seconds kept behind it
const EPS = 0.1; // seconds of slack at buffered range ends

const delay = (ms: number) => new Promise((r) => setTimeout(r, ms));

// One SourceBuffer and the segments it is fed, in a track's own numbering.
type Lane = {
  sb: SourceBuffer;
  tracks: Track[]; // the renditions it can switch between
  track: Track; // the track `next` indexes
  init: Track; // the track whose init segment `sb` last took
  next: number; // the next segment to append
  abort: AbortController; // the in-flight fetch, cancelled by a seek
};

// Plays a VOD manifest on MediaSource (docs/vod-instant-start.md). Dispatches
// "level" when the appended video track changes and "error" on a fatal error.
export class VodEngine extends EventTarget {
  readonly video: HTMLVideoElement;
  readonly manifest: Manifest;
  readonly levels: { name: string }[];
  level = 0;
  error: Error | null = null;
  transport: "http" | "moq" = "http";
  private ms: MediaSource;
  private lanes: Lane[];
  private starts = new Map<Track, number[]>();
  private pinned = -1;
  private throughput = 0; // bits/s, EWMA over video segment downloads
  private moq?: MoqFetcher;
  private closed = false;
  private waiters = new Set<() => void>();
  private listeners = new AbortController();

  // A fresh engine on `video` for a manifest, or the URL of one.
  static async start(
    src: Manifest | string,
    video: HTMLVideoElement,
    opts: { startTime?: number } = {},
  ): Promise<VodEngine> {
    let manifest: Manifest;
    if (typeof src === "string") {
      const res = await fetch(src);
      if (!res.ok) throw new Error(`VOD manifest: HTTP ${res.status}`);
      manifest = await res.json();
    } else {
      manifest = src;
    }
    const {
      video: [v],
      audio: a,
    } = playable(manifest);
    const tracks = a ? [v, a] : [v];
    const MS = mediaSourceClass();
    if (!MS || !v || !tracks.every((t) => MS.isTypeSupported(mime(t)))) {
      throw new Error("VOD tracks are not playable with MediaSource");
    }
    const ms = await openMediaSource(MS, video);
    ms.duration = manifest.durationMs / 1000;
    const sourceBuffers = {
      video: ms.addSourceBuffer(mime(v)),
      audio: a && ms.addSourceBuffer(mime(a)),
    };
    for (const t of tracks) {
      const sb = sourceBuffers[t.type]!;
      await update(sb, () => sb.appendBuffer(bytes(t.init)));
    }
    const startTime = Math.min(
      opts.startTime ?? 0,
      Math.max(0, manifest.durationMs / 1000 - 0.1),
    );
    if (startTime > 0) video.currentTime = startTime;
    return new VodEngine(
      { video, mediaSource: ms, sourceBuffers, manifest, appended: {} },
      startTime,
    );
  }

  // Takes over a MediaSource whose SourceBuffers hold the first tracks' inits.
  constructor(s: BootState, startTime = 0) {
    super();
    this.video = s.video;
    this.ms = s.mediaSource;
    this.manifest = s.manifest;
    const { video, audio } = playable(s.manifest);
    this.levels = video.map((t) => ({ name: `${t.height}p` }));
    for (const t of s.manifest.tracks) {
      let ticks = 0;
      const ends = t.durations.map((d) => (ticks += d) / t.timescale);
      this.starts.set(t, [0, ...ends]);
    }
    const lane = (sb: SourceBuffer, tracks: Track[]): Lane => ({
      sb,
      tracks,
      track: tracks[0],
      init: tracks[0],
      next: Math.max(
        s.appended[tracks[0].id] ?? 0,
        this.segmentAt(tracks[0], startTime),
      ),
      abort: new AbortController(),
    });
    this.lanes = [lane(s.sourceBuffers.video, video)];
    if (audio && s.sourceBuffers.audio) {
      this.lanes.push(lane(s.sourceBuffers.audio, [audio]));
    }

    const signal = this.listeners.signal;
    this.video.addEventListener("seeking", () => this.seek(), { signal });
    this.video.addEventListener("timeupdate", () => this.wake(), { signal });
    // The first source this browser can use; HTTP until MoQ connects.
    const source = s.manifest.sources.find(
      (x) => x.type === "http" || typeof WebTransport === "function",
    );
    if (source?.type === "moq") {
      connectMoq(source).then(
        (moq) => {
          if (this.closed) return moq.close();
          this.moq = moq;
          this.transport = "moq";
        },
        (err) => console.warn("VOD: MoQ unavailable, staying on HTTP", err),
      );
    }
    for (const l of this.lanes) this.run(l).catch((err) => this.fail(err));
  }

  // Pins a video level (an index into `levels`), or -1 for automatic.
  setLevel(index: number) {
    this.pinned = index;
    const lane = this.lanes[0];
    const track = lane.tracks[index];
    if (track && track !== lane.track) {
      // Replace what is buffered after the playing segment, not after the buffer.
      const queued = this.starts.get(lane.track)![lane.next];
      lane.next = Math.min(
        this.segmentAt(track, this.video.currentTime) + 1,
        this.segmentAt(track, queued),
      );
      lane.track = track;
      lane.abort.abort();
      lane.abort = new AbortController();
    }
    this.wake();
  }

  destroy() {
    if (this.closed) return;
    this.closed = true;
    this.listeners.abort();
    for (const l of this.lanes) l.abort.abort();
    this.wake();
    this.moq?.close();
    this.video.removeAttribute("src");
    this.video.load();
    if (globals().__spVod === this) globals().__spVod = undefined;
  }

  private async run(lane: Lane) {
    while (!this.closed && !this.error) {
      const ahead = this.video.currentTime + AHEAD;
      if (
        lane.next >= lane.track.sizes.length ||
        this.starts.get(lane.track)![lane.next] > ahead
      ) {
        this.endIfDone();
        await this.sleep();
        continue;
      }
      if (lane.tracks.length > 1) this.adapt(lane);
      const { track, next, abort } = lane;
      const data = await this.fetchSegment(track, next, abort.signal).catch(
        (err) => {
          if (abort.signal.aborted) return null;
          throw err;
        },
      );
      if (!data || abort.signal.aborted) continue;
      await this.append(lane, track, next, data);
      if (abort.signal.aborted) continue;
      lane.next = next + 1;
      const level = lane.tracks.indexOf(track);
      if (lane === this.lanes[0] && level !== this.level) {
        this.level = level;
        this.dispatchEvent(new Event("level"));
      }
    }
  }

  // At a segment boundary: the pinned level, else up while throughput beats
  // 1.4x the next level's bandwidth and down while it is under the current one's.
  private adapt(lane: Lane) {
    const { tracks } = lane;
    let i = tracks.indexOf(lane.track);
    if (this.pinned >= 0) {
      i = Math.min(this.pinned, tracks.length - 1);
    } else if (this.throughput) {
      while (
        i + 1 < tracks.length &&
        this.throughput > 1.4 * tracks[i + 1].bandwidth
      ) {
        i++;
      }
      while (i > 0 && this.throughput < tracks[i].bandwidth) i--;
    }
    if (tracks[i] === lane.track) return;
    const at = this.starts.get(lane.track)![lane.next];
    lane.next = this.segmentAt(tracks[i], at);
    lane.track = tracks[i];
  }

  // The number of `track`'s segments that end by `t`: the one playing at `t`.
  private segmentAt(track: Track, t: number) {
    const starts = this.starts.get(track)!;
    let i = 0;
    while (i < track.sizes.length && starts[i + 1] <= t + 1e-3) i++;
    return i;
  }

  private seek() {
    const t = this.video.currentTime;
    for (const lane of this.lanes) {
      let next = this.segmentAt(lane.track, t);
      const b = lane.sb.buffered;
      for (let r = 0; r < b.length; r++) {
        if (b.start(r) <= t + EPS && t < b.end(r)) {
          next = Math.max(next, this.segmentAt(lane.track, b.end(r) - EPS) + 1);
        }
      }
      if (next === lane.next) continue;
      lane.abort.abort();
      lane.abort = new AbortController();
      lane.next = next;
    }
    this.wake();
  }

  private async fetchSegment(track: Track, i: number, signal: AbortSignal) {
    for (let attempt = 1; ; attempt++) {
      const t0 = performance.now();
      try {
        const data = await this.download(track, i, signal);
        if (track.type === "video") {
          const ms = Math.max(performance.now() - t0, 1);
          const bps = (data.byteLength * 8000) / ms;
          this.throughput = this.throughput
            ? 0.7 * this.throughput + 0.3 * bps
            : bps;
        }
        return data;
      } catch (err) {
        if (signal.aborted || attempt === 3) throw err;
        console.warn(`VOD: segment ${i} of track ${track.id}, retrying`, err);
        await delay(1000 * attempt);
      }
    }
  }

  private async download(track: Track, i: number, signal: AbortSignal) {
    const moq = this.moq;
    if (moq) {
      try {
        return await moq.fetch(track.id, track.first + i, signal);
      } catch (err) {
        if (signal.aborted) throw err;
        console.warn("VOD: MoQ fetch failed, staying on HTTP", err);
        moq.close();
        if (this.moq === moq) this.moq = undefined;
        this.transport = "http";
      }
    }
    return fetchRange(track.url, track.offsets[i], track.sizes[i], signal);
  }

  private async append(
    lane: Lane,
    track: Track,
    i: number,
    data: Uint8Array<ArrayBuffer>,
  ) {
    const { sb } = lane;
    if (lane.init !== track) {
      if (lane.init.codec !== track.codec) sb.changeType(mime(track));
      await update(sb, () => sb.appendBuffer(bytes(track.init)));
      lane.init = track;
    }
    for (let tries = 0; ; tries++) {
      const t = this.video.currentTime;
      // Once the browser's buffer is full, keep only what playback needs next.
      if (tries) await this.evict(sb, t - 2, t + AHEAD);
      else await this.evict(sb, t - BEHIND, t + 2 * AHEAD);
      try {
        const start = this.starts.get(track)![i];
        return await appendAt(sb, data, start, track.timescale);
      } catch (err) {
        if (err instanceof DOMException && err.name === "QuotaExceededError") {
          if (tries < 10) {
            await delay(1000);
            continue;
          }
        }
        throw err;
      }
    }
  }

  // Removes what is buffered outside [from, to].
  private async evict(sb: SourceBuffer, from: number, to: number) {
    const b = sb.buffered;
    if (!b.length) return;
    if (b.start(0) < from - 1) await update(sb, () => sb.remove(0, from));
    if (b.end(b.length - 1) > to + 1) {
      await update(sb, () => sb.remove(to, Infinity));
    }
  }

  private endIfDone() {
    const done = this.lanes.every(
      (l) => l.next >= l.track.sizes.length && !l.sb.updating,
    );
    if (done && this.ms.readyState === "open") this.ms.endOfStream();
  }

  // Resolves on the next timeupdate, seek or level change, or after a second.
  private sleep() {
    return new Promise<void>((resolve) => {
      const done = () => {
        clearTimeout(timer);
        this.waiters.delete(done);
        resolve();
      };
      const timer = setTimeout(done, 1000);
      this.waiters.add(done);
    });
  }

  private wake() {
    for (const w of [...this.waiters]) w();
  }

  private fail(err: unknown) {
    if (this.closed || this.error) return;
    this.error = err instanceof Error ? err : new Error(String(err));
    console.error("VOD engine:", err);
    this.dispatchEvent(new Event("error"));
  }
}

// Called by the boot script, once its first frame is on screen.
export function adoptBoot(state: BootState) {
  return (globals().__spVod = new VodEngine(state));
}
