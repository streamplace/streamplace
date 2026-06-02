import { View } from "react-native";
import { px, py } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { VodMobileMetadata } from "./vod-mobile-metadata";

// VodSection is the VOD metadata block (title, author, likes, view count)
// rendered beneath the player at every breakpoint. The live-stream metadata
// bar (BottomMetadata) is suppressed for VOD, so this is the single source
// of VOD metadata across all widths.
export function VodSection() {
  const aturi = useVideoStore((x) => x.aturi);

  if (!aturi) {
    return null;
  }

  return (
    <View
      style={[
        px[4],
        py[4],
        { maxWidth: 720, alignSelf: "center" as const, width: "100%" },
      ]}
    >
      <VodMobileMetadata />
    </View>
  );
}
