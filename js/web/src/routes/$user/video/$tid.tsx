import { VideoSectionInner } from "@/components/stream/video-section";
import { VodWatchHeader } from "@/components/video/vod-watch-header";
import { useFullscreen } from "@/contexts/fullscreen-context";
import { useVideoRecord } from "@/hooks/use-video-record";
import { getStreamplaceUrl } from "@/lib/streamplace-url";
import { createFileRoute } from "@tanstack/react-router";
import { useCallback, useMemo, useState } from "react";

export const Route = createFileRoute("/$user/video/$tid")({
  component: VodPage,
});

function VodPage() {
  const { user, tid } = Route.useParams();
  const { video, loading, error } = useVideoRecord(user, tid);
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

  const handleDownload = useCallback(async () => {
    setDownloading(true);
    try {
      // The playlist endpoint returns an HLS master playlist, not a
      // downloadable media file. Browser-side concatenation of HLS
      // segments produces corrupt output (segments are TS, not MP4;
      // master playlist variant URLs are playlists, not media; and
      // the whole thing requires demuxing/remuxing). Until a
      // server-side download endpoint exists, point the user to the
      // playlist URL directly.
      window.open(playlistUrl, "_blank");
    } catch (e) {
      console.error("Download failed", e);
    } finally {
      setDownloading(false);
    }
  }, [playlistUrl]);
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
            <>
              {loading && <VodWatchHeaderSkeleton />}
              {video && (
                <VodWatchHeader
                  video={video}
                  routeUser={user}
                  tid={tid}
                  downloading={downloading}
                  onDownload={handleDownload}
                />
              )}
              {error && (
                <p className="mx-auto mt-4 max-w-350 px-4 text-sm text-(--color-danger) sm:px-6">
                  {error}
                </p>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  );
}

function VodWatchHeaderSkeleton() {
  return (
    <div className="mx-auto mt-5 max-w-350 animate-pulse px-4 pb-8 sm:px-6">
      <div className="h-7 w-2/3 rounded bg-(--color-bg-elevated)" />
      <div className="mt-2 h-4 w-48 rounded bg-(--color-bg-elevated)" />
      <div className="mt-4 flex items-center gap-3">
        <div className="size-11 rounded-full bg-(--color-bg-elevated)" />
        <div className="h-4 w-36 rounded bg-(--color-bg-elevated)" />
      </div>
    </div>
  );
}
