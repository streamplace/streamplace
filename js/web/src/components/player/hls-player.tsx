// HLS backend for the <Player> component. Owns hls.js setup, manifest
// loading, and error recovery. Renders nothing — the video element
// lives in <Player> so the chrome (controls, fullscreen, error display)
// is shared across backends. When a WebRTC backend lands it will be a
// sibling of this file with the same shape.
import Hls, { type Level } from "hls.js";
import { useEffect, useImperativeHandle, useRef, type RefObject } from "react";
import type { PlayerBackendHandle, PlayerStats, QualityOption } from "./player";

const QUALITY_KEY = "player-quality";

function readQualityPreference(): number | null {
  try {
    const v = localStorage.getItem(QUALITY_KEY);
    if (v !== null) return parseInt(v, 10);
  } catch {
    // localStorage unavailable
  }
  return null;
}

export type HLSPlayerProps = {
  /** The video element managed by the parent <Player>. */
  videoRef: RefObject<HTMLVideoElement | null>;
  /** Full URL to the master HLS playlist. */
  src: string;
  /** False stops the current load and tears down hls.js. */
  active: boolean;
  /**
   * Called when a fatal hls.js error occurs. The parent surfaces the
   * message to the user; recovery (network retries, media recovery)
   * happens inside this component.
   */
  onError?: (message: string) => void;
  /** Called once hls.js parses the manifest, with the available qualities. */
  onQualitiesChange?: (qualities: QualityOption[]) => void;
  /** Called whenever hls.js switches to a new level (auto or user-initiated). */
  onCurrentQualityChange?: (index: number) => void;
  /** Called roughly once per second with a stats snapshot. */
  onStatsChange?: (stats: PlayerStats) => void;
};

