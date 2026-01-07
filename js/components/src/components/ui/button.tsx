import { cva, type VariantProps } from "class-variance-authority";
import React, { forwardRef, useMemo } from "react";
import { ActivityIndicator } from "react-native";
import { useTheme } from "../../lib/theme/theme";
import * as zero from "../../ui";
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
  width?: "full" | "min" | number;
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
      width = "full",
      ...props
    },
    ref,
  ) => {
    const { zero: zt, icons } = useTheme();

    // Get variant styles using theme.zero
    const buttonStyle = useMemo(() => {
      switch (variant) {
        case "primary":
          return zt.button.primary;
        case "secondary":
          return zt.button.secondary;
        case "outline":
          return zt.button.outline;
        case "ghost":
          return zt.button.ghost;
        case "destructive":
          return [zt.bg.destructive, zero.shadows.sm];
        case "success":
          return [zt.bg.success, zero.shadows.sm];
        default:
          return zt.button.primary;
      }
    }, [variant, zt]);

    // Get text styles using theme.zero
    const textStyle = useMemo(() => {
      switch (variant) {
        case "primary":
          return [zt.text.primaryForeground, { fontWeight: "600" }];
        case "secondary":
          return [zt.text.secondaryForeground, { fontWeight: "500" }];
        case "outline":
        case "ghost":
          return [zt.text.foreground, { fontWeight: "500" }];
        case "destructive":
          return [zt.text.destructiveForeground, { fontWeight: "600" }];
        case "success":
          return [zt.text.successForeground, { fontWeight: "600" }];
        default:
          return [zt.text.primaryForeground, { fontWeight: "600" }];
      }
    }, [variant, zt]);

    // Size styles using theme.zero
    const sizeStyles = useMemo(() => {
      switch (size) {
        case "sm":
          return {
            button: [
              zero.px[3],
              zero.py[2],
              { borderRadius: zero.borderRadius.md },
            ],
            inner: { gap: 4 },
            text: zero.typography.universal.sm,
          };
        case "lg":
          return {
            button: [
              zero.px[6],
              zero.py[3],
              { borderRadius: zero.borderRadius.md },
            ],
            inner: { gap: 12 },
            text: zero.typography.universal.lg,
          };
        case "xl":
          return {
            button: [
              zero.px[8],
              zero.py[4],
              { borderRadius: zero.borderRadius.lg },
            ],
            inner: { gap: 12 },
            text: zero.typography.universal.xl,
          };
        case "pill":
          return {
            button: [
              zero.px[2],
              zero.py[1],
              { borderRadius: zero.borderRadius.full },
            ],
            inner: { gap: 4 },
            text: zero.typography.universal.xs,
          };
        case "md":
        default:
          return {
            button: [
              zero.px[4],
              zero.py[2],
              { borderRadius: zero.borderRadius.md },
            ],
            inner: { gap: 6 },
            text: zero.typography.universal.sm,
          };
      }
    }, [size, zt]);

    const iconSize = React.useMemo(() => {
      switch (size) {
        case "sm":
          return icons.size.sm;
        case "lg":
          return icons.size.lg;
        case "xl":
          return icons.size.xl;
        case "md":
        default:
          return icons.size.md;
      }
    }, [size, icons]);

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
          return icons.color.primary;
        case "secondary":
          return icons.color.secondary;
        case "destructive":
          return icons.color.destructive;
        case "success":
          return icons.color.success;
        default:
          return icons.color.default;
      }
    }, [variant, icons]);

    const widthStyle = useMemo(() => {
      if (width === "full") {
        return { width: "100%" };
      } else if (width === "min") {
        return { alignSelf: "flex-start" as const };
      } else {
        return { width };
      }
    }, [width]);

    return (
      <ButtonPrimitive.Root
        ref={ref}
        disabled={disabled || loading}
        style={[buttonStyle, sizeStyles.button, widthStyle, style]}
        {...props}
      >
        <ButtonPrimitive.Content style={sizeStyles.inner}>
          {loading && !leftIcon ? (
            <ButtonPrimitive.Icon position="left">
              <ActivityIndicator size={spinnerSize} color={spinnerColor} />
            </ButtonPrimitive.Icon>
          ) : leftIcon ? (
            <ButtonPrimitive.Icon position="left">
              {leftIcon}
            </ButtonPrimitive.Icon>
          ) : null}

          {typeof children === "string" ? (
            <TextPrimitive.Root style={[textStyle as any, sizeStyles.text]}>
              {loading && loadingText ? loadingText : children}
            </TextPrimitive.Root>
          ) : loading && loadingText ? (
            loadingText
          ) : (
            children
          )}

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

// Export button variants for external use
export { buttonVariants };
