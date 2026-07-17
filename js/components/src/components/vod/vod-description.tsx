import { useState } from "react";
import { Pressable, View } from "react-native";
import type { PlaceStreamVideo } from "streamplace";
import { useViews } from "../../hooks/useViews";
import { spacing } from "../../lib/theme/tokens";
import { useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { Text } from "../ui/text";

// 11400 -> "11K views", 6300 -> "6.3K views", 942 -> "942 views"
function formatViews(n: number): string {
  const s =
    n >= 1_000_000
      ? `${(n / 1_000_000).toFixed(n < 10_000_000 ? 1 : 0)}M`
      : n >= 10_000
        ? `${Math.round(n / 1000)}K`
        : n >= 1000
          ? `${(n / 1000).toFixed(1)}K`
          : `${n}`;
  return `${s} ${n === 1 ? "view" : "views"}`;
}

function timeAgo(iso?: string): string | null {
  if (!iso) return null;
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return null;
  const secs = Math.max(0, (Date.now() - then) / 1000);
  const units: [number, string][] = [
    [31536000, "year"],
    [2592000, "month"],
    [604800, "week"],
    [86400, "day"],
    [3600, "hour"],
    [60, "minute"],
  ];
  for (const [size, label] of units) {
    const v = Math.floor(secs / size);
    if (v >= 1) return `${v} ${label}${v === 1 ? "" : "s"} ago`;
  }
  return "just now";
}

// YouTube-grammar description box: a quiet surface card that leads with
// "N views · posted", then the description, collapsed to a few lines with a
// "Show more" toggle. Tags sit at the bottom.
export function VodDescription() {
  const video = useVideoStore((x) => x.video);
  const views = useViews();
  const [expanded, setExpanded] = useState(false);
  const { theme } = useTheme();

  if (!video) return null;
  const record = video.record as unknown as PlaceStreamVideo.Record;
  const description = record.description?.trim();
  const tags = (record.tags as string[] | undefined) ?? [];
  const posted = timeAgo(record.createdAt as string | undefined);

  const meta = [views != null ? formatViews(views) : null, posted]
    .filter(Boolean)
    .join("  ·  ");

  const hasBody = !!description || tags.length > 0;

  const cardStyle = {
    backgroundColor: theme.colors.surface1,
    borderRadius: theme.borderRadius.lg,
    borderCurve: "continuous" as const,
    padding: spacing[3],
    gap: spacing[2],
  };

  const body = (
    <>
      {meta ? (
        <Text size="sm" weight="semibold">
          {meta}
        </Text>
      ) : null}

      {description ? (
        <Text
          size="sm"
          numberOfLines={expanded ? undefined : 2}
          style={{ color: theme.colors.text2 }}
        >
          {description}
        </Text>
      ) : null}

      {expanded && tags.length > 0 ? (
        <View
          style={{
            flexDirection: "row",
            flexWrap: "wrap",
            gap: spacing[2],
            marginTop: spacing[1],
          }}
        >
          {tags.map((tag) => (
            <View
              key={tag}
              style={{
                borderRadius: theme.borderRadius.full,
                borderWidth: 1,
                borderColor: theme.colors.borderSubtle,
                backgroundColor: theme.colors.surface2,
                paddingHorizontal: spacing[2],
                paddingVertical: 2,
              }}
            >
              <Text size="xs" color="muted">
                {tag}
              </Text>
            </View>
          ))}
        </View>
      ) : null}

      {hasBody && expanded ? (
        // Expanded: only this toggle collapses the card, so the description
        // stays selectable and clicks elsewhere don't close it.
        <Pressable
          onPress={() => setExpanded(false)}
          style={{ alignSelf: "flex-start" }}
        >
          <Text size="sm" weight="medium" style={{ color: theme.colors.text3 }}>
            Show less
          </Text>
        </Pressable>
      ) : hasBody ? (
        <Text size="sm" weight="medium" style={{ color: theme.colors.text3 }}>
          …more
        </Text>
      ) : null}
    </>
  );

  // Collapsed: the whole card is a tap target that expands. Expanded: the card
  // is inert — only "Show less" collapses it.
  if (hasBody && !expanded) {
    return (
      <Pressable onPress={() => setExpanded(true)} style={cardStyle}>
        {body}
      </Pressable>
    );
  }

  return <View style={cardStyle}>{body}</View>;
}
