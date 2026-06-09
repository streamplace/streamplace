import { VideoSectionInner } from "@/components/stream/video-section";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/$user/video/$tid")({
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
    <div className="flex flex-col gap-3 h-full">
      <div className="flex-1 flex min-h-0 gap-4">
        <div className="flex-1 min-w-0 overflow-y-auto">
          <VideoSectionInner
            user={user}
            liveness="live"
            segment={null}
            problems={[]}
            playlistUrl={playlistUrl}
            thumbnailUrl={thumbnailUrl}
            mode="vod"
          />
          <div className="mt-3 space-y-3 mx-3">
            <div className="flex items-start gap-3">
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2">
                  <span className="truncate">@{user}</span>
                </div>
                <h2 className="font-display font-semibold text-[var(--color-fg)] mt-0.5 line-clamp-2">
                  Video playback
                </h2>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
