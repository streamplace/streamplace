import { ScrollView, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { borders, px, py, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { VodDescription } from "./vod-description";
import { VodMobileMetadata } from "./vod-mobile-metadata";

// VodSection is the VOD metadata block (title, author, likes, view count)
// plus the description below a divider line.
//
// With scrollDescription (mobile), the metadata header stays fixed and only the
// description scrolls, so the video above is always visible and a long
// description can't shrink the player. Without it (desktop), everything flows
// in the surrounding scroll view.
export function VodSection({
  scrollDescription = false,
}: {
  scrollDescription?: boolean;
}) {
  const aturi = useVideoStore((x) => x.aturi);
  const { theme } = useTheme();
  const insets = useSafeAreaInsets();

  if (!aturi) {
    return null;
  }

  const header = (
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

  const description = (
    <View
      style={[
        px[4],
        py[4],
        { maxWidth: 720, alignSelf: "center" as const, width: "100%" },
      ]}
    >
      <VodDescription />
    </View>
  );

  if (scrollDescription) {
    return (
      <View style={{ flex: 1, width: "100%" }}>
        {header}
        <ScrollView
          style={{ flex: 1 }}
          contentContainerStyle={{ paddingBottom: insets.bottom }}
          showsVerticalScrollIndicator={false}
        >
          {description}
        </ScrollView>
      </View>
    );
  }

  return (
    <View style={{ width: "100%" }}>
      {header}
      {description}
    </View>
  );
}
