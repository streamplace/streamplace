import { VideoSectionInner } from "@/components/stream/video-section";
import { useVideoRecord } from "@/hooks/use-video-record";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { formatDuration } from "@/lib/video";
import { createFileRoute } from "@tanstack/react-router";
import { useMemo } from "react";

export const Route = createFileRoute("/$user/video/$tid")({
  component: VodPage,
});

function VodPage() {
  const { user, tid } = Route.useParams();
  const { record, author, loading, error } = useVideoRecord(user, tid);

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    const uri = `at://${user}/place.stream.video/${tid}`;
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getVideoPlaylist?uri=${encodeURIComponent(uri)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/video/${tid}/thumb.jpg`,
    };
  }, [user, tid]);

  const title = record?.title || "Untitled";
  const description = record?.description;
  const createdAt = record?.createdAt
    ? new Date(record.createdAt).toLocaleDateString(undefined, {
        year: "numeric",
        month: "long",
        day: "numeric",
      })
    : null;
  const duration = record?.durationMs ? formatDuration(record.durationMs) : null;

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
                <h2 className="font-display font-semibold text-[var(--color-fg)] line-clamp-2">
                  {loading ? "Loading..." : title}
                </h2>

                <div className="flex items-center gap-2 mt-1 text-sm text-[var(--color-fg-muted)]">
                  <span className="truncate">
                    {author?.displayName || author?.handle || user}
                  </span>
                  {(author?.displayName || author?.handle) && (
                    <span className="text-[var(--color-fg-subtle)]">@</span>
                  )}
                  <span className="truncate">
                    {author?.handle || user}
                  </span>
                </div>

                {duration && (
                  <span className="text-xs text-[var(--color-fg-muted)] mt-1 block">
                    {duration}
                  </span>
                )}

                {createdAt && (
                  <span className="text-xs text-[var(--color-fg-muted)]">
                    {createdAt}
                  </span>
                )}

                {description && (
                  <p className="text-sm text-[var(--color-fg)] mt-3 whitespace-pre-wrap">
                    {description}
                  </p>
                )}

                {error && (
                  <p className="text-sm text-[var(--color-danger)] mt-2">
                    {error}
                  </p>
                )}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
