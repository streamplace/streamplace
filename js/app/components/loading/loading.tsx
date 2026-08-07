import { useTheme, zero } from "@streamplace/components";
import { ActivityIndicator, View } from "react-native";

export default function () {
  return (
    <View
      style={[
        zero.flex.values[1],
        { justifyContent: "center", alignItems: "center" },
      ]}
    >
      <Spinner />
    </View>
  );
}

export function Spinner() {
  const { theme } = useTheme();
  return <ActivityIndicator size="large" color={theme.colors.primary} />;
}
