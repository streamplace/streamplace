import { Player } from "@/components/player/player";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/embed/$user/video/$tid")({
  component: EmbedVideo,
});

function EmbedVideo() {
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
    <div className="w-screen h-screen bg-black flex items-center justify-center">
      <Player src={playlistUrl} poster={thumbnailUrl} active mode="vod" />
    </div>
  );
}
