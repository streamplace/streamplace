import { Text, View } from "@streamplace/components";
import {
  bg,
  borders,
  colors,
  flex,
  gap,
  layout,
  px,
  py,
  r,
} from "@streamplace/components/src/lib/theme/atoms";
import { STORE_LABELS, STORE_URLS } from "constants/store-urls";
import { Image } from "expo-image";
import usePlatform from "hooks/usePlatform.native";
import { X } from "lucide-react-native";
import { useState } from "react";
import { Linking, Pressable } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

const DISMISSED_KEY = "mobile_app_banner_dismissed";

export function MobileAppBanner() {
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(DISMISSED_KEY) === "1";
    } catch {
      return false;
    }
  });

  const insets = useSafeAreaInsets();
  const platform = usePlatform();

  if (
    dismissed ||
    !((platform.isWebIOS && !platform.isMobileSafari) || platform.isWebAndroid)
  )
    return null;

  const dismiss = () => {
    try {
      localStorage.setItem(DISMISSED_KEY, "1");
    } catch {}
    setDismissed(true);
  };

  const mobilePlatform = platform.isWebIOS ? "ios" : "android";

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        bg.gray[900],
        borders.bottom.width.thin,
        borders.bottom.color.gray[700],
        px[3],
        gap.all[2],
        { paddingTop: insets.top + 8, paddingBottom: 8 },
      ]}
    >
      <Image
        source={require("../assets/images/cube_small.png")}
        style={[{ width: 36, height: 36 }, r.md]}
      />
      <View style={[flex.values[1]]}>
        <Text weight="semibold" size="sm">
          Get the Streamplace app
        </Text>
        <Text size="xs" color="muted">
          Better experience on mobile
        </Text>
      </View>
      <Pressable
        onPress={() => Linking.openURL(STORE_URLS[mobilePlatform])}
        style={[bg.primary[500], r.md, px[3], py[1]]}
      >
        <Text size="sm" weight="semibold">
          {STORE_LABELS[mobilePlatform]}
        </Text>
      </Pressable>
      <Pressable onPress={dismiss} style={[py[1], px[1]]}>
        <X size={16} color={colors.gray[400]} />
      </Pressable>
    </View>
  );
}
