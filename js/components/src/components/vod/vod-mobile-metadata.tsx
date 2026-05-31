import { View } from "react-native";
import { zero } from "../..";
import { useTitle } from "../../hooks/useTitle";
import { borders, gap, layout, mb, pb, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { Text } from "../ui/text";
import { LikeButton } from "./like-button";

export function VodMobileMetadata() {
  const video = useVideoStore((x) => x.video);
  const aturi = useVideoStore((x) => x.aturi);
  const title = useTitle();
  const { theme } = useTheme();

  if (!video || !aturi) return null;

  const author = video.author;
  const viewCount = video.viewCounts?.count ?? 0;

  return (
    <View
      style={[
        pb[3],
        borders.bottom.width.thin,
        { borderBottomColor: theme.colors.border },
        mb[3],
        zero.layout.flex.direction.row,
        zero.layout.flex.justify.between,
      ]}
    >
      <View
        style={[layout.flex.row, layout.flex.alignCenter, gap.all[2], mb[2]]}
      >
        <View style={zero.flex[1]}>
          <Text weight="semibold" numberOfLines={1}>
            {title || "Untitled"}
          </Text>
          <Text
            size="sm"
            style={{ color: theme.colors.textMuted }}
            numberOfLines={1}
          >
            {author.handle ? `@${author.handle}` : author.did}
          </Text>
        </View>
      </View>

      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[4]]}>
        <LikeButton subjectUri={aturi} />
        <Text size="sm" style={{ color: theme.colors.textMuted }}>
          {viewCount.toLocaleString()} view{viewCount !== 1 ? "s" : ""}
        </Text>
      </View>
    </View>
  );
}
