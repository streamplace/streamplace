// Source - https://stackoverflow.com/a/74182982
// Posted by TOPKAT, modified by community. See post 'Timeline' for change history
// Retrieved 2026-02-18, License - CC BY-SA 4.0

import { DimensionValue, StyleSheet, View, ViewProps } from "react-native";
import Animated from "react-native-reanimated";
import Svg, { Defs, LinearGradient, Rect, Stop } from "react-native-svg";

type GradientProps = {
  fromColor: string;
  toColor: string;
  children?: any;
  height?: DimensionValue;
  opacityColor1?: number;
  opacityColor2?: number;
} & ViewProps;

function Gradient({
  children,
  fromColor,
  toColor,
  height = "100%",
  opacityColor1 = 1,
  opacityColor2 = 1,
  ...otherViewProps
}: GradientProps) {
  const gradientUniqueId = `grad${fromColor}+${toColor}`.replace(
    /[^a-zA-Z0-9 ]/g,
    "",
  );
  return (
    <>
      <View
        style={[
          {
            ...StyleSheet.absoluteFillObject,
            height,
            zIndex: -1,
            top: 0,
            left: 0,
          },
          otherViewProps.style,
        ]}
        {...otherViewProps}
      >
        <Svg height="100%" width="100%" style={StyleSheet.absoluteFillObject}>
          <Defs>
            <LinearGradient
              id={gradientUniqueId}
              x1="0%"
              y1="0%"
              x2="0%"
              y2="100%"
            >
              <Stop
                offset="0"
                stopColor={fromColor}
                stopOpacity={opacityColor1}
              />
              <Stop
                offset="1"
                stopColor={toColor}
                stopOpacity={opacityColor2}
              />
            </LinearGradient>
          </Defs>
          <Rect width="100%" height="100%" fill={`url(#${gradientUniqueId})`} />
        </Svg>
      </View>
      {children}
    </>
  );
}

export const AnimatedGradient = Animated.createAnimatedComponent(Gradient);

export default Gradient;
