import { View } from "react-native";
import type { PlaceStreamVideo } from "streamplace";
import { mt, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { Text } from "../ui/text";

// The video's description, shown below the metadata. For now this is just the
// plain text below the video; a collapsible dropdown (and richtext facets) can
// come later.
export function VodDescription() {
  const video = useVideoStore((x) => x.video);
  const { theme } = useTheme();

  if (!video) return null;
  const record = video.record as unknown as PlaceStreamVideo.Record;
  const description = record.description?.trim();
  if (!description) return null;

  return (
    <View style={[mt[3]]}>
      <Text style={{ color: theme.colors.text }}>{description}</Text>
    </View>
  );
}
