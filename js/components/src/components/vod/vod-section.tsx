import { View } from "react-native";
import { borders, px, py, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { VodMobileMetadata } from "./vod-mobile-metadata";

// VodSection is the VOD metadata block (title, author, likes, view count)
// rendered beneath the player at every breakpoint. The live-stream metadata
// bar (BottomMetadata) is suppressed for VOD, so this is the single source
// of VOD metadata across all widths.
//
// The outer wrapper spans the full content width and carries the bottom
// divider so it runs edge to edge; the inner wrapper keeps the metadata
// itself centered at a readable max width.
export function VodSection() {
  const aturi = useVideoStore((x) => x.aturi);
  const { theme } = useTheme();

  if (!aturi) {
    return null;
  }

  return (
    <View
      style={[
        py[4],
        borders.bottom.width.thin,
        { borderBottomColor: theme.colors.border, width: "100%" },
      ]}
    >
      <View
        style={[
          px[4],
          { maxWidth: 720, alignSelf: "center" as const, width: "100%" },
        ]}
      >
        <VodMobileMetadata />
      </View>
    </View>
  );
}
