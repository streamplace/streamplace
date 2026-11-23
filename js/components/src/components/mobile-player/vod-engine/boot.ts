import type { VodEngine, VodGlobals } from "./engine";
import {
  appendAt,
  bytes,
  type Manifest,
  mediaSourceClass,
  mime,
  openMediaSource,
  playable,
  type Track,
  update,
} from "./mse";

// Inlined into a video page: plays the page's starter on #sp-vod-boot's
// <video>, then hands it to the engine (docs/vod-instant-start.md).

type Starter = {
  video: { track: string; key: string | null; rest: string | null };
  audio?: { track: string; data: string | null } | null;
};

const json = (id: string) =>
  JSON.parse(document.getElementById(id)?.textContent || "null");

async function boot(root: HTMLElement): Promise<VodEngine | undefined> {
  const video = root.querySelector("video")!;
  const manifest: Manifest = json("sp-vod-manifest");
  const starter: Starter | null = json("sp-vod-starter");
  const MS = mediaSourceClass();
  const {
    video: [tv],
    audio: ta,
  } = playable(manifest);
  const tracks = ta ? [tv, ta] : [tv];
  const key = starter?.video.track === tv.id && starter.video.key;
  if (!MS || !key || !tracks.every((t) => MS.isTypeSupported(mime(t)))) {
    return;
  }
  const rest = starter.video.rest;
  const audio = ta && starter.audio?.track === ta.id && starter.audio.data;

  // The body's var(--bootstrap-background) color, without inlining the theme.
  root.style.cssText =
    "position:fixed;inset:0;z-index:2147483647;background:inherit";
  video.style.cssText =
    "display:block;width:100%;height:100%;object-fit:contain";
  const firstFrame = new Promise((r) => {
    video.requestVideoFrameCallback?.(r);
    video.addEventListener("loadeddata", () => setTimeout(r, 500), {
      once: true,
    });
  });

  const ms = await openMediaSource(MS, video);
  ms.duration = manifest.durationMs / 1000;
  const sourceBuffers = {
    video: ms.addSourceBuffer(mime(tv)),
    audio: ta && ms.addSourceBuffer(mime(ta)),
  };
  const sb = (t: Track) => sourceBuffers[t.type]!;
  for (const t of tracks) {
    await update(sb(t), () => sb(t).appendBuffer(bytes(t.init)));
  }
  await appendAt(sourceBuffers.video, bytes(key), 0, tv.timescale);
  if (audio) await appendAt(sb(ta), bytes(audio), 0, ta.timescale);
  if (rest) {
    await update(sourceBuffers.video, () =>
      sourceBuffers.video.appendBuffer(bytes(rest)),
    );
  }
  // A blocked autoplay leaves the first frame showing.
  video.play().catch(() => {});

  await firstFrame;
  const engine: typeof import("./engine") = await import(root.dataset.engine!);
  return engine.adoptBoot({
    video,
    mediaSource: ms,
    sourceBuffers,
    manifest,
    appended: { [tv.id]: rest ? 1 : 0, ...(audio && { [ta.id]: 1 }) },
  });
}

const root = document.getElementById("sp-vod-boot");
if (root) {
  (window as Window & VodGlobals).__spVodBoot = boot(root).then(
    (engine) => {
      if (!engine) root.remove();
      return engine;
    },
    (err) => {
      console.error("VOD boot:", err);
      root.remove();
      return undefined;
    },
  );
}
