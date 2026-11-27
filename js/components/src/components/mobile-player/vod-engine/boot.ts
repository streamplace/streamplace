import { withAlpha } from "../../../lib/theme/chrome";
import {
  borderAlphas,
  borderRadius,
  breakpoints,
  colors,
  motion,
  spacing,
  surfaces,
  typeScale,
} from "../../../lib/theme/tokens";
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

// Keep these bands in sync with PlayerInner, useIsLargeScreen and VodMobileMetadata.
const headerMinWidth = 981;
const theaterMinWidth = 1200;
const metadataMinWidth = 480;
const theaterHeightFraction = 0.8;
// shell.tsx's VOD header bar.
const headerHeight = 56;
const headerBlur = 18;

function frame(
  root: HTMLElement,
  video: HTMLVideoElement,
  aspectRatio: number,
) {
  const style = document.createElement("style");
  style.textContent = `
    #sp-vod-boot {
      position: fixed; inset: 0; z-index: 2147483647; overflow: hidden;
      background: ${colors.black};
    }
    #sp-vod-boot * { box-sizing: border-box; }
    #sp-vod-boot .skeleton {
      background: ${surfaces.dark[2]}; border-radius: ${borderRadius.sm}px;
      animation: sp-vod-pulse ${motion.slow * 3}ms ${motion.easingCss} infinite alternate;
    }
    #sp-vod-boot .header { display: none; }
    #sp-vod-boot .player {
      width: 100%; aspect-ratio: ${aspectRatio};
      ${
        aspectRatio > 1
          ? `max-height: 100vh; margin-inline: max(0px, calc((100vw - 100vh * ${aspectRatio}) / 2));`
          : "max-height: 70vh;"
      }
    }
    #sp-vod-boot .metadata {
      display: grid; gap: ${spacing[4]}px; padding: ${spacing[4]}px;
    }
    #sp-vod-boot .identity { display: grid; gap: ${spacing[2]}px; }
    #sp-vod-boot .title {
      width: ${spacing[40]}px; max-width: 75%; height: ${typeScale.base.lineHeight}px;
      display: flex; align-items: center;
    }
    #sp-vod-boot .line { height: ${typeScale.xs.fontSize}px; width: 100%; }
    #sp-vod-boot .author, #sp-vod-boot .actions {
      display: flex; align-items: center; gap: ${spacing[2]}px; min-width: 0;
    }
    #sp-vod-boot .avatar {
      width: ${spacing[8]}px; height: ${spacing[8]}px;
      border-radius: ${borderRadius.full}px; flex-shrink: 0;
    }
    #sp-vod-boot .handle { width: ${spacing[80]}px; max-width: 80%; }
    #sp-vod-boot .actions { gap: ${spacing[4]}px; }
    #sp-vod-boot .pill {
      width: ${spacing[16]}px; height: ${spacing[8]}px;
      border-radius: ${borderRadius.full}px;
    }
    #sp-vod-boot .share { width: ${spacing[20]}px; }
    #sp-vod-boot .description {
      background: ${surfaces.dark[1]}; border-radius: ${borderRadius.lg}px;
      padding: ${spacing[3]}px; min-height: ${typeScale.sm.lineHeight + spacing[3] * 2}px;
      display: flex; align-items: center;
    }
    #sp-vod-boot .views { width: ${spacing[48]}px; max-width: 75%; }
    #sp-vod-boot .comments { width: ${spacing[24]}px; height: ${typeScale.base.lineHeight}px; }
    @media (min-width: ${metadataMinWidth}px) {
      #sp-vod-boot .identity { grid-template-columns: minmax(0, 1fr) auto; }
      #sp-vod-boot .title { grid-column: 1 / -1; }
    }
    @media (min-width: ${headerMinWidth}px) {
      #sp-vod-boot { padding-top: ${headerHeight}px; }
      #sp-vod-boot .header {
        position: absolute; inset: 0 0 auto; height: ${headerHeight}px;
        display: flex; align-items: center; gap: ${spacing[4]}px;
        padding-inline: ${spacing[6]}px;
        background: ${withAlpha(surfaces.dark[0], 0.55)};
        backdrop-filter: blur(${headerBlur}px);
        border-bottom: ${spacing[1] / 4}px solid ${borderAlphas.dark.subtle};
      }
      #sp-vod-boot .menu { width: ${spacing[4]}px; height: ${spacing[4]}px; }
      #sp-vod-boot .brand { width: ${spacing[56]}px; height: ${spacing[6]}px; }
      #sp-vod-boot .search {
        position: absolute; left: 50%; transform: translateX(-50%);
        width: 45%; max-width: ${breakpoints.sm}px;
        height: ${headerHeight - spacing[3]}px; border-radius: ${borderRadius.md}px;
        background: ${surfaces.dark[1]};
        border: ${spacing[1] / 4}px solid ${borderAlphas.dark.subtle};
      }
    }
    @media (min-width: ${theaterMinWidth + 1}px) {
      #sp-vod-boot { padding-top: ${headerHeight + spacing[4]}px; }
      #sp-vod-boot .player {
        width: min(calc(100% - ${spacing[6] * 2}px), ${100 * theaterHeightFraction * aspectRatio}vh);
        margin-inline: auto;
      }
      #sp-vod-boot .metadata { padding-inline: ${spacing[6]}px; }
    }
    @keyframes sp-vod-pulse { from { opacity: 0.55; } to { opacity: 1; } }
    @media (prefers-reduced-motion: reduce) {
      #sp-vod-boot .skeleton { animation: none; }
    }
  `;
  root.append(style);
  root.insertAdjacentHTML(
    "beforeend",
    `<div class="header" aria-hidden="true">
      <div class="menu skeleton"></div><div class="brand skeleton"></div>
      <div class="search"></div>
    </div>
    <div class="player"></div>
    <div class="metadata" aria-hidden="true">
      <div class="identity">
        <div class="title"><div class="line skeleton"></div></div>
        <div class="author"><div class="avatar skeleton"></div><div class="handle line skeleton"></div></div>
        <div class="actions"><div class="pill skeleton"></div><div class="pill share skeleton"></div></div>
      </div>
      <div class="description"><div class="views line skeleton"></div></div>
      <div class="comments"><div class="line skeleton"></div></div>
    </div>`,
  );
  root.querySelector(".player")!.append(video);
  video.style.cssText =
    "display:block;width:100%;height:100%;object-fit:contain";
}

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
  const audio = ta && starter?.audio?.track === ta.id && starter.audio.data;
  if (
    !MS ||
    !key ||
    (ta && !audio) ||
    !tracks.every((t) => MS.isTypeSupported(mime(t)))
  ) {
    return;
  }
  const rest = starter.video.rest;

  frame(root, video, tv.width && tv.height ? tv.width / tv.height : 16 / 9);
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
