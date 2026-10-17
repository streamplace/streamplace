import { useEffect } from "react";
import { type DimensionValue, type ViewProps } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import { useTheme } from "../../lib/theme/theme";
import { borderRadius, motion } from "../../lib/theme/tokens";

export interface SkeletonProps extends ViewProps {
  /** rect: cards/thumbnails. circle: avatars. text: one line of copy. */
  shape?: "rect" | "circle" | "text";
  width?: DimensionValue;
  height?: DimensionValue;
  /** Corner radius token for rects */
  radius?: keyof typeof borderRadius;
}

/**
 * Loading placeholder for every async surface. A quiet opacity pulse —
 * no shimmer, no bounce. Match the skeleton's footprint to the loaded
 * content so nothing shifts when data arrives.
 */
export function Skeleton({
  shape = "rect",
  width,
  height,
  radius = "md",
  style,
  ...props
}: SkeletonProps) {
  const { theme } = useTheme();
  const pulse = useSharedValue(0.55);

  useEffect(() => {
    pulse.value = withRepeat(
      withTiming(1, {
        duration: motion.slow * 3,
        easing: Easing.bezier(...motion.bezier),
      }),
      -1,
      true,
    );
  }, [pulse]);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: pulse.value,
  }));

  const dims = (() => {
    switch (shape) {
      case "circle": {
        const side = width ?? height ?? 32;
        return {
          width: side,
          height: side,
          borderRadius: borderRadius.full,
        };
      }
      case "text":
        return {
          width: width ?? "100%",
          height: height ?? 14,
          borderRadius: borderRadius.sm,
        };
      default:
        return {
          width: width ?? "100%",
          height: height ?? 80,
          borderRadius: borderRadius[radius],
        };
    }
  })();

  return (
    <Animated.View
      style={[
        {
          backgroundColor: theme.colors.surface2,
          ...dims,
        },
        animatedStyle,
        style,
      ]}
      accessibilityElementsHidden
      importantForAccessibility="no-hide-descendants"
      {...props}
    />
  );
}
