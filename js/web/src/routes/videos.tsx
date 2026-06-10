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
    <div className="max-w-[1600px] mx-auto px-4 py-6">
      <h1 className="text-2xl font-semibold font-display text-[var(--color-fg)] mb-6">
        {t("videos-title")}
      </h1>

      {videos.length > 0 && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
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
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className="animate-pulse space-y-2">
              <div className="aspect-video rounded-xl bg-[var(--color-bg-elevated)]" />
              <div className="flex gap-2.5">
                <div className="w-8 h-8 rounded-full bg-[var(--color-bg-elevated)]" />
                <div className="flex-1 space-y-1.5">
                  <div className="h-3.5 bg-[var(--color-bg-elevated)] rounded w-3/4" />
                  <div className="h-3 bg-[var(--color-bg-elevated)] rounded w-1/2" />
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {!loading && videos.length === 0 && (
        <div className="text-center py-20">
          <div className="text-2xl mb-2">🎬</div>
          <p className="text-[var(--color-fg-muted)]">
            {error ? t("could-not-load-videos", { error }) : t("no-videos-yet")}
          </p>
        </div>
      )}

      {/* Infinite scroll sentinel */}
      {hasMore && <div ref={sentinelRef} className="h-1" />}
      {loading && videos.length > 0 && (
        <div className="flex justify-center py-8">
          <div className="w-6 h-6 border-2 border-[var(--color-border)] border-t-[var(--color-accent)] rounded-full animate-spin" />
        </div>
      )}
    </div>
  );
}
