import type { LivestreamStore } from "@streamplace/core";
import { useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { Player } from "../../components/player/player";
import { useFullscreen } from "../../contexts/fullscreen-context";
import type { Liveness } from "../../hooks/use-liveness-state";
import { captureError } from "../../lib/log";
import { useStore as useAppStore } from "../../lib/store";
import { getStreamplaceUrl } from "../../lib/streamplace-url";
import { DanmuOverlay } from "./danmu-overlay";
import { PlayerOffline } from "./player-offline";
import { UserOffline } from "./user-offline";

const DANMU_KEY = "danmu-enabled";

function readDanmuPreference(): boolean {
  try {
    return localStorage.getItem(DANMU_KEY) === "true";
  } catch {
    return false;
  }
}

function writeDanmuPreference(enabled: boolean) {
  try {
    localStorage.setItem(DANMU_KEY, String(enabled));
  } catch {
    // ignore
  }
}

type Segment = { width: number; height: number } | null;
type Problem = { code: string; severity: string; message: string };

export function VideoSection({
  store,
  user,
  liveness,
}: {
  store: LivestreamStore;
  user: string;
  liveness: Liveness;
}) {
  const [showDanmu, setShowDanmu] = useState(readDanmuPreference);

  const handleDanmuChange = useCallback((enabled: boolean) => {
    setShowDanmu(enabled);
    writeDanmuPreference(enabled);
  }, []);

  // Danmu tuning preferences from the shared store (settings page).
  const danmuOpacity = useAppStore((s) => s.danmuOpacity);
  const danmuSpeed = useAppStore((s) => s.danmuSpeed);
  const danmuLaneCount = useAppStore((s) => s.danmuLaneCount);
  const danmuMaxMessages = useAppStore((s) => s.danmuMaxMessages);

  const state = useStore(
    store,
    useShallow((s) => ({
      livestream: s.livestream,
      problems: s.problems,
      websocketConnected: s.websocketConnected,
      segment: s.segment,
    })),
  );

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(user)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/stream.jpg`,
    };
  }, [user]);

  return (
    <VideoSectionInner
      user={user}
      liveness={liveness}
      segment={state.segment?.video?.at(0) ?? null}
      problems={state.problems}
      playlistUrl={playlistUrl}
      thumbnailUrl={thumbnailUrl}
      store={store}
      showDanmu={showDanmu}
      onShowDanmuChange={handleDanmuChange}
      danmuOpacity={danmuOpacity}
      danmuSpeed={danmuSpeed}
      danmuLaneCount={danmuLaneCount}
      danmuMaxMessages={danmuMaxMessages}
    />
  );
}

/**
 * Props-based video section that works without a LivestreamStore.
 * Used by VideoSection (live) and directly by VOD pages.
 */
export function VideoSectionInner({
  user,
  liveness,
  segment,
  problems,
  playlistUrl,
  thumbnailUrl,
  mode = "live",
  store,
  showDanmu = false,
  onShowDanmuChange,
  danmuOpacity = 80,
  danmuSpeed = 1,
  danmuLaneCount = 12,
  danmuMaxMessages = 50,
}: {
  user: string;
  liveness: Liveness;
  segment: Segment;
  problems: Problem[];
  playlistUrl: string;
  thumbnailUrl: string;
  mode?: "live" | "vod";
  store?: LivestreamStore;
  showDanmu?: boolean;
  onShowDanmuChange?: (show: boolean) => void;
  danmuOpacity?: number;
  danmuSpeed?: number;
  danmuLaneCount?: number;
  danmuMaxMessages?: number;
}) {
  const { t } = useTranslation("common");
  const { theatre } = useFullscreen();
  const neverLive = liveness === "never-live";
  const offline = liveness === "offline";
  // Show the live player when neither never-live nor offline. This
  // includes the initial loading state before the WebSocket sends its
  // first snapshot, plus live and temporarily stale streams.
  const showPlayer = !neverLive && !offline;

  // calculate seg ratio for poster aspect ratio correction
  const segRatio = segment ? segment.width / segment.height : 16 / 9;

  return (
    <div
      className="w-full bg-black transition-[height] duration-500 ease-in-out"
      style={{
        // In theatre mode the sidebar and header are hidden, so the video
        // fills the full viewport. Otherwise match the live aspect ratio
        // via the segRatio-driven maxHeight. When offline, collapse to a
        // 4:3 panel (75vw of height) capped at half the viewport so the
        // page below gets room. Using an explicit height (not
        // aspect-ratio) lets CSS transition the size change when the
        // stream goes live/offline.
        ...(theatre
          ? { height: "100vh" }
          : offline
            ? { height: "min(100vw, 50vh)" }
            : {
                height: `min(calc(100vw / ${segRatio}), calc(100vh - 240px))`,
              }),
      }}
    >
      <div className="relative mx-auto h-full">
        {neverLive ? (
          <img
            src={thumbnailUrl}
            alt=""
            className="absolute inset-0 h-full w-full bg-black object-contain"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).style.visibility = "hidden";
            }}
          />
        ) : (
          <>
            {/* Live player fades out when the stream goes offline. */}
            <div
              className={`absolute inset-0 transition-opacity duration-500 ${
                showPlayer ? "opacity-100" : "pointer-events-none opacity-0"
              }`}
            >
              <Player
                src={playlistUrl}
                poster={thumbnailUrl}
                active
                mode={mode}
                showDanmu={showDanmu}
                onShowDanmuChange={onShowDanmuChange}
                onError={(message) => captureError(message, { user, mode })}
              />
            </div>
            {offline && store && (
              <div className="animate-in fade-in absolute inset-0 duration-500">
                <PlayerOffline store={store} user={user} />
              </div>
            )}
          </>
        )}

        {/* Danmu overlay sits on top of the video */}
        {store && (
          <DanmuOverlay
            store={store}
            enabled={showDanmu && showPlayer}
            opacity={danmuOpacity}
            speed={danmuSpeed}
            laneCount={danmuLaneCount}
            maxMessages={danmuMaxMessages}
          />
        )}
        {liveness === "stale" && (
          <div className="absolute inset-0 z-10 flex items-center justify-center bg-black/60">
            <div className="text-center">
              <div className="mx-auto mb-3 h-10 w-10 animate-spin rounded-full border-2 border-amber-400 border-t-transparent" />
              <p className="font-medium text-amber-400">{t("reconnecting")}</p>
              <p className="mt-1 text-sm text-white/60">
                {t("stream-may-resume")}
              </p>
            </div>
          </div>
        )}

        {neverLive && <UserOffline user={user} />}

        {problems
          .filter((p) => p.severity === "error")
          .map((p) => (
            <div
              key={p.code}
              className="absolute bottom-3 left-3 rounded bg-red-500/90 px-3 py-1.5 text-xs text-white"
            >
              {p.message}
            </div>
          ))}
      </div>
    </div>
  );
}
