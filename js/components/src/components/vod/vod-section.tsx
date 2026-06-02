import { useWindowDimensions, View } from "react-native";
import { px, py } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { VodMobileMetadata } from "./vod-mobile-metadata";

export function VodSection() {
  const aturi = useVideoStore((x) => x.aturi);
  const { width } = useWindowDimensions();
  const isNarrow = width < 768;

  // Wide layouts surface metadata through BottomMetadata, so the mobile
  // metadata block is all this section renders now.
  if (!aturi || !isNarrow) {
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
