import { Text, useTheme, useTranslation, zero } from "@streamplace/components";
import { View } from "react-native";

// ──── or ────, between two ways to sign in.
export function OrDivider() {
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const line = (
    <View
      style={{ flex: 1, height: 1, backgroundColor: theme.colors.borderSubtle }}
    />
  );
  return (
    <View
      style={[
        zero.layout.flex.row,
        zero.layout.flex.alignCenter,
        zero.gap.all[3],
      ]}
    >
      {line}
      <Text size="sm" color="muted">
        {t("login-or")}
      </Text>
      {line}
    </View>
  );
}