export function HLSPlayer({
  ref,
  videoRef,
  src,
  active,
  onError,
  onQualitiesChange,
  onCurrentQualityChange,
  onStatsChange,
}: HLSPlayerProps & {
  /** Imperative handle so the chrome can set quality. */
  ref?: RefObject<PlayerBackendHandle | null>;
}) {
  const hlsRef = useRef<Hls | null>(null);
  // Ref so the stats-polling interval (a child of the main effect) can
  // call the latest onStatsChange without re-creating the interval on
  // every parent render.
  const onStatsChangeRef = useRef(onStatsChange);
  onStatsChangeRef.current = onStatsChange;

  useImperativeHandle(
    ref ?? { current: null },
    () => ({
      setQuality: (index: number) => {
        if (hlsRef.current) {
          hlsRef.current.currentLevel = index;
        }
      },
    }),
    [],
  );

  useEffect(() => {
    if (!active) return;
    const video = videoRef.current;
    if (!video) return;

    if (Hls.isSupported()) {
      const hls = new Hls({
        maxAudioFramesDrift: 20,
        lowLatencyMode: true,
        liveSyncDuration: 2.75,
        liveMaxLatencyDuration: 6,
        maxLiveSyncPlaybackRate: 1.5,
        backBufferLength: 90,
        enableWorker: true,
        debug: import.meta.env.DEV,
      });
      hlsRef.current = hls;

      hls.on(Hls.Events.MANIFEST_PARSED, (_e, data) => {
        onQualitiesChange?.(buildQualities(hls.levels));
        // Restore persisted quality preference.
        const saved = readQualityPreference();
        if (saved !== null && saved >= -1 && saved < hls.levels.length) {
          hls.currentLevel = saved;
        }
        video
          .play()
          .catch((err) => console.warn("[hls-player] play() rejected", err));
      });

      hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
        onCurrentQualityChange?.(data.level);
      });

      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return;
        const status = (data.response as Response | undefined)?.status;
        if (status === 404) {
          onError?.("Stream not live");
          return;
        }
        switch (data.type) {
          case Hls.ErrorTypes.NETWORK_ERROR:
            onError?.("Network error — retrying");
            hls.startLoad();
            return;
          case Hls.ErrorTypes.MEDIA_ERROR:
            onError?.("Media error — recovering");
            hls.recoverMediaError();
            return;
          default:
            onError?.(`${data.type}: ${data.details}`);
            return;
        }
      });

      hls.loadSource(src);
      try {
        hls.attachMedia(video);
      } catch {
        hls.stopLoad();
      }

      return () => {
        hls.destroy();
        hlsRef.current = null;
        // hls.js destroy() does not release the video element's source.
        // Null it out so the next backend starts clean.
        video.srcObject = null;
      };
    } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = src;
      const onCanPlay = () => {
        video.play().catch(() => {});
        video.removeEventListener("canplay", onCanPlay);
      };
      video.addEventListener("canplay", onCanPlay);
      return () => {
        video.removeEventListener("canplay", onCanPlay);
        video.removeAttribute("src");
        video.srcObject = null;
        video.load();
      };
    } else {
      onError?.("Your browser doesn't support HLS playback.");
    }
  }, [
    src,
    active,
    onError,
    onQualitiesChange,
    onCurrentQualityChange,
    videoRef,
  ]);

  // Poll stats ~once per second. Cheap reads from the video element and
  // hls.js instance; we only push when the consumer is listening.
  useEffect(() => {
    if (!active) return;
    // FPS via frame-delta sampling. We can't ask the element for its
    // current fps, so we measure how many frames were decoded between
    // two polls and divide by the elapsed wall-clock time.
    const lastFpsSample = { time: 0, frames: 0 };
    const id = setInterval(() => {
      const video = videoRef.current;
      if (!video) return;
      const hls = hlsRef.current;
      const playback = (
        video as HTMLVideoElement & {
          getVideoPlaybackQuality?: () => VideoPlaybackQuality;
        }
      ).getVideoPlaybackQuality?.();
      const buffered =
        video.buffered.length > 0
          ? video.buffered.end(video.buffered.length - 1) - video.currentTime
          : 0;
      const currentLevel = hls?.currentLevel ?? -1;
      // currentLevel is -1 in auto mode; loadLevel gives the actual level selected by ABR.
      const activeLevel =
        currentLevel >= 0 ? currentLevel : (hls?.loadLevel ?? -1);
      const currentLevelData =
        activeLevel >= 0 ? hls?.levels[activeLevel] : undefined;
      const totalFrames = playback?.totalVideoFrames ?? 0;
      const now = performance.now();
      let fps: number | undefined;
      if (lastFpsSample.time > 0) {
        const elapsed = now - lastFpsSample.time;
        const deltaFrames = totalFrames - lastFpsSample.frames;
        if (elapsed > 0 && deltaFrames >= 0) {
          fps = (deltaFrames / elapsed) * 1000;
        }
      }
      lastFpsSample.time = now;
      lastFpsSample.frames = totalFrames;

      // Latency to the live edge: distance between where we're playing
      // and the position hls.js considers "live". hls.js nulls this out
      // before the first playlist loads and for VOD.
      const liveEdge = hls?.liveSyncPosition;
      const latencyToBroadcaster =
        typeof liveEdge === "number" && liveEdge > 0
          ? Math.max(0, liveEdge - video.currentTime)
          : undefined;

      onStatsChangeRef.current?.({
        width: currentLevelData?.width ?? video.videoWidth,
        height: currentLevelData?.height ?? video.videoHeight,
        viewportWidth: typeof window === "undefined" ? 0 : window.innerWidth,
        viewportHeight: typeof window === "undefined" ? 0 : window.innerHeight,
        buffered,
        droppedFrames: playback?.droppedVideoFrames ?? 0,
        totalFrames,
        fps,
        bitrate: currentLevelData?.bitrate,
        ttfbEstimate: hls?.ttfbEstimate,
        level: currentLevel,
        codecs: currentLevelData?.codecs,
        hlsVersion: Hls.version,
        latencyToBroadcaster,
      });
    }, 1000);
    return () => clearInterval(id);
  }, [active, videoRef]);

  return null;
}

function buildQualities(levels: Level[]): QualityOption[] {
  const explicit = levels.map((level, index) => ({
    index,
    label: labelForLevel(level, index),
  }));
  // "Auto" maps to hls.js currentLevel = -1.
  return [{ index: -1, label: "Auto" }, ...explicit];
}

function labelForLevel(level: Level, index: number): string {
  if (level.height) return `${level.height}p`;
  if (level.bitrate) {
    const kbps = Math.round(level.bitrate / 1000);
    if (kbps > 0) return `${kbps} kbps`;
  }
  return `Level ${index}`;
}
