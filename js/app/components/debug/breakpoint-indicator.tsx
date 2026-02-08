import { Text, zero } from "@streamplace/components";
import { View, useWindowDimensions } from "react-native";

export default function BreakpointIndicator() {
  const { width } = useWindowDimensions();

  // Simple breakpoint detection based on width
  let current = "xs";
  if (width >= 1536) {
    current = "xxl";
  } else if (width >= 1280) {
    current = "xl";
  } else if (width >= 1024) {
    current = "lg";
  } else if (width >= 768) {
    current = "md";
  } else if (width >= 640) {
    current = "sm";
  }

  return (
    <View
      style={[
        zero.r.full,
        zero.bg.blue[600],
        zero.h[32],
        zero.w[32],
        { justifyContent: "center", alignItems: "center" },
      ]}
    >
      <Text style={[{ fontSize: 14 }, { color: "white" }]}>{current}</Text>
    </View>
  );
}
