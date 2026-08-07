import { Button, Text, View } from "@streamplace/components";
import {
  bg,
  borders,
  flex,
  gap,
  layout,
  px,
  py,
} from "@streamplace/components/src/lib/theme/atoms";
import { textAlphas } from "@streamplace/components/src/lib/theme/tokens";
import { LogoTile } from "components/brand/logo";
import { STORE_LABELS, STORE_URLS } from "constants/store-urls";
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
      <LogoTile size={36} />
      <View style={[flex.values[1]]}>
        <Text weight="semibold" size="sm">
          Get the Streamplace app
        </Text>
        <Text size="xs" color="muted">
          Better experience on mobile
        </Text>
      </View>
      <Button
        size="sm"
        width="min"
        onPress={() => Linking.openURL(STORE_URLS[mobilePlatform])}
      >
        {STORE_LABELS[mobilePlatform]}
      </Button>
      <Pressable onPress={dismiss} style={[py[1], px[1]]}>
        <X size={16} color={textAlphas.dark[3]} />
      </Pressable>
    </View>
  );
}
