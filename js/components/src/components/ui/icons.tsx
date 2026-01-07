import { type LucideProps } from "lucide-react-native";
import React from "react";
import { useTheme } from "../../lib/theme";

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
  color?: string;
}

// Size mapping
const sizeMap = {
  sm: 16,
  md: 20,
  lg: 24,
  xl: 32,
} as const;

// HOC to create themed icons
export function createThemedIcon(
  IconComponent: React.ComponentType<LucideProps>,
): React.FC<IconProps> {
  return ({ variant = "default", size = "md", color, ...restProps }) => {
    let theme = useTheme(); // Ensure theme is available
    // Calculate size
    const iconSize = typeof size === "number" ? size : sizeMap[size];

    // Calculate color if not provided using atoms
    const iconColor =
      color ||
      theme.theme.colors[variant] ||
      theme.theme.colors.secondaryForeground;

    return (
      <IconComponent
        size={iconSize}
        color={iconColor}
        {...(restProps as Omit<LucideProps, "size" | "color">)}
      />
    );
  };
}

// usage of createThemedIcon
export function Icon({
  icon,
  variant = "default",
  size = "md",
  color,
  ...restProps
}: { icon: React.ComponentType<LucideProps> } & IconProps) {
  const ThemedIcon = createThemedIcon(icon);
  return (
    <ThemedIcon variant={variant} size={size} color={color} {...restProps} />
  );
}
