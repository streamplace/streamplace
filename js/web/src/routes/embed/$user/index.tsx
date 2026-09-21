import { Player } from "@/components/player/player";
import { captureError } from "@/lib/log";
import {
  getLiveHLSUrl,
  getLiveLLHLSUrl,
  getStreamplaceUrl,
} from "@/lib/streamplace-url";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/embed/$user/")({
  component: EmbedLive,
});

function EmbedLive() {
  const { user } = Route.useParams();

  const { playlistUrl, llHlsUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    return {
      playlistUrl: getLiveHLSUrl(user),
      llHlsUrl: getLiveLLHLSUrl(user),
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/stream.jpg`,
    };
  }, [user]);

  return (
    <div className="flex h-screen w-screen items-center justify-center bg-black">
      <Player
        src={playlistUrl}
        llHlsSrc={llHlsUrl}
        poster={thumbnailUrl}
        active
        mode="live"
        onError={(message) =>
          captureError(message, { user, source: "embed-live" })
        }
      />
    </div>
  );
}
