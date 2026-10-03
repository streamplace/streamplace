import { Player } from "@/components/player/player";
import { FullscreenProvider } from "@/contexts/fullscreen-context";
import { captureError } from "@/lib/log";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { parseTimeParam } from "@streamplace/core";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/embed/$user/video/$tid")({
  // `?t=` is the playback start time, YouTube-style: seconds or an
  // `1h2m3s` duration.
  validateSearch: (search: Record<string, unknown>): { t?: number } => ({
    t: parseTimeParam(search.t) ?? undefined,
  }),
  component: EmbedVideo,
});

function EmbedVideo() {
  const { user, tid } = Route.useParams();
  const { t: startTime } = Route.useSearch();

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    const uri = `at://${user}/place.stream.video/${tid}`;
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getVideoPlaylist?uri=${encodeURIComponent(uri)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/video/${tid}/thumb.jpg`,
    };
  }, [user, tid]);

  return (
    <FullscreenProvider>
      <div className="flex h-screen w-screen items-center justify-center bg-black">
        <Player
          src={playlistUrl}
          captionSource={{ video: `at://${user}/place.stream.video/${tid}` }}
          poster={thumbnailUrl}
          active
          mode="vod"
          startTime={startTime}
          onError={(message) =>
            captureError(message, { user, tid, source: "embed-vod" })
          }
        />
      </div>
    </FullscreenProvider>
  );
}
