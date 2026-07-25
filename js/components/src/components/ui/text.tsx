import { cva, type VariantProps } from "class-variance-authority";
import React, { forwardRef } from "react";
import { useTheme } from "../../lib/theme/theme";
import { fontFamilies } from "../../lib/theme/tokens";
import * as zero from "../../ui";
import { TextPrimitive, TextPrimitiveProps } from "./primitives/text";

// Text variants using class-variance-authority pattern
const textVariants = cva("", {
  variants: {
    variant: {
      h1: "h1",
      h2: "h2",
      h3: "h3",
      h4: "h4",
      h5: "h5",
      h6: "h6",
      subtitle1: "subtitle1",
      subtitle2: "subtitle2",
      body1: "body1",
      body2: "body2",
      caption: "caption",
      overline: "overline",
    },
    size: {
      xs: "xs",
      sm: "sm",
      base: "base",
      lg: "lg",
      xl: "xl",
      "2xl": "2xl",
      "3xl": "3xl",
      "4xl": "4xl",
    },
    weight: {
      thin: "thin",
      light: "light",
      normal: "normal",
      medium: "medium",
      semibold: "semibold",
      bold: "bold",
      extrabold: "extrabold",
      black: "black",
    },
    color: {
      default: "default",
      muted: "muted",
      primary: "primary",
      secondary: "secondary",
      destructive: "destructive",
      success: "success",
      warning: "warning",
    },
  },
  defaultVariants: {
    variant: "body1",
    size: "base",
    weight: "normal",
    color: "default",
  },
});

export interface TextProps
  extends
    Omit<TextPrimitiveProps, "variant" | "size" | "weight" | "color">,
    Omit<VariantProps<typeof textVariants>, "color"> {
  // Override color to accept hex values and custom strings
  color?:
    | "default"
    | "muted"
    | "primary"
    | "secondary"
    | "destructive"
    | "success"
    | "warning"
    | (string & {});
  // Additional convenience props
  muted?: boolean;
  bold?: boolean;
  italic?: boolean;
  underline?: boolean;
  strikethrough?: boolean;
  uppercase?: boolean;
  lowercase?: boolean;
  capitalize?: boolean;
  center?: boolean;
  right?: boolean;
  justify?: boolean;
  // Custom color override
  customColor?: string;
}

export const Text = forwardRef<any, TextProps>(
  (
    {
      variant = undefined,
      size = undefined,
      weight = undefined,
      color = undefined,
      muted = false,
      bold = false,
      italic = false,
      underline = false,
      strikethrough = false,
      uppercase = false,
      lowercase = false,
      capitalize = false,
      center = false,
      right = false,
      justify = false,
      customColor,
      style,
      children,
      ...props
    },
    ref,
  ) => {
    const { zero: zt } = useTheme();

    // Override props based on convenience props
    const finalColor = customColor ? customColor : muted ? "muted" : color;

    const finalTransform = uppercase
      ? "uppercase"
      : lowercase
        ? "lowercase"
        : capitalize
          ? "capitalize"
          : "none";

    const finalDecoration =
      underline && strikethrough
        ? "underline line-through"
        : underline
          ? "underline"
          : strikethrough
            ? "line-through"
            : "none";

    const finalAlign = center
      ? "center"
      : right
        ? "right"
        : justify
          ? "justify"
          : "left";

    // Get variant-specific styles using theme.zero
    const variantStyle = React.useMemo(() => {
      switch (variant) {
        case "h1":
          return zero.my[6];
        case "h2":
        case "h3":
          return zero.my[4];
        case "h4":
        case "h5":
        case "h6":
        case "subtitle1":
          return zero.my[2];
        case "subtitle2":
        case "caption":
        case "overline":
          return zero.my[1];
        case "body1":
          return zero.my[4];
        case "body2":
          return zero.my[2];
        default:
          return {};
      }
    }, [variant, zt]);

    const styleArr = (
      Array.isArray(style) ? style : [style || undefined]
    ).filter((s) => s !== undefined);

    const finalWeight = bold ? "bold" : (weight ?? undefined);

    return (
      <TextPrimitive.Root
        ref={ref}
        variant={variant || "body1"}
        size={size || "base"}
        weight={finalWeight}
        color={finalColor || "default"}
        align={finalAlign}
        transform={finalTransform}
        decoration={finalDecoration as any}
        italic={italic}
        style={[variantStyle, ...styleArr]}
        {...props}
      >
        {children}
      </TextPrimitive.Root>
    );
  },
);

Text.displayName = "Text";

// Convenience components for common text elements
export const Heading = forwardRef<
  any,
  Omit<TextProps, "variant"> & { level?: 1 | 2 | 3 | 4 | 5 | 6 }
>(({ level = 1, ...props }, ref) => (
  <Text ref={ref} variant={`h${level}` as any} {...props} />
));

Heading.displayName = "Heading";

export const Subtitle = forwardRef<
  any,
  Omit<TextProps, "variant"> & { level?: 1 | 2 }
>(({ level = 1, ...props }, ref) => (
  <Text
    ref={ref}
    variant={level === 1 ? "subtitle1" : "subtitle2"}
    {...props}
  />
));

Subtitle.displayName = "Subtitle";

export const Body = forwardRef<
  any,
  Omit<TextProps, "variant"> & { level?: 1 | 2 }
>(({ level = 1, ...props }, ref) => (
  <Text ref={ref} variant={level === 1 ? "body1" : "body2"} {...props} />
));

Body.displayName = "Body";

export const Caption = forwardRef<any, Omit<TextProps, "variant">>(
  (props, ref) => <Text ref={ref} variant="caption" {...props} />,
);

Caption.displayName = "Caption";

export const Label = forwardRef<any, Omit<TextProps, "variant">>(
  (props, ref) => (
    <Text ref={ref} variant="subtitle1" weight="medium" {...props} />
  ),
);

Label.displayName = "Label";

export const Code = forwardRef<any, TextProps>(({ style, ...props }, ref) => {
  const { zero: zt } = useTheme();

  const codeStyle = React.useMemo(
    () => [
      {
        borderRadius: zero.borderRadius.sm,
        fontFamily: fontFamilies.monoRegular,
      },
    ],
    [zt],
  );

  // if style is not an array, convert it to an array
  const styleArr = (Array.isArray(style) ? style : [style || undefined]).filter(
    (s) => s !== undefined,
  );

  return <Text ref={ref} style={[...codeStyle, ...styleArr]} {...props} />;
});

Code.displayName = "Code";

// Span component for inline text styling (inherits from parent)
export const Span = forwardRef<
  any,
  Omit<TextProps, "variant" | "size" | "weight" | "color"> & {
    variant?:
      | "h1"
      | "h2"
      | "h3"
      | "h4"
      | "h5"
      | "h6"
      | "subtitle1"
      | "subtitle2"
      | "body1"
      | "body2"
      | "caption"
      | "overline";
    size?: "xs" | "sm" | "base" | "lg" | "xl" | "2xl" | "3xl" | "4xl";
    weight?:
      | "thin"
      | "light"
      | "normal"
      | "medium"
      | "semibold"
      | "bold"
      | "extrabold"
      | "black";
    color?:
      | "default"
      | "muted"
      | "primary"
      | "secondary"
      | "destructive"
      | "success"
      | "warning";
  }
>(({ children, ...props }, ref) => (
  <TextPrimitive.Span ref={ref} {...props}>
    {children}
  </TextPrimitive.Span>
));

Span.displayName = "Span";

// Export text variants for external use
export { textVariants };

// Re-export primitive components for advanced usage
export { TextPrimitive };
