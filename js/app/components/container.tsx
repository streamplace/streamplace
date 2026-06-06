import { zero } from "@streamplace/components";
import { View, ViewStyle, useWindowDimensions } from "react-native";

const maxContainerWidths = {
  xxs: 440,
  xs: 440,
  sm: 440,
  md: 660,
  lg: 740,
  xl: 800,
  twoXl: 1260,
  threeXl: 1660,
};

interface ContainerProps {
  children: React.ReactNode;
  style?: ViewStyle;
}

function getMaxWidth(width: number): number {
  if (width >= 1660) return maxContainerWidths.threeXl;
  if (width >= 1260) return maxContainerWidths.twoXl;
  if (width >= 800) return maxContainerWidths.xl;
  if (width >= 740) return maxContainerWidths.lg;
  if (width >= 660) return maxContainerWidths.md;
  if (width >= 440) return width;
  return maxContainerWidths.sm;
}

export default function Container({
  children,
  style,
  ...props
}: ContainerProps) {
  const { width } = useWindowDimensions();
  const maxWidth = getMaxWidth(width);
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
          isMobile ? zero.px[2] : zero.px[8],
          { marginHorizontal: "auto" },
          { maxWidth },
          style,
        ]}
        {...props}
      >
        {children}
      </View>
    </View>
  );
}
