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
      className="group flex flex-col rounded-xl overflow-hidden border border-[var(--color-border)] bg-[var(--color-bg-elevated)] hover:border-[var(--color-border-strong)] transition-colors"
    >
      {/* Thumbnail */}
      <div className="relative aspect-video bg-black">
        <img
          src={`/api/playback/${user}/stream.jpg?ts=${(Date.now() / 120000).toFixed(0)}`}
          alt=""
          className="absolute inset-0 w-full h-full object-cover"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.visibility = "hidden";
          }}
        />
        {/* Live dot */}
        <div className="absolute top-2 left-2 flex items-center gap-1.5 bg-red-600 px-2 py-0.5 rounded text-white text-xs font-bold uppercase tracking-wide">
          <div className="w-1.5 h-1.5 rounded-full bg-white" />
          {t("live-badge")}
        </div>
        {/* Viewer count */}
        {viewers !== undefined && viewers !== null && (
          <div className="absolute top-2 right-2 bg-black/60 backdrop-blur-sm rounded px-2 py-0.5 text-white text-xs font-medium">
            {formatViewers(viewers)}
          </div>
        )}
      </div>

      {/* Content */}
      <div className="flex items-start gap-3 px-3 py-2.5">
        {/* Avatar */}
        <div className="w-9 h-9 rounded-full bg-[var(--color-bg-overlay)] border border-[var(--color-border)] overflow-hidden flex-shrink-0">
          {avatarUrl ? (
            <img
              src={avatarUrl}
              alt=""
              className="w-full h-full object-cover"
              onError={(e) => {
                (e.currentTarget as HTMLImageElement).style.display = "none";
              }}
            />
          ) : (
            <div className="w-full h-full flex items-center justify-center text-xs font-medium text-[var(--color-fg-muted)]">
              {handle[0]?.toUpperCase() ?? "?"}
            </div>
          )}
        </div>

        {/* Text */}
        <div className="flex-1 min-w-0">
          <div className="text-sm font-medium text-[var(--color-fg)] truncate">
            {title}
          </div>
          <div className="text-xs text-[var(--color-fg-muted)] truncate mt-0.5">
            @{handle}
          </div>
          {(activity || tags.length > 0) && (
            <div className="flex items-center gap-1.5 mt-1.5 flex-wrap overflow-hidden">
              {activity && (
                <span className="text-xs text-[var(--color-fg-muted)] flex-shrink-0">
                  {activity}
                </span>
              )}
              {tags.slice(0, 3).map((tag) => (
                <span
                  key={tag}
                  className="text-xs px-2 py-0.5 rounded-full bg-[var(--color-bg-overlay)] border border-[var(--color-border)] text-[var(--color-fg-subtle)] flex-shrink-0"
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
