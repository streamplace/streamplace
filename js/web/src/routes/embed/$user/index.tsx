import { Player } from "@/components/player/player";
import { FullscreenProvider } from "@/contexts/fullscreen-context";
import { useLivestreamStore } from "@/hooks/use-livestream-store";
import { captureError } from "@/lib/log";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/embed/$user/")({
  component: EmbedLive,
});

function EmbedLive() {
  const { user } = Route.useParams();
  const { store } = useLivestreamStore(user);

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getLivePlaylist?streamer=${encodeURIComponent(user)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/stream.jpg`,
    };
  }, [user]);

  return (
    <FullscreenProvider>
      <div className="flex h-screen w-screen items-center justify-center bg-black">
        <Player
          src={playlistUrl}
          captionSource={{ streamer: user, store: store ?? undefined }}
          poster={thumbnailUrl}
          active
          mode="live"
          onError={(message) =>
            captureError(message, { user, source: "embed-live" })
          }
        />
      </div>
    </FullscreenProvider>
  );
}
