import { zero } from "@streamplace/components";
import { spacing } from "@streamplace/components/src/lib/theme/tokens";
import { View, ViewStyle, useWindowDimensions } from "react-native";

// Content well: fluid with comfortable gutters, capped for very wide
// displays so line lengths and grids stay composed.
const MAX_CONTENT_WIDTH = 1680;

interface ContainerProps {
  children: React.ReactNode;
  style?: ViewStyle;
}

export default function Container({
  children,
  style,
  ...props
}: ContainerProps) {
  const { width } = useWindowDimensions();
  const isMobile = width < 768;

  return (
    <View
      style={[
        zero.flex.values[1],
        { justifyContent: "center" },
        { alignItems: "center" },
      ]}
    >
      <View
        style={[
          zero.w.percent[100],
          { paddingHorizontal: isMobile ? spacing[4] : spacing[6] },
          { marginHorizontal: "auto" },
          { maxWidth: MAX_CONTENT_WIDTH },
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    </View>
  );
}
