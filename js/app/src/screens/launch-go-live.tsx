import { useNavigation } from "@react-navigation/native";
import {
  Button,
  Text,
  useBrandingAsset,
  useSocialShell,
  useStreamerLockedOut,
  useTheme,
  View,
  zero,
} from "@streamplace/components";
import { LandingTabs } from "components/landing/landing-tabs";
import { useOpenExternal } from "components/sidebar/sidebar-overlay";
import { RadioTower, Video } from "lucide-react-native";
import { Pressable } from "react-native";

export default function LaunchGoLive() {
  const navigation = useNavigation();
  const theme = useTheme();
  const social = useSocialShell();
  // A node that gates who may stream tells a viewer without the role who
  // can, instead of offering a stream it would refuse (branding keys
  // goLiveDeniedTitle / goLiveDeniedMessage, with the apply link).
  const lockedOut = useStreamerLockedOut();
  const deniedTitle =
    useBrandingAsset("goLiveDeniedTitle")?.data?.trim() ||
    "Do you want to go live?";
  const deniedMessage =
    useBrandingAsset("goLiveDeniedMessage")?.data?.trim() ||
    "Only selected accounts can go live here.";
  const applyUrl = useBrandingAsset("networkApplyUrl")?.data?.trim();
  const applyLinkLabel =
    useBrandingAsset("applyLinkLabel")?.data?.trim() || "Apply for access";
  const openExternal = useOpenExternal();

  const body = lockedOut ? (
    <View
      style={[
        zero.layout.flex.center,
        zero.h.percent[100],
        zero.gap.all[2],
        zero.px[6],
      ]}
    >
      <Text weight="semibold" center style={{ fontSize: 15, lineHeight: 23 }}>
        {deniedTitle}
      </Text>
      <Text
        center
        style={{
          fontSize: 15,
          lineHeight: 23,
          color: theme.theme.colors.text2,
          maxWidth: 520,
        }}
      >
        {deniedMessage}
        {applyUrl ? " " : ""}
        {applyUrl && (
          <Pressable onPress={() => openExternal(applyUrl)}>
            <Text
              style={{
                fontSize: 15,
                lineHeight: 23,
                color: theme.theme.colors.text1,
                textDecorationLine: "underline",
              }}
            >
              {applyLinkLabel}
            </Text>
          </Pressable>
        )}
      </Text>
    </View>
  ) : (
    <View
      style={[
        zero.layout.flex.center,
        zero.h.percent[100],
        zero.gap.all[6],
        zero.px[4],
      ]}
    >
      <View
        style={[
          zero.layout.flex.column,
          zero.gap.all[4],
          zero.layout.flex.alignCenter,
        ]}
      >
        <View style={[zero.r.full]}>
          <Video size={48} color={theme.theme.colors.text1} />
        </View>
        <Text size="2xl" weight="semibold" center>
          Ready to go live?
        </Text>
      </View>
      <View style={[zero.layout.flex.center]}>
        <Button
          size="lg"
          width="min"
          onPress={() => {
            navigation.navigate("MobileGoLive");
          }}
          leftIcon={<RadioTower color={theme.theme.colors.primaryForeground} />}
        >
          Start streaming
        </Button>
      </View>
    </View>
  );

  if (!social) return body;
  return (
    <View style={{ flex: 1 }}>
      <LandingTabs active="golive" />
      <View style={{ flex: 1 }}>{body}</View>
    </View>
  );
}
