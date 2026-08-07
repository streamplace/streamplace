import { Player } from "@/components/player/player";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/embed/$user/")({
  component: EmbedLive,
});

function EmbedLive() {
  const { user } = Route.useParams();

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(user)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/stream.jpg`,
    };
  }, [user]);

  return (
    <div className="flex h-screen w-screen items-center justify-center bg-black">
      <Player src={playlistUrl} poster={thumbnailUrl} active mode="live" />
    </div>
  );
}
