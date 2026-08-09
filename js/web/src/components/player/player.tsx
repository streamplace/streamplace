// Public <Player> component. Owns the video element and the shared
// chrome (controls overlay, error display, fullscreen, auto-hide,
// click-to-toggle). The actual playback source is handled by a
// backend, currently <HLSPlayer>, soon <WebRTCPlayer> and others.
// Backends are rendered as siblings of the <video> element and use
// the shared videoRef to attach their source. This keeps the chrome
// implementation-free of any specific transport.
import { useSonare } from "@/lib/useSonare";
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type RefObject,
} from "react";
import { useTranslation } from "react-i18next";
import { cn } from "../../lib/utils";
import { Loader } from "../ui/loader";
import { HLSPlayer } from "./hls-player";
import {
  getBufferingOverlayPresentation,
  getBufferingState,
  shouldShowBufferingIndicator,
  type BufferingMediaEvent,
} from "./player-buffering";
import { PlayerControls } from "./player-controls";
import { WebRTCPlayer } from "./webrtc-player";

export type PlayerProps = {
  /** Full URL to the media source. Scheme/route is dispatched to a backend. */
  src: string;
  /** Live streams hide the scrubber; VODs get a seek bar. */
  mode?: "live" | "vod";
  /** Optional poster image (e.g. /api/playback/:user/stream.jpg). */
  poster?: string;
  /** Optional poster that overrides the default when the stream is offline. */
  fallbackPoster?: string;
  /** True when playback should be active. When false we just render the poster. */
  active: boolean;
  /**
   * Called when the player hits a fatal error. The parent may surface
   * this as a `LivestreamProblem` in the store.
   */
  onError?: (message: string) => void;
  /** Called once playback has actually started. */
  onPlaying?: () => void;
  /** Whether the danmu overlay is visible. */
  showDanmu?: boolean;
  /** Toggle the danmu overlay. */
  onShowDanmuChange?: (show: boolean) => void;
};

/** One quality option shown in the player's settings menu. */
export type QualityOption = {
  /** Backend-specific index. hls.js uses -1 for "auto". */
  index: number;
  /** Human-readable label, e.g. "Auto", "1080p", "720p". */
  label: string;
};

/**
 * Snapshot of player stats pushed roughly once a second. The shape is
 * shared by all backends; fields are optional when a backend doesn't
 * supply them (e.g. WebRTC won't have hls.js's bitrate).
 */
export type PlayerStats = {
  /** Current decoded video width. */
  width: number;
  /** Current decoded video height. */
  height: number;
  /** Current browser viewport width. */
  viewportWidth: number;
  /** Current browser viewport height. */
  viewportHeight: number;
  /** Seconds of buffer ahead of the playhead. */
  buffered: number;
  /** Decoded-and-dropped video frames since load. */
  droppedFrames: number;
  /** Total decoded video frames since load. */
  totalFrames: number;
  /** Computed from frame deltas (current interval, fps). */
  fps?: number;
  /** Current level bitrate in bits per second (HLS-specific). */
  bitrate?: number;
  /** hls.js's time-to-first-byte estimate in milliseconds (HLS-specific). */
  ttfbEstimate?: number;
  /** Current level index (-1 for auto in HLS). */
  level: number;
  /** Comma-separated codec string for the current level (HLS-specific). */
  codecs?: string;
  /** hls.js package version, e.g. "1.5.17" (HLS-specific). */
  hlsVersion?: string;
  /** Seconds between the playhead and the live edge (HLS-specific). */
  latencyToBroadcaster?: number;
};

/**
 * Imperative handle the chrome holds for the active backend. Backends
 * implement whichever operations their transport supports. HLS exposes
 * `setQuality`; future WebRTC may expose a no-op or a different model.
 */
export type PlayerBackendHandle = {
  setQuality: (index: number) => void;
};

const IDLE_HIDE_MS = 3000;
const TRANSPORT_KEY = "player-transport";

function readTransportPreference(): boolean {
  try {
    const v = localStorage.getItem(TRANSPORT_KEY);
    if (v === "hls") return false;
    if (v === "webrtc") return true;
  } catch {
    // localStorage unavailable (SSR, private browsing, etc.)
  }
  return true;
}

function writeTransportPreference(useWebRTC: boolean) {
  try {
    localStorage.setItem(TRANSPORT_KEY, useWebRTC ? "webrtc" : "hls");
  } catch {
    // ignore
  }
}

const QUALITY_KEY = "player-quality";

export function readQualityPreference(): number | null {
  try {
    const v = localStorage.getItem(QUALITY_KEY);
    if (v !== null) return parseInt(v, 10);
  } catch {
    // localStorage unavailable
  }
  return null;
}

