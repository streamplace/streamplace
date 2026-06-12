import type { LivestreamStore } from "@streamplace/core";
import { useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { Player } from "../../components/player/player";
import { useFullscreen } from "../../contexts/fullscreen-context";
import type { Liveness } from "../../hooks/use-liveness-state";
import { getStreamplaceUrl } from "../../lib/streamplace-url";
import { DanmuOverlay } from "./danmu-overlay";
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
}) {
  const { t } = useTranslation("common");
  const { theatre } = useFullscreen();
  const neverLive = liveness === "never-live";

  // calculate seg ratio for poster aspect ratio correction
  const segRatio = segment ? segment.width / segment.height : 16 / 9;

  return (
    <div
      className="w-full bg-black"
      style={{
        // In theatre mode the sidebar and header are hidden, so the video
        // fills the full viewport. Otherwise constrain to aspect ratio.
        ...(theatre
          ? { height: "100vh" }
          : {
              maxHeight: `min(calc(100vw / ${segRatio}), calc(100vh - 240px))`,
              aspectRatio: `${segRatio}`,
            }),
      }}
    >
      <div className="relative mx-auto h-full bg-green-400">
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
          <Player
            src={playlistUrl}
            poster={thumbnailUrl}
            active
            mode={mode}
            showDanmu={showDanmu}
            onShowDanmuChange={onShowDanmuChange}
          />
        )}

        {/* Danmu overlay sits on top of the video */}
        {store && (
          <DanmuOverlay store={store} enabled={showDanmu && !neverLive} />
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
