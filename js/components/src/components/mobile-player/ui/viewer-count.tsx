import { useMemo } from "react";
import { StyleSheet, TextStyle } from "react-native";
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

  return (
    <Text leading="snug" style={[styles.label, style]}>
      {formattedNumber}
    </Text>
  );
}

const styles = StyleSheet.create({
  label: {
    color: "#fd5050",
    textShadowColor: "black",
    textShadowRadius: 3,
    fontSize: 16,
    lineHeight: 24,
  },
});

export default ViewerCount;
