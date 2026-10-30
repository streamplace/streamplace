import { type LucideProps } from "lucide-react-native";
import React from "react";
import { View } from "react-native";
import { useTheme } from "../../lib/theme/theme";

// Simple icon wrapper that integrates with theme
export interface IconProps {
  variant?:
    | "default"
    | "muted"
    | "primary"
    | "secondary"
    | "destructive"
    | "success"
    | "warning";
  size?: number | "sm" | "md" | "lg" | "xl";
  color?: LucideProps["color"];
}

// Size mapping
const sizeMap = {
  sm: 16,
  md: 20,
  lg: 24,
  xl: 32,
} as const;

type IconSource = React.ComponentType<LucideProps> | React.ReactElement<any>;

export function createThemedIcon(icon: IconSource): React.FC<IconProps> {
  return (props) => <Icon icon={icon} {...props} />;
}

export function Icon({
  icon,
  variant = "default",
  size = "md",
  color,
  ...restProps
}: { icon: IconSource } & IconProps) {
  const { theme } = useTheme();
  const iconSize = typeof size === "number" ? size : sizeMap[size];
  const iconColor =
    color || theme.colors[variant] || theme.colors.secondaryForeground;

  // Theme text colors carry alpha. Composite it once after the SVG paths
  // are drawn, so intersecting strokes keep the same visual weight.
  const rgba =
    typeof iconColor === "string"
      ? iconColor.match(/^(rgba\(.+),\s*([\d.]+)\)$/)
      : null;
  const props = {
    ...restProps,
    size: iconSize,
    color: rgba ? `${rgba[1]},1)` : iconColor,
  };

  return (
    <View
      needsOffscreenAlphaCompositing={rgba !== null}
      style={{ opacity: rgba ? Number(rgba[2]) : 1 }}
    >
      {React.isValidElement(icon)
        ? React.cloneElement(icon, props)
        : React.createElement(icon as React.ComponentType<LucideProps>, props)}
    </View>
  );
}
