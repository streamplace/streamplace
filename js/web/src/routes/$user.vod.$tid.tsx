// VOD playback page. Plays a single video using the existing HLSPlayer
// pointed at the streamplace VOD playlist endpoint. Port of
// js/app/src/screens/video.tsx (simplified for web).

import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";
import { HLSPlayer } from "../components/player/hls-player";
import { getStreamplaceUrl } from "../lib/streamplace-url";

export const Route = createFileRoute("/$user/vod/$tid")({
  component: VodPage,
});

function VodPage() {
  const { user, tid } = Route.useParams();

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    const uri = `at://${user}/place.stream.video/${tid}`;
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getVideoPlaylist?uri=${encodeURIComponent(uri)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/video/${tid}/thumb.jpg`,
    };
  }, [user, tid]);

  return (
    <div className="max-w-5xl mx-auto px-4 py-6">
      <div className="w-full max-h-[75vh] overflow-hidden rounded-xl bg-black">
        <HLSPlayer src={playlistUrl} poster={thumbnailUrl} active />
      </div>
      <div className="mt-4">
        <h1 className="text-lg font-semibold text-[var(--color-fg)]">
          @{user}
        </h1>
        <p className="text-sm text-[var(--color-fg-muted)] mt-1">
          Video playback
        </p>
      </div>
    </div>
  );
}
