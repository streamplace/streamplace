import { useWindowDimensions, View } from "react-native";
import { px, py } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { VodComments } from "./vod-comments";
import { VodMobileMetadata } from "./vod-mobile-metadata";

export function VodSection() {
  const aturi = useVideoStore((x) => x.aturi);
  const { width } = useWindowDimensions();
  const isNarrow = width < 768;

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
      {isNarrow && <VodMobileMetadata />}
      <VodComments videoUri={aturi} />
    </View>
  );
}