function writeQualityPreference(index: number) {
  try {
    localStorage.setItem(QUALITY_KEY, String(index));
  } catch {
    // ignore
  }
}

export function Player({
  src,
  mode = "live",
  poster,
  fallbackPoster,
  active,
  onError,
  onPlaying,
  showDanmu = false,
  onShowDanmuChange,
}: PlayerProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const backendRef = useRef<PlayerBackendHandle | null>(null);
  const idleTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { t } = useTranslation();

  const [playing, setPlaying] = useState(false);
  const [buffering, setBuffering] = useState(active);
  const [hasPlayed, setHasPlayed] = useState(false);
  const [manifestReady, setManifestReady] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showControls, setShowControls] = useState(true);
  const [qualities, setQualities] = useState<QualityOption[]>([]);
  const [currentQuality, setCurrentQuality] = useState<number>(-1);
  const [useWebRTC, setUseWebRTC] = useState(readTransportPreference);
  const [showStats, setShowStats] = useState(false);
  const [stats, setStats] = useState<PlayerStats | null>(null);
  // Stable per-mount id for video playback
  const [sessionId] = useSonare();

  // Refs so the video event-listener effect below doesn't have to list
  // these as deps; it would otherwise tear down and re-add its
  // listeners on every transport toggle or parent re-render.
  const useWebRTCRef = useRef(useWebRTC);
  const onErrorRef = useRef(onError);
  useWebRTCRef.current = useWebRTC;
  onErrorRef.current = onError;

  const surfaceError = useCallback((msg: string) => {
    setError(msg);
    onErrorRef.current?.(msg);
    // WebRTC failed; fall back to HLS automatically.
    if (useWebRTCRef.current) {
      setUseWebRTC(false);
      writeTransportPreference(false);
    }
  }, []);

  const handleWebRTCChange = useCallback((value: boolean) => {
    setUseWebRTC(value);
    writeTransportPreference(value);
  }, []);

  // Reset per-source state when src or active changes.
  useEffect(() => {
    setError(null);
    setBuffering(active);
    setHasPlayed(false);
    setManifestReady(false);
    setQualities([]);
    setCurrentQuality(-1);
    setStats(null);
  }, [src, active]);

  // When the user toggles transport (HLS <-> WebRTC) the backend child
  // inside <PlayerBackend> is a different component type, so React
  // unmounts the old one and mounts the new one automatically. We just
  // need to reset the chrome's per-transport state.
  useEffect(() => {
    setBuffering(active);
    setQualities([]);
    setCurrentQuality(-1);
    setStats(null);
  }, [active, useWebRTC]);

  // Mirror the video element's state into React.
  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;

    const onPlay = () => setPlaying(true);
    const onPause = () => setPlaying(false);
    const onPlayingEvt = () => {
      setBuffering(false);
      setHasPlayed(true);
      setManifestReady(true);
      setError(null);
      onPlaying?.();
    };
    const onLoadedMetadata = () => setManifestReady(true);
    const onBufferingChange = (event: Event) => {
      setBuffering(getBufferingState(event.type as BufferingMediaEvent));
    };
    const onErrorEvt = () => {
      const code = video.error?.code;
      // MediaError code 1 (MEDIA_ERR_ABORTED) fires when the user or
      // a backend tears down the source; not a real error to surface.
      if (code === 1) return;
      surfaceError(
        code === 2
          ? t("player-error-network")
          : code === 3
            ? t("player-error-playback")
            : code === 4
              ? t("player-error-format")
              : t("player-error-playback"),
      );
    };

    video.addEventListener("play", onPlay);
    video.addEventListener("pause", onPause);
    video.addEventListener("playing", onPlayingEvt);
    video.addEventListener("loadedmetadata", onLoadedMetadata);
    video.addEventListener("error", onErrorEvt);
    const bufferingEvents = [
      "loadstart",
      "waiting",
      "seeking",
      "canplay",
      "seeked",
      "pause",
      "ended",
      "error",
    ] satisfies BufferingMediaEvent[];
    for (const event of bufferingEvents) {
      video.addEventListener(event, onBufferingChange);
    }

    return () => {
      video.removeEventListener("play", onPlay);
      video.removeEventListener("pause", onPause);
      video.removeEventListener("playing", onPlayingEvt);
      video.removeEventListener("loadedmetadata", onLoadedMetadata);
      video.removeEventListener("error", onErrorEvt);
      for (const event of bufferingEvents) {
        video.removeEventListener(event, onBufferingChange);
      }
    };
  }, [active, onPlaying, surfaceError]);

  // Big play button shows whenever playback is gated (paused, or
  // manifest not yet ready and the user hasn't successfully played).
  const showBigPlay = active && (!playing || (!hasPlayed && manifestReady));
  const showBuffering = shouldShowBufferingIndicator({
    active,
    buffering,
    bigPlay: showBigPlay,
    hasError: !!error,
  });
  const bufferingOverlay = getBufferingOverlayPresentation(showBuffering);

  // Auto-hide controls on idle. Reset on any pointer activity.
  const bumpControls = useCallback(() => {
    setShowControls(true);
    if (idleTimerRef.current) clearTimeout(idleTimerRef.current);
    if (playing && !error) {
      idleTimerRef.current = setTimeout(
        () => setShowControls(false),
        IDLE_HIDE_MS,
      );
    }
  }, [playing, error]);

  useEffect(() => {
    bumpControls();
    return () => {
      if (idleTimerRef.current) clearTimeout(idleTimerRef.current);
    };
  }, [bumpControls]);

  const setQuality = useCallback((index: number) => {
    backendRef.current?.setQuality(index);
    writeQualityPreference(index);
  }, []);

  return (
    <div
      ref={containerRef}
      className="group relative h-full w-full bg-black"
      onMouseMove={bumpControls}
      onMouseLeave={() => {
        if (playing && !error) {
          if (idleTimerRef.current) clearTimeout(idleTimerRef.current);
          idleTimerRef.current = setTimeout(
            () => setShowControls(false),
            IDLE_HIDE_MS,
          );
        }
      }}
      onClick={() => {
        if (showBigPlay) return; // big play button handles it
        const video = videoRef.current;
        if (!video) return;
        if (video.paused) {
          video
            .play()
            .catch((err) => console.warn("[player] play() rejected", err));
        } else {
          video.pause();
        }
      }}
    >
      {active ? (
        <video
          ref={videoRef}
          poster={poster}
          muted
          autoPlay
          playsInline
          className="absolute inset-0 h-full w-full object-contain"
        />
      ) : (
        <img
          src={fallbackPoster ?? poster}
          alt=""
          className="absolute inset-0 h-full w-full bg-black object-contain"
        />
      )}

      {active && (
        <PlayerBackend
          src={src}
          useWebRTC={useWebRTC}
          videoRef={videoRef}
          active={active}
          onError={surfaceError}
          onQualitiesChange={setQualities}
          onCurrentQualityChange={setCurrentQuality}
          onStatsChange={setStats}
          ref={backendRef}
        />
      )}

      {active && (
        <div
          aria-hidden={bufferingOverlay.ariaHidden}
          className={cn(
            "pointer-events-none absolute inset-0 z-10 flex items-center justify-center transition-opacity duration-200 ease-in-out motion-reduce:duration-150",
            bufferingOverlay.opacityClass,
          )}
        >
          <Loader label={t("player-buffering")} className="text-white" />
        </div>
      )}

      {active && (
        <PlayerControls
          videoRef={videoRef}
          containerRef={containerRef}
          isLive={mode === "live"}
          showControls={showControls}
          bigPlay={showBigPlay}
          forceVisible={!!error}
          qualities={qualities}
          currentQuality={currentQuality}
          onQualityChange={setQuality}
          useWebRTC={useWebRTC}
          onUseWebRTCChange={handleWebRTCChange}
          showStats={showStats}
          onShowStatsChange={setShowStats}
          showDanmu={showDanmu}
          onShowDanmuChange={(v) => onShowDanmuChange?.(v)}
        />
      )}

      {active && showStats && stats && (
        <StatsOverlay
          stats={stats}
          protocol={
            useWebRTC ? t("player-protocol-webrtc") : t("player-protocol-hls")
          }
          latencyMode={
            useWebRTC ? t("player-latency-low") : t("player-latency-standard")
          }
          sessionId={sessionId}
        />
      )}

      {error && (
        <div className="pointer-events-auto absolute top-2 right-2 left-2 rounded bg-red-500/90 px-3 py-2 text-sm text-white">
          {error}
        </div>
      )}
    </div>
  );
}

