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
      contentInsetAdjustmentBehavior="automatic"
      keyboardShouldPersistTaps="handled"
      contentContainerStyle={{
        flexGrow: 1,
        gap: spacing[4],
        width: "100%",
        maxWidth: breakpoints.sm,
        alignSelf: "center",
        paddingHorizontal: spacing[4],
        paddingTop: spacing[4],
        paddingBottom:
          Platform.OS === "ios"
            ? spacing[4]
            : (Platform.OS !== "web" ? spacing[16] : 0) +
              safeAreaInsets.bottom +
              spacing[4],
      }}
    >
      <UserSearch variant="page" />
    </ScrollView>
  );
}
