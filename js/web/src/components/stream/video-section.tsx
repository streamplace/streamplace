import type { LivestreamStore } from "@streamplace/core";
import { useMemo } from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import { Player } from "../../components/player/player";
import type { Liveness } from "../../hooks/use-liveness-state";
import { getStreamplaceUrl } from "../../lib/streamplace-url";
import { UserOffline } from "./user-offline";

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
}: {
  user: string;
  liveness: Liveness;
  segment: Segment;
  problems: Problem[];
  playlistUrl: string;
  thumbnailUrl: string;
  mode?: "live" | "vod";
}) {
  const neverLive = liveness === "never-live";

  // calculate seg ratio for poster aspect ratio correction
  const segRatio = segment ? segment.width / segment.height : 16 / 9;

  return (
    <div className="w-full max-h-[calc(100vh-240px)] h-full bg-black">
      <div
        className="relative bg-black h-full mx-auto"
        style={{
          maxHeight: "calc(100vh-240px)",
        }}
      >
        {neverLive ? (
          <img
            src={thumbnailUrl}
            alt=""
            className="absolute inset-0 w-full h-full object-contain bg-black"
            onError={(e) => {
              (e.currentTarget as HTMLImageElement).style.visibility = "hidden";
            }}
          />
        ) : (
          <Player src={playlistUrl} poster={thumbnailUrl} active mode={mode} />
        )}

        {liveness === "live" && (
          <div className="absolute top-3 left-3 pointer-events-none z-10">
            <div className="flex items-center gap-1.5 bg-red-600 px-2 py-0.5 rounded text-white text-xs font-bold uppercase tracking-wide">
              <div className="w-1.5 h-1.5 rounded-full bg-white" />
              Live
            </div>
          </div>
        )}

        {liveness === "stale" && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/60 z-10">
            <div className="text-center">
              <div className="w-10 h-10 mx-auto mb-3 rounded-full border-2 border-amber-400 border-t-transparent animate-spin" />
              <p className="text-amber-400 font-medium">Reconnecting…</p>
              <p className="text-white/60 text-sm mt-1">
                The stream may resume shortly.
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
              className="absolute bottom-3 left-3 bg-red-500/90 text-white text-xs px-3 py-1.5 rounded"
            >
              {p.message}
            </div>
          ))}
      </div>
    </div>
  );
}
