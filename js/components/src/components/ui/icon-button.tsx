import React, { forwardRef, useMemo } from "react";
import { Platform } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import { borderRadius, spacing, touchTargets } from "../../lib/theme/tokens";
import { ButtonPrimitive, ButtonPrimitiveProps } from "./primitives/button";

export interface IconButtonProps extends Omit<
  ButtonPrimitiveProps,
  "children"
> {
  children: React.ReactNode;
  /** ghost (default): transparent until hover. secondary: raised surface. */
  variant?: "ghost" | "secondary";
  /** sm 32px, md 40px, lg 48px square. Touch target stays ≥44px via hitSlop. */
  size?: "sm" | "md" | "lg";
  /** Fully rounded */
  round?: boolean;
  /** Required for accessibility — icon buttons have no visible label */
  accessibilityLabel: string;
}

const SIZES = {
  sm: spacing[8],
  md: spacing[10],
  lg: touchTargets.comfortable,
} as const;

/**
 * A square icon-only button. Ghost by default: quiet at rest, surface on
 * hover, never a spinner. Always give it an accessibilityLabel.
 */
export const IconButton = forwardRef<any, IconButtonProps>(
  (
    {
      children,
      variant = "ghost",
      size = "md",
      round = false,
      disabled,
      style,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();
    const side = SIZES[size];

    const variantStyles = useMemo(() => {
      const c = theme.colors;
      if (variant === "secondary") {
        return {
          base: {
            backgroundColor: c.surface2,
            borderWidth: 1,
            borderColor: c.border,
          },
          hover: {
            backgroundColor: c.surfaceHover,
            borderColor: c.borderStrong,
          },
          pressed: { backgroundColor: c.surface3 },
        };
      }
      return {
        base: { backgroundColor: "transparent", borderWidth: 0 },
        hover: { backgroundColor: c.surface2 },
        pressed: { backgroundColor: c.surface3 },
      };
    }, [variant, theme.colors]);

    const hitSlop =
      Platform.OS !== "web" && side < touchTargets.minimum
        ? (touchTargets.minimum - side) / 2
        : undefined;

    return (
      <ButtonPrimitive.Root
        ref={ref}
        disabled={disabled}
        hitSlop={hitSlop}
        style={
          [
            variantStyles.base,
            {
              width: side,
              height: side,
              borderRadius: round ? borderRadius.full : borderRadius.md,
              alignItems: "center",
              justifyContent: "center",
            },
            style,
          ] as any
        }
        hoverStyle={variantStyles.hover}
        pressedStyle={variantStyles.pressed}
        {...props}
      >
        {children}
      </ButtonPrimitive.Root>
    );
  },
);

IconButton.displayName = "IconButton";
