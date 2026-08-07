import { VideoSectionInner } from "@/components/stream/video-section";
import { useFullscreen } from "@/contexts/fullscreen-context";
import { useVideoRecord } from "@/hooks/use-video-record";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { formatDuration } from "@/lib/video";
import { createFileRoute } from "@tanstack/react-router";
import { Download } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

export const Route = createFileRoute("/$user/video/$tid")({
  component: VodPage,
});

function VodPage() {
  const { t } = useTranslation("common");
  const { user, tid } = Route.useParams();
  const { record, author, loading, error } = useVideoRecord(user, tid);
  const [downloading, setDownloading] = useState(false);
  const { theatre } = useFullscreen();

  const { playlistUrl, thumbnailUrl } = useMemo(() => {
    const base = getStreamplaceUrl();
    const uri = `at://${user}/place.stream.video/${tid}`;
    return {
      playlistUrl: `${base}/xrpc/place.stream.playback.getVideoPlaylist?uri=${encodeURIComponent(uri)}`,
      thumbnailUrl: `${base}/api/playback/${encodeURIComponent(user)}/video/${tid}/thumb.jpg`,
    };
  }, [user, tid]);

  const title = record?.title || t("untitled");

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
    <div className="flex h-full flex-col gap-3">
      <div className="flex min-h-0 flex-1 gap-4">
        <div className="min-w-0 flex-1 overflow-y-auto">
          <VideoSectionInner
            user={user}
            liveness="live"
            segment={null}
            problems={[]}
            playlistUrl={playlistUrl}
            thumbnailUrl={thumbnailUrl}
            mode="vod"
          />
          {!theatre && (
            <div className="mx-3 mt-3 space-y-3">
              {loading ? (
                <div className="flex items-start gap-3">
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="h-6 w-2/3 animate-pulse rounded bg-(--color-bg-elevated)" />
                    <div className="h-4 w-1/3 animate-pulse rounded bg-(--color-bg-elevated)" />
                    <div className="h-3 w-1/4 animate-pulse rounded bg-(--color-bg-elevated)" />
                  </div>
                </div>
              ) : (
                <div className="flex items-start gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="flex items-start gap-2">
                      <h2 className="font-display line-clamp-2 flex-1 font-semibold text-(--color-fg)">
                        {title}
                      </h2>
                      <button
                        type="button"
                        onClick={handleDownload}
                        disabled={downloading}
                        className="shrink-0 rounded-md border border-(--color-border) p-2 text-(--color-fg-muted) transition-colors hover:border-(--color-border-strong) hover:text-(--color-fg) disabled:opacity-50"
                        title={t("download-video")}
                      >
                        <Download className="h-4 w-4" />
                      </button>
                    </div>

                    <div className="mt-1 flex items-center gap-2 text-sm text-(--color-fg-muted)">
                      <span className="truncate">
                        {author?.displayName || author?.handle || user}
                      </span>
                      {(author?.displayName || author?.handle) && (
                        <span className="text-(--color-fg-subtle)">@</span>
                      )}
                      <span className="truncate">{author?.handle || user}</span>
                    </div>

                    {duration && (
                      <span className="mt-1 block text-xs text-(--color-fg-muted)">
                        {duration}
                      </span>
                    )}

                    {createdAt && (
                      <span className="text-xs text-(--color-fg-muted)">
                        {createdAt}
                      </span>
                    )}

                    {description && (
                      <p className="mt-3 text-sm whitespace-pre-wrap text-(--color-fg)">
                        {description}
                      </p>
                    )}

                    {error && (
                      <p className="mt-2 text-sm text-(--color-danger)">
                        {error}
                      </p>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
