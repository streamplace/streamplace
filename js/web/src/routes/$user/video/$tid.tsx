import { VideoSectionInner } from "@/components/stream/video-section";
import { useVideoRecord } from "@/hooks/use-video-record";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { formatDuration } from "@/lib/video";
import { createFileRoute } from "@tanstack/react-router";
import { Download } from "lucide-react";
import { useCallback, useMemo, useState } from "react";

export const Route = createFileRoute("/$user/video/$tid")({
  component: VodPage,
});

function VodPage() {
  const { user, tid } = Route.useParams();
  const { record, author, loading, error } = useVideoRecord(user, tid);
  const [downloading, setDownloading] = useState(false);

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    const uri = `at://${user}/place.stream.video/${tid}`;
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getVideoPlaylist?uri=${encodeURIComponent(uri)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/video/${tid}/thumb.jpg`,
    };
  }, [user, tid]);

  const title = record?.title || "Untitled";

  const handleDownload = useCallback(async () => {
    setDownloading(true);
    try {
      // Fetch the HLS playlist and extract segment URLs.
      const res = await fetch(playlistUrl);
      const playlistText = await res.text();
      const lines = playlistText.split("\n");
      const segmentUrls: string[] = [];
      const playlistBase = playlistUrl.split("?")[0];
      const baseUrl = playlistBase.substring(0, playlistBase.lastIndexOf("/"));

      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith("#")) continue;
        // Segment URLs may be absolute or relative.
        if (trimmed.startsWith("http")) {
          segmentUrls.push(trimmed);
        } else {
          segmentUrls.push(`${baseUrl}/${trimmed}`);
        }
      }

      if (segmentUrls.length === 0) {
        // Fallback: download the playlist file itself.
        const blob = new Blob([playlistText], {
          type: "application/x-mpegURL",
        });
        const a = document.createElement("a");
        a.href = URL.createObjectURL(blob);
        a.download = `${title}.m3u8`;
        a.click();
        URL.revokeObjectURL(a.href);
        return;
      }

      // Download all segments and concatenate.
      const chunks: ArrayBuffer[] = [];
      for (const url of segmentUrls) {
        const segRes = await fetch(url);
        chunks.push(await segRes.arrayBuffer());
      }

      const blob = new Blob(chunks, { type: "video/mp4" });
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = `${title}.mp4`;
      a.click();
      URL.revokeObjectURL(a.href);
    } catch (e) {
      console.error("Download failed", e);
    } finally {
      setDownloading(false);
    }
  }, [playlistUrl, title]);
  const description = record?.description;
  const createdAt = record?.createdAt
    ? new Date(record.createdAt).toLocaleDateString(undefined, {
        year: "numeric",
        month: "long",
        day: "numeric",
      })
    : null;
  const duration = record?.durationMs
    ? formatDuration(record.durationMs)
    : null;

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
                <div className="flex items-start gap-2">
                  <h2 className="flex-1 font-display font-semibold text-[var(--color-fg)] line-clamp-2">
                    {loading ? "Loading..." : title}
                  </h2>
                  <button
                    type="button"
                    onClick={handleDownload}
                    disabled={downloading}
                    className="flex-shrink-0 p-2 rounded-md border border-[var(--color-border)] hover:border-[var(--color-border-strong)] text-[var(--color-fg-muted)] hover:text-[var(--color-fg)] transition-colors disabled:opacity-50"
                    title="Download video"
                  >
                    <Download className="w-4 h-4" />
                  </button>
                </div>

                <div className="flex items-center gap-2 mt-1 text-sm text-[var(--color-fg-muted)]">
                  <span className="truncate">
                    {author?.displayName || author?.handle || user}
                  </span>
                  {(author?.displayName || author?.handle) && (
                    <span className="text-[var(--color-fg-subtle)]">@</span>
                  )}
                  <span className="truncate">{author?.handle || user}</span>
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
