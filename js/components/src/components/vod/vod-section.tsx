import { ScrollView, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { spacing } from "../../lib/theme/tokens";
import { useVideoStore } from "../../video-store/video-store";
import { VodComments } from "./vod-comments";
import { VodDescription } from "./vod-description";
import { VodMobileMetadata } from "./vod-mobile-metadata";

// Below-the-player detail column (YouTube grammar): title + channel/actions
// row, then the description card, then comments.
//
// Mobile scrolls the details beneath the fixed video. Desktop flows in the
// surrounding scroll view.
export function VodSection({
  scrollDescription = false,
}: {
  scrollDescription?: boolean;
}) {
  const aturi = useVideoStore((x) => x.aturi);
  const video = useVideoStore((x) => x.video);
  const insets = useSafeAreaInsets();

  if (!aturi) return null;
  const videoUri = video?.uri ?? aturi;

  if (!scrollDescription) {
    return (
      <View style={{ width: "100%", paddingTop: spacing[4] }}>
        {/* Full width — matches the player above. When related videos land,
            this becomes the left column with the related rail on the right. */}
        <View style={{ width: "100%", gap: spacing[4] }}>
          <VodMobileMetadata />
          <VodDescription />
          {video ? <VodComments videoUri={videoUri} /> : null}
        </View>
      </View>
    );
  }

  return (
    <ScrollView
      style={{ flex: 1 }}
      contentContainerStyle={{
        padding: spacing[4],
        paddingBottom: insets.bottom + spacing[6],
        gap: spacing[4],
      }}
      showsVerticalScrollIndicator={false}
    >
      <VodMobileMetadata />
      <VodDescription />
      {video ? <VodComments videoUri={videoUri} /> : null}
    </ScrollView>
  );
}
