import { Text, useTheme, zero } from "@streamplace/components";
import {
  borderRadius,
  colors,
  motion,
  scrims,
  spacing,
  tabularNums,
} from "@streamplace/components/src/lib/theme/tokens";
import { formatHandle, formatHandleWithAt } from "@streamplace/core";
import { Image } from "expo-image";
import { VideoView } from "hooks/useVideoList";
import { View } from "react-native";
import { place } from "streamplace";
import {
  formatDuration,
  getTidFromAtUri,
  getVideoThumbnailUrl,
} from "utils/video";
import AQLink from "../aqlink";

function formatCount(count: number): string {
  if (count >= 1_000_000) {
    return `${(count / 1_000_000).toFixed(count >= 10_000_000 ? 0 : 1)}M`;
  }
  if (count >= 1_000) {
    return `${(count / 1_000).toFixed(count >= 10_000 ? 0 : 1)}K`;
  }
  return `${count}`;
}

export default function VideoCard({
  video,
  avatarUrl,
  compact = false,
}: {
  video: VideoView;
  avatarUrl?: string;
  compact?: boolean;
}) {
  const { theme } = useTheme();
  const record = video.record as place.stream.video.Main;
  const author = video.author;
  const user = formatHandle(author);
  const tid = getTidFromAtUri(video.uri);
  const title = record.title || "Untitled";
  const thumbnailUrl = getVideoThumbnailUrl(record, author.did);
  const duration = formatDuration(record.durationMs);
  const viewCount = video.viewCounts?.count ?? 0;
  const likeCount = video.likeCount ?? 0;

  return (
    <AQLink
      testID="video-card"
      to={{ screen: "Video", params: { user, tid } }}
      style={{ flex: 1 }}
    >
      <View
        style={[
          zero.flex.values[1],
          zero.gap.all[3],
          compact && { flexDirection: "row", alignItems: "flex-start" },
        ]}
      >
        {/* Thumbnail */}
        <View
          style={{
            width: compact ? "40%" : "100%",
            aspectRatio: 16 / 9,
            borderRadius: borderRadius.lg,
            borderCurve: "continuous",
            overflow: "hidden",
            backgroundColor: theme.colors.muted,
            position: "relative",
          }}
        >
          {thumbnailUrl ? (
            <Image
              source={{ uri: thumbnailUrl }}
              style={{ width: "100%", height: "100%" }}
              contentFit="cover"
              transition={motion.fast}
            />
          ) : (
            <Image
              source={require("../../assets/images/jelly.png")}
              style={{ width: "100%", height: "100%" }}
              contentFit="contain"
            />
          )}
          {duration && (
            <View
              style={{
                position: "absolute",
                bottom: spacing[1],
                right: spacing[1],
                backgroundColor: scrims.dark,
                borderRadius: borderRadius.sm,
                paddingHorizontal: spacing[1],
              }}
            >
              <Text size="xs" style={[{ color: colors.white }, tabularNums]}>
                {duration}
              </Text>
            </View>
          )}
        </View>

        {/* Metadata */}
        <View
          style={[
            zero.layout.flex.row,
            zero.gap.all[3],
            { flex: 1, minWidth: 0 },
          ]}
        >
          {!compact && (
            <View
              style={{
                width: 36,
                height: 36,
                borderRadius: 18,
                overflow: "hidden",
                flexShrink: 0,
                backgroundColor: theme.colors.muted,
              }}
            >
              {avatarUrl ? (
                <Image
                  source={{ uri: avatarUrl }}
                  style={{ width: "100%", height: "100%" }}
                  contentFit="cover"
                />
              ) : (
                <Image
                  source={require("../../assets/images/goose.png")}
                  style={{ width: "100%", height: "100%" }}
                  contentFit="cover"
                />
              )}
            </View>
          )}
          <View style={[zero.flex.values[1], { minWidth: 0 }]}>
            <Text weight="semibold" numberOfLines={2}>
              {title}
            </Text>
            <Text
              size="sm"
              style={{ color: theme.colors.textMuted }}
              numberOfLines={1}
            >
              {formatHandleWithAt(author)}
            </Text>
            {(viewCount > 0 || likeCount > 0) && (
              <Text
                size="xs"
                style={[{ color: theme.colors.text3 }, tabularNums]}
              >
                {viewCount > 0 &&
                  `${formatCount(viewCount)} view${viewCount === 1 ? "" : "s"}`}
                {viewCount > 0 && likeCount > 0 && " · "}
                {likeCount > 0 &&
                  `${formatCount(likeCount)} like${likeCount === 1 ? "" : "s"}`}
              </Text>
            )}
          </View>
        </View>
      </View>
    </AQLink>
  );
}
