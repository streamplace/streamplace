import { useMemo } from "react";
import { StyleSheet, TextStyle } from "react-native";
import { useTheme } from "../../ui";
import { Text } from "../../ui/text";

export interface ViewerCountProps {
  count?: number | null;
  style?: TextStyle;
  locales?: Intl.LocalesArgument;
  numberFormat?: Intl.NumberFormatOptions;
}

export function ViewerCount({
  count,
  style = {},
  locales,
  numberFormat = { notation: "compact" },
}: ViewerCountProps) {
  const formattedNumber = useMemo(() => {
    return new Intl.NumberFormat(locales, numberFormat).format(count || 0);
  }, [numberFormat, count]);
  const { theme } = useTheme();

  return (
    <Text
      leading="snug"
      style={[styles.label, { color: theme.colors.live }, style]}
    >
      {formattedNumber}
    </Text>
  );
}

const styles = StyleSheet.create({
  label: {
    textShadowColor: "black",
    textShadowRadius: 3,
    fontSize: 16,
    lineHeight: 24,
  },
});

export default ViewerCount;
