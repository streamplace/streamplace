import { useLinkTo, useNavigation } from "@react-navigation/native";
import { VideoProvider, View, VodPlayer, zero } from "@streamplace/components";
import { colors, scrims } from "@streamplace/components/src/lib/theme/tokens";
import { Redirect } from "components/aqlink";
import { ChevronLeft } from "lucide-react-native";
import { Pressable } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

// A pared-down VOD route (/:user/vod/:tid) that drops the unified live+vod
// player in favor of the minimal <VodPlayer>. VideoProvider supplies the video
// context (title/author/etc.) for anything we want to render around it; the
// player itself is just a thin wrapper over the <Video> element.
export default function VodScreen({
  route,
}: {
  route?: { params?: { user?: string; tid?: string } };
}) {
  const navigation = useNavigation();
  const linkTo = useLinkTo();
  const insets = useSafeAreaInsets();

  if (!route?.params?.user || !route?.params?.tid) {
    return <Redirect to={{ screen: "HomeMain" }} />;
  }
  const aturi = `at://${route.params.user}/place.stream.video/${route.params.tid}`;

  const goBack = () => {
    if (navigation.canGoBack()) {
      navigation.goBack();
    } else {
      // Reached directly (deep link / share), so there's nothing to pop —
      // fall back to the video listing.
      linkTo("/video");
    }
  };

  return (
    <VideoProvider aturi={aturi}>
      <View style={[zero.flex.values[1], { backgroundColor: colors.black }]}>
        {/* Pad the bottom by the safe-area inset so expo-video's native
            scrubber sits above the Android nav buttons / iOS home indicator
            (the app draws edge-to-edge, so without this the bar is
            unreachable). */}
        <View style={{ flex: 1, paddingBottom: insets.bottom }}>
          <VodPlayer src={aturi} />
        </View>

        {/* Back affordance — the player screen is otherwise chrome-less. */}
        <Pressable
          onPress={goBack}
          hitSlop={12}
          style={{
            position: "absolute",
            top: insets.top + 8,
            left: 12,
            width: 40,
            height: 40,
            borderRadius: 20,
            alignItems: "center",
            justifyContent: "center",
            backgroundColor: scrims.dark,
            zIndex: 10,
            elevation: 10,
          }}
        >
          <ChevronLeft size={26} color={colors.white} />
        </Pressable>
      </View>
    </VideoProvider>
  );
}
