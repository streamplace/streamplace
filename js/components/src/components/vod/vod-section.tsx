import { ScrollView, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { spacing } from "../../lib/theme/tokens";
import { useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { VodComments } from "./vod-comments";
import { VodDescription } from "./vod-description";
import { VodMobileMetadata } from "./vod-mobile-metadata";

// Below-the-player detail column (YouTube grammar): title + channel/actions
// row, then the description card, then comments.
//
// scrollDescription (mobile) keeps the metadata header fixed and scrolls the
// rest, so the video above is always visible. Desktop flows in the surrounding
// scroll view.
export function VodSection({
  scrollDescription = false,
}: {
  scrollDescription?: boolean;
}) {
  const aturi = useVideoStore((x) => x.aturi);
  const video = useVideoStore((x) => x.video);
  const { theme } = useTheme();
  const insets = useSafeAreaInsets();

  if (!aturi) return null;
  const videoUri = video?.uri ?? aturi;

  if (!scrollDescription) {
    return (
      <View style={{ width: "100%", paddingTop: spacing[4] }}>
        {/* Full width — matches the player above. When related videos land,
            this becomes the left column with the related rail on the right. */}
        <View style={{ width: "100%", gap: spacing[5] }}>
          <VodMobileMetadata />
          <VodDescription />
          {video ? <VodComments videoUri={videoUri} /> : null}
        </View>
      </View>
    );
  }

  return (
    <View style={{ flex: 1, width: "100%" }}>
      <View
        style={{
          paddingHorizontal: spacing[4],
          paddingVertical: spacing[3],
          borderBottomWidth: 1,
          borderBottomColor: theme.colors.borderSubtle,
        }}
      >
        <VodMobileMetadata />
      </View>
      <ScrollView
        style={{ flex: 1 }}
        contentContainerStyle={{
          padding: spacing[4],
          paddingBottom: insets.bottom + spacing[6],
          gap: spacing[5],
        }}
        showsVerticalScrollIndicator={false}
      >
        <VodDescription />
        {video ? <VodComments videoUri={videoUri} /> : null}
      </ScrollView>
    </View>
  );
}
