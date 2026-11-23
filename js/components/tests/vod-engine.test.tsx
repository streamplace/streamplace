import { afterEach, expect, it, vi } from "vitest";
import { VodEngine } from "../src/components/mobile-player/vod-engine/engine";
import {
  decodeTime,
  type Manifest,
  type Track,
} from "../src/components/mobile-player/vod-engine/mse";

const ascii = (s: string) => new TextEncoder().encode(s);
const text = (b: Uint8Array) => new TextDecoder().decode(b).trim();
function box(type: string, ...parts: Uint8Array[]) {
  const size = 8 + parts.reduce((n, p) => n + p.length, 0);
  const out = new Uint8Array(size);
  new DataView(out.buffer).setUint32(0, size);
  out.set(ascii(type), 4);
  parts.reduce((at, p) => (out.set(p, at), at + p.length), 8);
  return out;
}
function tfdt(ticks: number, version: 0 | 1) {
  const body = new Uint8Array(version ? 12 : 8);
  body[0] = version;
  const v = new DataView(body.buffer);
  if (version) v.setBigUint64(4, BigInt(ticks));
  else v.setUint32(4, ticks);
  return box("tfdt", body);
}
// A segment as the node stores it: a leading uuid box (here naming it), then moof.
function fragment(label: string, ticks: number) {
  const uuid = box("uuid", ascii(label.padEnd(16)));
  const moof = box("moof", box("traf", tfdt(ticks, 1)));
  const out = new Uint8Array(uuid.length + moof.length);
  out.set(uuid);
  out.set(moof, uuid.length);
  return out;
}

it("reads tfdt past uuid boxes, in both versions", () => {
  expect(decodeTime(fragment("x", 2 ** 40))).toBe(2 ** 40);
  const empty = new Uint8Array(8);
  const moof = box(
    "moof",
    box("mfhd", empty),
    box("traf", box("tfhd", empty), tfdt(90, 0)),
  );
  expect(decodeTime(moof)).toBe(90);
});

type Append = { label: string; at: number };

// Records appends by label and placement, and keeps `buffered` from them.
class FakeSourceBuffer extends EventTarget {
  timestampOffset = 0;
  updating = false;
  appends: Append[] = [];
  ranges: [number, number][] = [];
  get buffered() {
    const r = [...this.ranges].sort((a, b) => a[0] - b[0]);
    const merged: [number, number][] = [];
    for (const [s, e] of r) {
      const last = merged.at(-1);
      if (last && s <= last[1] + 1e-6) last[1] = Math.max(last[1], e);
      else merged.push([s, e]);
    }
    return {
      length: merged.length,
      start: (i: number) => merged[i][0],
      end: (i: number) => merged[i][1],
    };
  }
  private op(run: () => void) {
    this.updating = true;
    run();
    setTimeout(() => {
      this.updating = false;
      this.dispatchEvent(new Event("updateend"));
    });
  }
  appendBuffer(data: Uint8Array) {
    this.op(() => {
      if (text(data.subarray(4, 8)) !== "uuid") {
        this.appends.push({ label: text(data), at: NaN });
        return;
      }
      const at = this.timestampOffset + decodeTime(data) / 1000;
      this.appends.push({ label: text(data.subarray(8, 24)), at });
      this.ranges.push([at, at + 2]);
    });
  }
  remove(start: number, end: number) {
    this.op(() => {
      this.ranges = this.ranges.filter(([s, e]) => e <= start || s >= end);
    });
  }
  changeType() {}
}

// 2 s segments, the timeline jumping 10 s (a discontinuity) at segment 2.
const ticks = (i: number) => 5000 + i * 2000 + (i >= 2 ? 10000 : 0);
const track = (id: string, type: Track["type"], height?: number): Track => ({
  id,
  type,
  codec: "x",
  height,
  bandwidth: height ? height * 1e5 : 1e5,
  timescale: 1000,
  init: btoa(`init ${id}`),
  url: id,
  first: 0,
  offsets: Array.from({ length: 40 }, (_, i) => i * 100),
  sizes: Array(40).fill(100),
  durations: Array(40).fill(2000),
});

function setup() {
  const manifest: Manifest = {
    uri: "at://did:plc:a/place.stream.video/r",
    sid: "s",
    durationMs: 80_000,
    tracks: [
      track("v0", "video", 240),
      track("v1", "video", 720),
      track("a", "audio"),
    ],
    sources: [{ type: "http" }],
  };
  vi.stubGlobal("fetch", async (url: string, init: RequestInit) => {
    const range = (init.headers as Record<string, string>).Range;
    const i = Number(range.match(/bytes=(\d+)-/)![1]) / 100;
    return new Response(fragment(`${url}:${i}`, ticks(i)), { status: 206 });
  });
  const video = Object.assign(new EventTarget(), {
    currentTime: 0,
    removeAttribute() {},
    load() {},
  });
  const sb = { video: new FakeSourceBuffer(), audio: new FakeSourceBuffer() };
  const ms = { readyState: "open", endOfStream() {} };
  const engine = new VodEngine({
    video: video as unknown as HTMLVideoElement,
    mediaSource: ms as unknown as MediaSource,
    sourceBuffers: sb as unknown as {
      video: SourceBuffer;
      audio: SourceBuffer;
    },
    manifest,
    appended: { v0: 1 },
  });
  return { engine, video, sb };
}

async function until(done: () => boolean) {
  for (let i = 0; i < 200 && !done(); i++)
    await new Promise((r) => setTimeout(r, 10));
  expect(done()).toBe(true);
}

let engine: VodEngine | undefined;
afterEach(() => {
  engine?.destroy();
  vi.unstubAllGlobals();
});

it("appends segments at their manifest start, 30 s ahead, after the boot's", async () => {
  const s = setup();
  engine = s.engine;
  await until(
    () => s.sb.video.appends.length === 15 && s.sb.audio.appends.length === 16,
  );
  expect(s.sb.video.appends.map((a) => a.label)).toEqual(
    Array.from({ length: 15 }, (_, i) => `v0:${i + 1}`),
  );
  s.sb.video.appends.forEach((a, i) => expect(a.at).toBeCloseTo((i + 1) * 2));
  s.sb.audio.appends.forEach((a, i) => expect(a.at).toBeCloseTo(i * 2));
});

it("refetches from the segment a seek lands in", async () => {
  const s = setup();
  engine = s.engine;
  await until(() => s.sb.video.appends.length === 15);
  s.sb.video.appends = [];
  s.video.currentTime = 61;
  s.video.dispatchEvent(new Event("seeking"));
  await until(() => s.sb.video.appends.length === 10);
  expect(s.sb.video.appends[0]).toMatchObject({ label: "v0:30" });
  expect(s.sb.video.appends[0].at).toBeCloseTo(60);
  expect(s.sb.video.buffered.start(0)).toBeGreaterThanOrEqual(30);
});

it("switches a pinned level after the playing segment, init first", async () => {
  const s = setup();
  engine = s.engine;
  await until(() => s.sb.video.appends.length === 15);
  const switched = vi.fn();
  engine.addEventListener("level", switched);
  s.sb.video.appends = [];
  s.video.currentTime = 3;
  engine.setLevel(1);
  await until(() => s.sb.video.appends.length >= 2);
  expect(s.sb.video.appends.slice(0, 2)).toMatchObject([
    { label: "init v1" },
    { label: "v1:2" },
  ]);
  expect(s.sb.video.appends[1].at).toBeCloseTo(4);
  expect(engine.level).toBe(1);
  expect(switched).toHaveBeenCalled();
});