/**
 * Picks the playback backend for the current `src` and renders it. New
 * transports (WebRTC, direct MP4, external embeds, etc.) go here.
 */
function PlayerBackend({
  src,
  useWebRTC,
  videoRef,
  active,
  onError,
  onQualitiesChange,
  onCurrentQualityChange,
  onStatsChange,
  ref,
}: {
  src: string;
  useWebRTC: boolean;
  videoRef: RefObject<HTMLVideoElement | null>;
  active: boolean;
  onError: (msg: string) => void;
  onQualitiesChange: (qualities: QualityOption[]) => void;
  onCurrentQualityChange: (index: number) => void;
  onStatsChange: (stats: PlayerStats) => void;
  ref: RefObject<PlayerBackendHandle | null>;
}) {
  if (useWebRTC || src.startsWith("webrtc://")) {
    return (
      <WebRTCPlayer
        ref={ref}
        videoRef={videoRef}
        src={src}
        active={active}
        onError={onError}
        onStatsChange={onStatsChange}
      />
    );
  }
  return (
    <HLSPlayer
      ref={ref}
      videoRef={videoRef}
      src={src}
      active={active}
      onError={onError}
      onQualitiesChange={onQualitiesChange}
      onCurrentQualityChange={onCurrentQualityChange}
      onStatsChange={onStatsChange}
    />
  );
}

