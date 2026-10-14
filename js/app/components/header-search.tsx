import {
  breakpoints,
  spacing,
} from "@streamplace/components/src/lib/theme/tokens";
import { DRAWER_WIDTH } from "hooks/useSidebarControl";
import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Keyboard, Platform, Pressable, StyleSheet, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import UserSearch from "./user-search";

// A shell-level layer avoids navigator clipping and stays available while a
// screen chunk loads. Keep clear of the logo/navigation and account controls.
export default function HeaderSearch() {
  const [open, setOpen] = useState(false);
  const insets = useSafeAreaInsets();
  const { t } = useTranslation("common");

  const dismiss = () => {
    setOpen(false);
    if (Platform.OS !== "web") Keyboard.dismiss();
  };

  return (
    <View
      pointerEvents="box-none"
      style={[StyleSheet.absoluteFillObject, { zIndex: 128002 }]}
    >
      {open && (
        <Pressable
          testID="header-search-dismiss"
          accessibilityLabel={t("close")}
          onPress={dismiss}
          style={StyleSheet.absoluteFillObject}
        />
      )}
      <View
        pointerEvents="box-none"
        style={{
          position: "absolute",
          top: (Platform.OS !== "web" ? insets.top : 0) + spacing[1],
          left: DRAWER_WIDTH + spacing[4],
          right: DRAWER_WIDTH,
          alignItems: "center",
        }}
      >
        <View style={{ width: "100%", maxWidth: breakpoints.sm }}>
          <UserSearch variant="dropdown" open={open} onOpenChange={setOpen} />
        </View>
      </View>
    </View>
  );
}
