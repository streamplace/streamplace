// Global VOD browse page. Fetches place.stream.media.getVideoList
// with cursor-based infinite scroll. Port of
// js/app/src/screens/video-list.tsx (without repo filter).

import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useMemo, useRef } from "react";
import { useTranslation } from "react-i18next";
import { VideoCard } from "../components/video/video-card";
import useAvatars from "../hooks/use-avatars";
import { useVideoList } from "../hooks/use-video-list";

export const Route = createFileRoute("/videos")({
  component: VideosPage,
});

function VideosPage() {
  const { t } = useTranslation("common");
  const { videos, loading, error, hasMore, loadMore } = useVideoList();

  const dids = useMemo(
    () => Array.from(new Set(videos.map((v) => v.author.did))),
    [videos],
  );
  const avatars = useAvatars(dids);

  // Infinite scroll sentinel.
  const sentinelRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!sentinelRef.current) return;
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting) loadMore();
      },
      { rootMargin: "200px" },
    );
    observer.observe(sentinelRef.current);
    return () => observer.disconnect();
  }, [loadMore]);

  return (
    <div className="mx-auto max-w-[1600px] px-4 py-6">
      <h1 className="font-display mb-6 text-2xl font-semibold text-[var(--color-fg)]">
        {t("videos-title")}
      </h1>

      {videos.length > 0 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {videos.map((video) => (
            <VideoCard
              key={video.uri}
              video={video}
              avatarUrl={avatars[video.author.did]?.avatar}
            />
          ))}
        </div>
      )}

      {loading && videos.length === 0 && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="animate-pulse space-y-2">
              <div className="aspect-video rounded-xl bg-[var(--color-bg-elevated)]" />
              <div className="flex gap-2.5">
                <div className="h-8 w-8 rounded-full bg-[var(--color-bg-elevated)]" />
                <div className="flex-1 space-y-1.5">
                  <div className="h-3.5 w-3/4 rounded bg-[var(--color-bg-elevated)]" />
                  <div className="h-3 w-1/2 rounded bg-[var(--color-bg-elevated)]" />
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && videos.length === 0 && (
        <div className="py-20 text-center">
          <div className="mb-2 text-2xl">🎬</div>
          <p className="text-[var(--color-fg-muted)]">
            {error ? t("could-not-load-videos", { error }) : t("no-videos-yet")}
          </p>
        </div>
      )}

      {/* Infinite scroll sentinel */}
      {hasMore && <div ref={sentinelRef} className="h-1" />}
      {loading && videos.length > 0 && (
        <div className="flex justify-center py-8">
          <div className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--color-border)] border-t-[var(--color-accent)]" />
        </div>
      )}
    </div>
  );
}
