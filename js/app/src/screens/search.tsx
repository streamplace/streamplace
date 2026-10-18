import {
  breakpoints,
  spacing,
} from "@streamplace/components/src/lib/theme/tokens";
import UserSearch from "components/user-search";
import { Platform, ScrollView } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

export default function SearchScreen() {
  const safeAreaInsets = useSafeAreaInsets();

  return (
    <ScrollView
      keyboardShouldPersistTaps="handled"
      contentContainerStyle={{
        flexGrow: 1,
        gap: spacing[4],
        width: "100%",
        maxWidth: breakpoints.sm,
        alignSelf: "center",
        paddingHorizontal: spacing[4],
        // Same insets as the Videos tab root: clear the transparent iOS
        // header and the native tab bar.
        paddingTop:
          Platform.OS === "ios" ? spacing[16] + spacing[8] : spacing[4],
        paddingBottom:
          (Platform.OS !== "web" ? spacing[16] : 0) +
          safeAreaInsets.bottom +
          spacing[4],
      }}
    >
      <UserSearch variant="page" />
    </ScrollView>
  );
}
