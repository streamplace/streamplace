import React, { forwardRef, useEffect } from "react";
import { Text, View, type ViewProps } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import { useTheme } from "../../lib/theme/theme";
import {
  borderRadius,
  fontFamilies,
  fontWeights,
  spacing,
  tabularNums,
  typeScale,
} from "../../lib/theme/tokens";

export interface BadgeProps extends ViewProps {
  children?: React.ReactNode;
  /**
   * neutral: quiet metadata. accent: highlighted state. live: ON AIR only.
   * success/warning/danger: status.
   */
  variant?: "neutral" | "accent" | "live" | "success" | "warning" | "danger";
  size?: "sm" | "md";
}

/** Status pill. Solid fill for `live`, tinted quiet fills otherwise. */
export const Badge = forwardRef<View, BadgeProps>(
  ({ children, variant = "neutral", size = "sm", style, ...props }, ref) => {
    const { theme } = useTheme();
    const c = theme.colors;

    const colors = (() => {
      switch (variant) {
        case "live":
          return { bg: c.live, fg: c.liveForeground };
        case "accent":
          return { bg: c.surface2, fg: c.primary };
        case "success":
          return { bg: c.surface2, fg: c.success };
        case "warning":
          return { bg: c.surface2, fg: c.warning };
        case "danger":
          return { bg: c.surface2, fg: c.danger };
        default:
          return { bg: c.surface2, fg: c.text2 };
      }
    })();

    const isSm = size === "sm";

    return (
      <View
        ref={ref}
        style={[
          {
            flexDirection: "row",
            alignItems: "center",
            alignSelf: "flex-start",
            gap: spacing[1],
            backgroundColor: colors.bg,
            borderRadius: borderRadius.sm,
            paddingHorizontal: isSm ? spacing[1] + 2 : spacing[2],
            paddingVertical: isSm ? 2 : spacing[1],
          },
          style,
        ]}
        {...props}
      >
        {typeof children === "string" || typeof children === "number" ? (
          <Text
            style={{
              color: colors.fg,
              fontSize: typeScale.xs.fontSize,
              lineHeight: typeScale.xs.lineHeight,
              fontWeight: fontWeights.medium,
              fontFamily: fontFamilies.medium,
            }}
          >
            {children}
          </Text>
        ) : (
          children
        )}
      </View>
    );
  },
);

Badge.displayName = "Badge";

/** The pulsing on-air dot. Continuous by design — it signals liveness. */
export function LivePulseDot({ size = 6 }: { size?: number }) {
  const { theme } = useTheme();
  const pulse = useSharedValue(1);
  const fade = useSharedValue(0.9);

  useEffect(() => {
    pulse.value = withRepeat(withTiming(2, { duration: 1200 }), -1);
    fade.value = withRepeat(withTiming(0, { duration: 1200 }), -1);
  }, [pulse, fade]);

  const pulseStyle = useAnimatedStyle(() => ({
    transform: [{ scale: pulse.value }],
    opacity: fade.value,
  }));

  return (
    <View
      style={{
        width: size,
        height: size,
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <Animated.View
        style={[
          {
            position: "absolute",
            width: size,
            height: size,
            borderRadius: borderRadius.full,
            backgroundColor: theme.colors.liveForeground,
          },
          pulseStyle,
        ]}
      />
      <View
        style={{
          position: "absolute",
          width: size,
          height: size,
          borderRadius: borderRadius.full,
          backgroundColor: theme.colors.liveForeground,
        }}
      />
    </View>
  );
}

export interface LiveBadgeProps extends Omit<BadgeProps, "variant"> {
  /** Optional viewer count, rendered with tabular numerals */
  count?: number;
  /** Show the pulsing dot (default true) */
  dot?: boolean;
  label?: string;
}

/**
 * The LIVE badge. Reads instantly: live-red fill, uppercase label,
 * pulsing dot, tabular viewer count.
 */
export const LiveBadge = forwardRef<View, LiveBadgeProps>(
  ({ count, dot = true, label = "LIVE", size = "sm", ...props }, ref) => {
    const { theme } = useTheme();
    return (
      <Badge ref={ref} variant="live" size={size} {...props}>
        {dot && <LivePulseDot />}
        <Text
          style={{
            color: theme.colors.liveForeground,
            fontSize: typeScale.xs.fontSize,
            lineHeight: typeScale.xs.lineHeight,
            fontWeight: fontWeights.semibold,
            fontFamily: fontFamilies.semiBold,
            letterSpacing: 0.5,
          }}
        >
          {label}
        </Text>
        {count !== undefined && count > 0 && (
          <Text
            style={{
              color: theme.colors.liveForeground,
              fontSize: typeScale.xs.fontSize,
              lineHeight: typeScale.xs.lineHeight,
              fontWeight: fontWeights.medium,
              fontFamily: fontFamilies.medium,
              ...tabularNums,
            }}
          >
            {count}
          </Text>
        )}
      </Badge>
    );
  },
);

LiveBadge.displayName = "LiveBadge";
