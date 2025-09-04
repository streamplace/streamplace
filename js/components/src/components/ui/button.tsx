import { cva, type VariantProps } from "class-variance-authority";
import React, { forwardRef, useMemo } from "react";
import { ActivityIndicator, StyleSheet } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import * as tokens from "../../lib/theme/tokens";
import { ButtonPrimitive, ButtonPrimitiveProps } from "./primitives/button";
import { TextPrimitive } from "./primitives/text";

// Button variants using class-variance-authority pattern
const buttonVariants = cva("", {
  variants: {
    variant: {
      primary: "primary",
      secondary: "secondary",
      outline: "outline",
      ghost: "ghost",
      destructive: "destructive",
      success: "success",
    },
    size: {
      sm: "sm",
      md: "md",
      lg: "lg",
      xl: "xl",
      pill: "pill",
    },
  },
  defaultVariants: {
    variant: "primary",
    size: "md",
  },
});

export interface ButtonProps
  extends Omit<ButtonPrimitiveProps, "children">,
    VariantProps<typeof buttonVariants> {
  children?: React.ReactNode;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
  loading?: boolean;
  loadingText?: string;
}

export const Button = forwardRef<any, ButtonProps>(
  (
    {
      variant = "primary",
      size = "md",
      children,
      leftIcon,
      rightIcon,
      loading = false,
      loadingText,
      disabled,
      style,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();

    // Create dynamic styles based on theme
    const styles = useMemo(() => createStyles(theme), [theme]);

    // Get variant styles
    const buttonStyle = useMemo(() => {
      const variantStyle = styles[`${variant}Button` as keyof typeof styles];
      const sizeStyle = styles[`${size}Button` as keyof typeof styles];
      return [variantStyle, sizeStyle];
    }, [variant, size, styles]);

    // Get inner styles for button content
    const buttonInnerStyle = useMemo(() => {
      const sizeInnerStyle =
        styles[`${size}ButtonInner` as keyof typeof styles];
      return sizeInnerStyle;
    }, [size, styles]);

    const textStyle = React.useMemo(() => {
      const variantTextStyle = styles[`${variant}Text` as keyof typeof styles];
      const sizeTextStyle = styles[`${size}Text` as keyof typeof styles];
      return [variantTextStyle, sizeTextStyle];
    }, [variant, size, styles]);

    const iconSize = React.useMemo(() => {
      switch (size) {
        case "sm":
          return 16;
        case "lg":
          return 20;
        case "xl":
          return 24;
        case "md":
        default:
          return 18;
      }
    }, [size]);

    const spinnerSize = useMemo(() => {
      switch (size) {
        case "sm":
          return "small" as const;
        case "lg":
        case "xl":
          return "large" as const;
        case "md":
        default:
          return "small" as const;
      }
    }, [size]);

    const spinnerColor = useMemo(() => {
      switch (variant) {
        case "outline":
        case "ghost":
          return theme.colors.primary;
        case "secondary":
          return theme.colors.secondaryForeground;
        case "destructive":
          return theme.colors.destructiveForeground;
        default:
          return theme.colors.primaryForeground;
      }
    }, [variant, theme.colors]);

    return (
      <ButtonPrimitive.Root
        ref={ref}
        disabled={disabled || loading}
        style={[buttonStyle, style]}
        {...props}
      >
        <ButtonPrimitive.Content style={buttonInnerStyle}>
          {loading && !leftIcon ? (
            <ButtonPrimitive.Icon position="left">
              <ActivityIndicator size={spinnerSize} color={spinnerColor} />
            </ButtonPrimitive.Icon>
          ) : leftIcon ? (
            <ButtonPrimitive.Icon
              position="left"
              style={{ width: iconSize, height: iconSize }}
            >
              {leftIcon}
            </ButtonPrimitive.Icon>
          ) : null}

          <TextPrimitive.Root style={textStyle}>
            {loading && loadingText ? loadingText : children}
          </TextPrimitive.Root>

          {loading && rightIcon ? (
            <ButtonPrimitive.Icon position="right">
              <ActivityIndicator size={spinnerSize} color={spinnerColor} />
            </ButtonPrimitive.Icon>
          ) : rightIcon ? (
            <ButtonPrimitive.Icon
              position="right"
              style={{ width: iconSize, height: iconSize }}
            >
              {rightIcon}
            </ButtonPrimitive.Icon>
          ) : null}
        </ButtonPrimitive.Content>
      </ButtonPrimitive.Root>
    );
  },
);

Button.displayName = "Button";

// Create theme-based styles
function createStyles(theme: any) {
  return StyleSheet.create({
    // Variant styles
    primaryButton: {
      backgroundColor: theme.colors.primary,
      borderWidth: 0,
      ...theme.shadows.sm,
    },
    primaryText: {
      color: theme.colors.primaryForeground,
      fontWeight: "600",
    },

    secondaryButton: {
      backgroundColor: theme.colors.secondary,
      borderWidth: 0,
    },
    secondaryText: {
      color: theme.colors.secondaryForeground,
      fontWeight: "500",
    },

    outlineButton: {
      backgroundColor: "transparent",
      borderWidth: 1,
      borderColor: theme.colors.border,
    },
    outlineText: {
      color: theme.colors.foreground,
      fontWeight: "500",
    },

    ghostButton: {
      backgroundColor: "transparent",
      borderWidth: 0,
    },
    ghostText: {
      color: theme.colors.foreground,
      fontWeight: "500",
    },

    destructiveButton: {
      backgroundColor: theme.colors.destructive,
      borderWidth: 0,
      ...theme.shadows.sm,
    },
    destructiveText: {
      color: theme.colors.destructiveForeground,
      fontWeight: "600",
    },

    successButton: {
      backgroundColor: theme.colors.success,
      borderWidth: 0,
      ...theme.shadows.sm,
    },
    successText: {
      color: theme.colors.successForeground,
      fontWeight: "600",
    },

    pillButton: {
      paddingHorizontal: theme.spacing[2],
      paddingVertical: theme.spacing[1],
      borderRadius: tokens.borderRadius.full,
      minHeight: tokens.touchTargets.minimum / 2,
    },

    pillText: {
      color: theme.colors.primaryForeground,
      fontWeight: "400",
    },

    // Size styles
    smButton: {
      paddingHorizontal: theme.spacing[3],
      paddingVertical: theme.spacing[2],
      borderRadius: tokens.borderRadius.md,
      minHeight: tokens.touchTargets.minimum,
      gap: theme.spacing[1],
    },
    smButtonInner: {
      gap: theme.spacing[1],
    },
    smText: {
      fontSize: 14,
      lineHeight: 16,
    },

    mdButton: {
      paddingHorizontal: theme.spacing[4],
      paddingVertical: theme.spacing[3],
      borderRadius: tokens.borderRadius.md,
      minHeight: tokens.touchTargets.minimum,
      gap: theme.spacing[2],
    },
    mdButtonInner: {
      gap: theme.spacing[2],
    },
    mdText: {
      fontSize: 16,
      lineHeight: 18,
    },

    lgButton: {
      paddingHorizontal: theme.spacing[6],
      paddingVertical: theme.spacing[4],
      borderRadius: tokens.borderRadius.md,
      minHeight: tokens.touchTargets.comfortable,
      gap: theme.spacing[3],
    },
    lgButtonInner: {
      gap: theme.spacing[3],
    },
    lgText: {
      fontSize: 18,
      lineHeight: 20,
    },

    xlButton: {
      paddingHorizontal: theme.spacing[8],
      paddingVertical: theme.spacing[5],
      borderRadius: tokens.borderRadius.lg,
      minHeight: tokens.touchTargets.large,
      gap: theme.spacing[4],
    },
    xlButtonInner: {
      gap: theme.spacing[4],
    },
    xlText: {
      fontSize: 20,
      lineHeight: 24,
    },
  });
}

// Export button variants for external use
export { buttonVariants };
