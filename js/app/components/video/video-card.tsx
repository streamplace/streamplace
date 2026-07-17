import { Text, useTheme, zero } from "@streamplace/components";
import {
  colors,
  motion,
  scrims,
} from "@streamplace/components/src/lib/theme/tokens";
import { Image } from "expo-image";
import { VideoView } from "hooks/useVideoList";
import { View } from "react-native";
import { PlaceStreamVideo } from "streamplace";
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
}: {
  video: VideoView;
  avatarUrl?: string;
}) {
  const { theme } = useTheme();
  const record = video.record as PlaceStreamVideo.Record;
  const author = video.author;
  const user = author.handle || author.did;
  const tid = getTidFromAtUri(video.uri);
  const title = record.title || "Untitled";
  const thumbnailUrl = getVideoThumbnailUrl(record, author.did);
  const duration = formatDuration(record.durationMs);
  const viewCount = video.viewCounts?.count ?? 0;
  const likeCount = video.likeCount ?? 0;

  return (
    <AQLink to={{ screen: "Video", params: { user, tid } }} style={{ flex: 1 }}>
      <View style={[zero.flex.values[1], zero.gap.all[2]]}>
        {/* Thumbnail */}
        <View
          style={{
            width: "100%",
            aspectRatio: 16 / 9,
            borderRadius: 12,
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
                bottom: 6,
                right: 6,
                backgroundColor: scrims.dark,
                borderRadius: 4,
                paddingHorizontal: 4,
                paddingVertical: 1,
              }}
            >
              <Text
                size="xs"
                style={{ color: colors.white, fontVariant: ["tabular-nums"] }}
              >
                {duration}
              </Text>
            </View>
          )}
        </View>

        {/* Metadata */}
        <View
          style={[zero.layout.flex.row, zero.gap.all[3], { paddingRight: 4 }]}
        >
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
          <View style={[zero.flex.values[1], { minWidth: 0 }]}>
            <Text
              weight="semibold"
              numberOfLines={2}
              style={{ lineHeight: 18 }}
            >
              {title}
            </Text>
            <Text
              size="sm"
              style={{ color: theme.colors.textMuted }}
              numberOfLines={1}
            >
              @{user}
            </Text>
            <Text size="sm" style={{ color: theme.colors.textMuted }}>
              {formatCount(viewCount)} view{viewCount === 1 ? "" : "s"} ·{" "}
              {formatCount(likeCount)} like{likeCount === 1 ? "" : "s"}
            </Text>
          </View>
        </View>
      </View>
    </AQLink>
  );
}
