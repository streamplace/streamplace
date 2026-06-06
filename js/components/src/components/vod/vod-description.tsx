import { ScrollView, View } from "react-native";
import type { PlaceStreamVideo } from "streamplace";
import { hexToRgba, mt, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { Text } from "../ui/text";

export function VodDescription() {
  const video = useVideoStore((x) => x.video);
  const { theme } = useTheme();

  if (!video) return null;
  const record = video.record as unknown as PlaceStreamVideo.Record;
  const description = record.description?.trim();
  const tags = (record.tags as string[] | undefined) ?? [];

  return (
    <View style={[mt[3]]}>
      {tags.length > 0 ? (
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentContainerStyle={{ gap: 6, alignItems: "center" }}
          style={{ marginBottom: 12 }}
        >
          {tags.map((tag) => (
            <View
              key={tag}
              style={{
                borderRadius: 999,
                borderWidth: 1,
                borderColor: theme.colors.border,
                backgroundColor: hexToRgba(theme.colors.secondary, 0.3),
                paddingHorizontal: 8,
                paddingVertical: 2,
                flexShrink: 0,
              }}
            >
              <Text
                size="xs"
                color={hexToRgba(theme.colors.primaryForeground, 0.85)}
              >
                {tag}
              </Text>
            </View>
          ))}
        </ScrollView>
      ) : null}
      {description ? (
        <Text style={{ color: theme.colors.text }}>{description}</Text>
      ) : null}
    </View>
  );
}
