// Source - https://stackoverflow.com/a/74182982
// Posted by TOPKAT, modified by community. See post 'Timeline' for change history
// Retrieved 2026-02-18, License - CC BY-SA 4.0

import { useState } from "react";
import { StyleSheet, View, ViewProps } from "react-native";
import Animated from "react-native-reanimated";
import Svg, {
  Defs,
  LinearGradient,
  NumberProp,
  Rect,
  Stop,
} from "react-native-svg";

type GradientProps = {
  fromColor: string;
  toColor: string;
  children?: any;
  height?: NumberProp;
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
  style,
  onLayout,
  ...otherViewProps
}: GradientProps) {
  const [size, setSize] = useState({ width: 0, height: 0 });
  const gradientHeight = height === "100%" ? size.height : height;
  const gradientUniqueId = `grad${fromColor}+${toColor}`.replace(
    /[^a-zA-Z0-9 ]/g,
    "",
  );
  return (
    <View
      style={[{ position: "relative", zIndex: 0 }, style]}
      onLayout={(event) => {
        const { width, height } = event.nativeEvent.layout;
        setSize((previous) =>
          previous.width === width && previous.height === height
            ? previous
            : { width, height },
        );
        onLayout?.(event);
      }}
      {...otherViewProps}
    >
      <Svg
        pointerEvents="none"
        height={gradientHeight}
        width="100%"
        style={[StyleSheet.absoluteFill, { zIndex: -1 }]}
      >
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
            <Stop offset="1" stopColor={toColor} stopOpacity={opacityColor2} />
          </LinearGradient>
        </Defs>
        <Rect
          width={size.width}
          height={gradientHeight}
          fill={`url(#${gradientUniqueId})`}
        />
      </Svg>
      {children}
    </View>
  );
}

export const AnimatedGradient = Animated.createAnimatedComponent(Gradient);

export default Gradient;
