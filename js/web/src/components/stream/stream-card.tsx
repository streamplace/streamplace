// Stream card for the home feed. Web-native Tailwind port of
// js/app/components/home/cards.tsx (which uses LiquidGlassView,
// expo-image, and React Native). Shows thumbnail, viewer count,
// avatar, title, handle, activity label, and tags. Links to /$user.

import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import type { PlaceStreamDefs, PlaceStreamLivestream } from "streamplace";
import { formatViewers } from "../../lib/format";

const ACTIVITY_I18N_KEYS: Record<string, string> = {
  events: "activity-events",
  just_chatting: "activity-just-chatting",
  music: "activity-music",
  art: "activity-art",
  software_dev: "activity-software-dev",
  cooking: "activity-cooking",
  miniatures: "activity-miniatures",
  makers_crafting: "activity-makers-crafting",
  fitness: "activity-fitness",
  sports: "activity-sports",
};

function displayTag(tag: string): string {
  if (tag.startsWith("lang:")) {
    try {
      const langNames = new Intl.DisplayNames(["en"], { type: "language" });
      return langNames.of(tag.slice(5)) ?? tag;
    } catch {
      return tag;
    }
  }
  return tag;
}

export function getStreamActivity(
  record: PlaceStreamLivestream.Record,
  t: (key: string) => string,
): string | undefined {
  if (!record.activity) return undefined;
  const act = record.activity;
  if (act.$type === "place.stream.defs#activityGame") {
    return (act as PlaceStreamDefs.ActivityGame).name ?? undefined;
  }
  if (act.$type === "place.stream.defs#activityLabel") {
    const label = act as PlaceStreamDefs.ActivityLabel;
    return t(ACTIVITY_I18N_KEYS[label.label] ?? label.label);
  }
  return undefined;
}

interface StreamCardProps {
  stream: PlaceStreamLivestream.LivestreamView;
  avatarUrl?: string;
}

export function StreamCard({ stream, avatarUrl }: StreamCardProps) {
  const { t } = useTranslation("common");
  const record = stream.record as PlaceStreamLivestream.Record;
  const handle = stream.author.handle || stream.author.did;
  const title = record.title || t("default-stream-title");
  const activity = getStreamActivity(record, t);
  const tags = record.tags ?? [];
  const viewers = stream.viewerCount?.count;
  const user = handle;

  return (
    <Link
      to="/$user"
      params={{ user }}
      className="group flex flex-col overflow-hidden rounded-xl border border-[var(--color-border)] bg-[var(--color-bg-elevated)] transition-colors hover:border-[var(--color-border-strong)]"
    >
      {/* Thumbnail */}
      <div className="relative aspect-video bg-black">
        <img
          src={`/api/playback/${user}/stream.jpg?ts=${(Date.now() / 120000).toFixed(0)}`}
          alt=""
          className="absolute inset-0 h-full w-full object-cover"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.visibility = "hidden";
          }}
        />
        {/* Live dot */}
        <div className="absolute top-2 left-2 flex items-center gap-1.5 rounded bg-red-600 px-2 py-0.5 text-xs font-bold tracking-wide text-white uppercase">
          <div className="h-1.5 w-1.5 rounded-full bg-white" />
          {t("live-badge")}
        </div>
        {/* Viewer count */}
        {viewers !== undefined && viewers !== null && (
          <div className="absolute top-2 right-2 rounded bg-black/60 px-2 py-0.5 text-xs font-medium text-white backdrop-blur-sm">
            {formatViewers(viewers)}
          </div>
        )}
      </div>

      {/* Content */}
      <div className="flex items-start gap-3 px-3 py-2.5">
        {/* Avatar */}
        <div className="h-9 w-9 flex-shrink-0 overflow-hidden rounded-full border border-[var(--color-border)] bg-[var(--color-bg-overlay)]">
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
            <div className="flex h-full w-full items-center justify-center text-xs font-medium text-[var(--color-fg-muted)]">
              {handle[0]?.toUpperCase() ?? "?"}
            </div>
          )}
        </div>

        {/* Text */}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-[var(--color-fg)]">
            {title}
          </div>
          <div className="mt-0.5 truncate text-xs text-[var(--color-fg-muted)]">
            @{handle}
          </div>
          {(activity || tags.length > 0) && (
            <div className="mt-1.5 flex flex-wrap items-center gap-1.5 overflow-hidden">
              {activity && (
                <span className="flex-shrink-0 text-xs text-[var(--color-fg-muted)]">
                  {activity}
                </span>
              )}
              {tags.slice(0, 3).map((tag) => (
                <span
                  key={tag}
                  className="flex-shrink-0 rounded-full border border-[var(--color-border)] bg-[var(--color-bg-overlay)] px-2 py-0.5 text-xs text-[var(--color-fg-subtle)]"
                >
                  {displayTag(tag)}
                </span>
              ))}
            </div>
          )}
        </div>
      </div>
    </Link>
  );
}