/**
 * "Stats for nerds" panel. Draggable, moveable window; grab anywhere
 * on the panel and drag to reposition. Uses pointer events so mouse and
 * touch both work, with setPointerCapture so the drag keeps tracking
 * even if the cursor leaves the panel mid-drag.
 */
function StatsOverlay({
  stats,
  protocol,
  latencyMode,
  sessionId,
}: {
  stats: PlayerStats;
  protocol: string;
  latencyMode: string;
  sessionId: string;
}) {
  const { t } = useTranslation();
  const [position, setPosition] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const dragStart = useRef({ x: 0, y: 0, posX: 0, posY: 0 });

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId);
    setIsDragging(true);
    dragStart.current = {
      x: e.clientX,
      y: e.clientY,
      posX: position.x,
      posY: position.y,
    };
  };

  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isDragging) return;
    const dx = e.clientX - dragStart.current.x;
    const dy = e.clientY - dragStart.current.y;
    setPosition({
      x: dragStart.current.posX + dx,
      y: dragStart.current.posY + dy,
    });
  };

  const onPointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.currentTarget.hasPointerCapture(e.pointerId)) {
      e.currentTarget.releasePointerCapture(e.pointerId);
    }
    setIsDragging(false);
  };

  const kbps = stats.bitrate ? Math.round(stats.bitrate / 1000) : null;
  const mbps = kbps ? (kbps / 1000).toFixed(2) : null;
  const bitrateDisplay =
    mbps && kbps && kbps > 1000
      ? `${mbps} Mbps`
      : kbps
        ? `${kbps} Kbps`
        : "N/A";
  return (
    <div
      role="dialog"
      aria-label={t("player-stats")}
      // Stop the synthetic click from bubbling to the player's
      // click-to-toggle when the user just taps the panel (no drag).
      onClick={(e) => e.stopPropagation()}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      className={cn(
        "absolute top-12 right-2 z-20 min-w-48 touch-none rounded bg-black/80 font-mono text-[11px] leading-snug text-white select-none",
        isDragging
          ? "cursor-grabbing shadow-lg ring-1 ring-white/20"
          : "cursor-grab hover:bg-black/90",
      )}
      style={{ transform: `translate(${position.x}px, ${position.y}px)` }}
    >
      <div className="bg-muted/50 mb-1 w-full rounded-t px-2.5 pt-1.5 pb-0.5 font-mono text-sm text-white/40 select-none">
        Stats
      </div>
      <div className="px-2.5 py-1.5">
        <Row
          label={t("player-stats-resolution")}
          value={`${stats.width}×${stats.height}`}
        />
        <Row
          label={t("player-stats-viewport")}
          value={`${stats.viewportWidth}×${stats.viewportHeight}`}
        />
        <Row label={t("player-stats-bitrate")} value={bitrateDisplay} />
        {stats.ttfbEstimate !== undefined && stats.ttfbEstimate > 0 && (
          <Row label="TTFB" value={`${Math.round(stats.ttfbEstimate)} ms`} />
        )}
        <Row
          label="FPS"
          value={
            stats.fps !== undefined ? Math.round(stats.fps).toString() : "N/A"
          }
        />
        <Row
          label={t("player-stats-skipped")}
          value={`${stats.droppedFrames} / ${stats.totalFrames}`}
        />
        <Row
          label={t("player-stats-buffer")}
          value={`${stats.buffered.toFixed(2)} sec`}
        />
        {stats.latencyToBroadcaster !== undefined && (
          <Row
            label={t("player-latency")}
            value={`${stats.latencyToBroadcaster.toFixed(2)} sec`}
          />
        )}
        {stats.codecs && (
          <Row label={t("player-stats-codecs")} value={stats.codecs} />
        )}
        <Row label={t("player-stats-protocol")} value={protocol} />
        <Row label={t("player-stats-latency-mode")} value={latencyMode} />
        <Row label={t("player-stats-render-surface")} value="video" />
        {stats.hlsVersion && (
          <Row label="hls.js version" value={stats.hlsVersion} />
        )}
        <Row label={t("player-stats-session")} value={sessionId} />
      </div>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="text-primary/50 font-mono text-sm">{label}</span>
      <span className="font-mono text-sm tabular-nums">{value}</span>
    </div>
  );
}
