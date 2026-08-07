// Video card for the VOD browse page. Web-native Tailwind port of
// js/app/components/video/video-card.tsx. Shows thumbnail, duration
// badge, avatar, title, handle, view/like count. Links to
// /$user/vod/:tid.

import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import type { VideoView } from "../../hooks/use-video-list";
import {
  formatDuration,
  getTidFromAtUri,
  getVideoThumbnailUrl,
} from "../../lib/video";

export function VideoCard({
  video,
  avatarUrl,
}: {
  video: VideoView;
  avatarUrl?: string;
}) {
  const { t } = useTranslation("common");
  const record = video.record as place.stream.video.Main;
  const author = video.author;
  const user = author.handle || author.did;
  const tid = getTidFromAtUri(video.uri);
  const title = record.title || t("untitled");
  const thumbnailUrl = getVideoThumbnailUrl(record, author.did);
  const duration = formatDuration(record.durationMs);
  const viewCount = video.viewCounts?.count ?? 0;
  const likeCount = video.likeCount ?? 0;

  return (
    <Link
      to="/$user/video/$tid"
      params={{ user, tid }}
      className="group flex flex-col gap-2"
    >
      {/* Thumbnail */}
      <div className="relative aspect-video overflow-hidden rounded-xl bg-(--color-bg-elevated)">
        {thumbnailUrl ? (
          <img
            src={thumbnailUrl}
            alt=""
            className="absolute inset-0 h-full w-full object-cover"
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center text-(--color-fg-muted)">
            <svg
              className="h-8 w-8 opacity-40"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={1.5}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="m15.75 10.5 4.72-4.72a.75.75 0 0 1 1.28.53v11.38a.75.75 0 0 1-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 0 0 2.25-2.25v-9a2.25 2.25 0 0 0-2.25-2.25h-9A2.25 2.25 0 0 0 2.25 7.5v9a2.25 2.25 0 0 0 2.25 2.25Z"
              />
            </svg>
          </div>
        )}
        {duration && (
          <div className="absolute right-1.5 bottom-1.5 rounded bg-black/80 px-1.5 py-0.5 text-xs font-medium text-white tabular-nums">
            {duration}
          </div>
        )}
      </div>

      {/* Metadata */}
      <div className="flex items-start gap-2.5 px-0.5">
        <div className="h-8 w-8 shrink-0 overflow-hidden rounded-full border border-(--color-border) bg-(--color-bg-elevated)">
          {avatarUrl ? (
            <img
              src={avatarUrl}
              alt=""
              className="h-full w-full object-cover"
              onError={(e) => {
                (e.currentTarget as HTMLImageElement).style.display = "none";
              }}
            />
          ) : (
            <div className="flex h-full w-full items-center justify-center text-xs font-medium text-(--color-fg-muted)">
              {user[0]?.toUpperCase() ?? "?"}
            </div>
          )}
        </div>
        <div className="min-w-0 flex-1">
          <div className="line-clamp-2 text-sm leading-tight font-semibold text-(--color-fg)">
            {title}
          </div>
          <div className="mt-0.5 truncate text-xs text-(--color-fg-muted)">
            @{user}
          </div>
          <div className="text-xs text-(--color-fg-muted)">
            {t("views-count", { count: viewCount })} ·{" "}
            {t("likes-count", { count: likeCount })}
          </div>
        </div>
      </div>
    </Link>
  );
}
