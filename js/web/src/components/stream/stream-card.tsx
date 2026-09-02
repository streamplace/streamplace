// Stream card for the home feed. Web-native Tailwind port of
// js/app/components/home/cards.tsx (which uses LiquidGlassView,
// expo-image, and React Native). Shows thumbnail, viewer count,
// avatar, title, handle, activity label, and tags. Links to /$user.

import { Link } from "@tanstack/react-router";
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type CSSProperties,
} from "react";
import { useTranslation } from "react-i18next";
import { place } from "streamplace";
import { getDidAccentColor } from "../../lib/color";
import { formatViewers, isPositiveCount } from "../../lib/format";
import { useStreamplaceUrl } from "../../lib/store/hooks";

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
  record: place.stream.livestream.Main,
  t: (key: string) => string,
): string | undefined {
  if (!record.activity) return undefined;
  const act = record.activity;
  if (act.$type === "place.stream.defs#activityGame") {
    return (act as place.stream.defs.ActivityGame).name ?? undefined;
  }
  if (act.$type === "place.stream.defs#activityLabel") {
    const label = act as place.stream.defs.ActivityLabel;
    return t(ACTIVITY_I18N_KEYS[label.label] ?? label.label);
  }
  return undefined;
}

interface StreamCardProps {
  stream: place.stream.livestream.LivestreamView;
  avatarUrl?: string;
}

export function StreamCard({ stream, avatarUrl }: StreamCardProps) {
  const { t } = useTranslation("common");
  const streamplaceUrl = useStreamplaceUrl();
  const record = stream.record as place.stream.livestream.Main;
  const handle = stream.author.handle || stream.author.did;
  const title = record.title || t("default-stream-title");
  const activity = getStreamActivity(record, t);
  const tags = record.tags ?? [];
  const viewers = stream.viewerCount?.count;
  const user = handle;

  // Keep a streamer's accent stable even if their handle changes.
  const borderColor = useMemo(
    () => getDidAccentColor(stream.author.did),
    [stream.author.did],
  );

  // Bounds-measuring one-row tag layout. Same approach as the app's
  // StreamCard: each item reports its measured width via a ref, and
  // we compute how many tags fit in the row's available width. Items
  // past the limit are not rendered, so the row is always exactly
  // one line tall. Widths are reset when activity or tags change;
  // rowWidth is re-measured via ResizeObserver when the card resizes.
  const tagsRowRef = useRef<HTMLDivElement>(null);
  const [rowWidth, setRowWidth] = useState(0);
  const [itemWidths, setItemWidths] = useState<Record<string, number>>({});

  const tagsKey = useMemo(() => tags.join(","), [tags]);

  useEffect(() => {
    setItemWidths({});
  }, [activity, tagsKey]);

  useLayoutEffect(() => {
    const el = tagsRowRef.current;
    if (!el) return;
    const update = () => setRowWidth(el.offsetWidth);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  const visibleTagCount = useMemo(() => {
    if (rowWidth === 0) return tags.length;
    const activityW = activity ? (itemWidths["activity"] ?? 0) : 0;
    let used = activityW;
    let count = 0;
    for (let i = 0; i < tags.length; i++) {
      const w = itemWidths[`tag-${i}`];
      if (w === undefined) {
        // Not measured yet
        count++;
        continue;
      }
      const gap = used > 0 ? 6 : 0; // matches gap-1.5 (0.375rem = 6px)
      // Strict < (not <=) so a tag that fits with exactly 0px to spare
      // doesn't get clipped mid-word by the row's overflow-hidden.
      if (used + gap + w < rowWidth) {
        used += gap + w;
        count++;
      } else {
        break;
      }
    }
    return count;
  }, [rowWidth, itemWidths, tags, activity]);

  // Ref callback for measuring an item's width
  const measureItem = (key: string, el: HTMLElement | null) => {
    if (el && el.offsetWidth > 0 && el.offsetWidth !== itemWidths[key]) {
      setItemWidths((prev) => ({ ...prev, [key]: el.offsetWidth }));
    }
  };

  return (
    <Link
      to="/$user"
      params={{ user }}
      className="group flex flex-col focus-visible:rounded-xl focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-(--color-accent)"
    >
      {/* Thumbnail */}
      <div
        className="outline-border/40 relative aspect-video overflow-clip rounded-xl bg-black ring-0 outline transition-all duration-200 group-hover:rounded-md group-hover:ring-4"
        style={{ "--tw-ring-color": borderColor } as CSSProperties}
      >
        <img
          src={`${streamplaceUrl}/api/playback/${user}/stream.jpg?ts=${(Date.now() / 120000).toFixed(0)}`}
          alt=""
          className="absolute inset-0 h-full w-full object-cover opacity-50 blur-sm"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.visibility = "hidden";
          }}
        />
        <img
          src={`${streamplaceUrl}/api/playback/${user}/stream.jpg?ts=${(Date.now() / 120000).toFixed(0)}`}
          alt=""
          className="absolute inset-0 h-full w-full object-contain"
          onError={(e) => {
            (e.currentTarget as HTMLImageElement).style.visibility = "hidden";
          }}
        />
        {/* Live dot */}
        <div className="absolute top-2 left-2 flex items-center gap-1.5 rounded bg-red-600 px-2 py-0.5 text-xs font-semibold text-white uppercase">
          <div className="h-1.5 w-1.5 rounded-full bg-white" />
          {t("live-badge")}
        </div>
        {/* Viewer count */}
        {isPositiveCount(viewers) && (
          <div className="absolute top-2 right-2 rounded bg-black/60 px-2 py-0.5 text-xs font-medium text-white backdrop-blur-sm">
            {formatViewers(viewers)}
          </div>
        )}
      </div>

      {/* Content */}
      <div className="mt-3 flex items-start gap-3">
        {/* Avatar */}
        <div className="h-9 w-9 shrink-0 overflow-hidden rounded-full border border-(--color-border) bg-(--color-bg-overlay)">
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
              {handle[0]?.toUpperCase() ?? "?"}
            </div>
          )}
        </div>

        {/* Text */}
        <div className="min-w-0 flex-1">
          <div className="truncate text-sm font-medium text-(--color-fg) transition-colors group-hover:text-(--color-accent)">
            {title}
          </div>
          <div className="truncate text-sm text-(--color-fg-muted)">
            @{handle}
          </div>
          {(activity || tags.length > 0) && (
            <div
              ref={tagsRowRef}
              className="mt-0.5 flex items-center gap-1.5 overflow-hidden whitespace-nowrap"
            >
              {activity && (
                <span
                  ref={(el) => measureItem("activity", el)}
                  className="shrink-0 truncate text-xs text-(--color-fg-muted)"
                >
                  {activity}
                </span>
              )}
              {tags.slice(0, visibleTagCount).map((tag, index) => (
                <span
                  key={tag}
                  ref={(el) => measureItem(`tag-${index}`, el)}
                  className="shrink-0 truncate rounded-full border border-(--color-border) bg-(--color-bg-overlay) px-2 py-0.5 text-xs text-(--color-fg-subtle)"
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
