import { createContext, forwardRef, useContext } from "react";
import type { ColorValue as RNColorValue } from "react-native";
import {
  AnimatableNumericValue,
  ColorValue,
  OpaqueColorValue,
  Platform,
  Text as RNText,
  TextProps as RNTextProps,
  TextStyle,
} from "react-native";
import { useTheme } from "../../../lib/theme/theme";
import { typography, type Typography } from "../../../lib/theme/tokens";

// Text inheritance context
interface TextContextValue {
  fontSize?: number;
  fontWeight?: TextStyle["fontWeight"];
  color?: string | RNColorValue | OpaqueColorValue;
  fontFamily?: string;
  lineHeight?: number;
  textAlign?: TextStyle["textAlign"];
  letterSpacing?: number;
  textTransform?: TextStyle["textTransform"];
  textDecorationLine?: TextStyle["textDecorationLine"];
  fontStyle?: TextStyle["fontStyle"];
  opacity?: number | AnimatableNumericValue;
}

export const TextContext = createContext<Partial<TextContextValue> | null>(
  null,
);

export function objectFromObjects(
  arr: Record<string, any>[],
): Record<string, any> {
  return Object.assign({}, ...arr);
}

// Text primitive props
export interface TextPrimitiveProps extends Omit<RNTextProps, "style"> {
  // Typography variants
  variant?:
    | "h1"
    | "h2"
    | "h3"
    | "h4"
    | "h5"
    | "h6"
    | "body1"
    | "body2"
    | "caption"
    | "overline"
    | "subtitle1"
    | "subtitle2";

  // Size system
  size?: "xs" | "sm" | "base" | "lg" | "xl" | "2xl" | "3xl" | "4xl" | number;

  // Weight system
  weight?:
    | "thin"
    | "light"
    | "normal"
    | "medium"
    | "semibold"
    | "bold"
    | "extrabold"
    | "black";

  // Color variants
  color?:
    | "default"
    | "muted"
    | "primary"
    | "secondary"
    | "destructive"
    | "success"
    | "warning"
    | (string & {});

  // Text alignment
  align?: "left" | "center" | "right" | "justify";

  // Line height
  leading?: "none" | "tight" | "snug" | "normal" | "relaxed" | "loose" | number;

  // Letter spacing
  tracking?:
    | "tighter"
    | "tight"
    | "normal"
    | "wide"
    | "wider"
    | "widest"
    | number;

  // Text transform
  transform?: "none" | "capitalize" | "uppercase" | "lowercase";

  // Text decoration
  decoration?: "none" | "underline" | "line-through";

  // Font style
  italic?: boolean;

  // Opacity
  opacity?: number;

  // Custom style
  style?: TextStyle | TextStyle[];

  // Inheritance - whether this component should inherit from parent context
  inherit?: boolean;

  // Reset inheritance - start fresh context
  reset?: boolean;
}

// Size mapping
const sizeMap = {
  xs: 12,
  sm: 14,
  base: 16,
  lg: 18,
  xl: 20,
  "2xl": 24,
  "3xl": 30,
  "4xl": 36,
} as const;

// Size-specific line height mapping (provides better default line heights for each size)
const sizeLineHeightMap = {
  xs: 16, // 12px * 1.33 = tight but readable
  sm: 20, // 14px * 1.43 = good for small text
  base: 24, // 16px * 1.5 = standard body text
  lg: 28, // 18px * 1.56 = comfortable for larger text
  xl: 30, // 20px * 1.5 = balanced
  "2xl": 32, // 24px * 1.33 = tighter for headings
  "3xl": 36, // 30px * 1.2 = tight for large headings
  "4xl": 40, // 36px * 1.11 = very tight for display text
} as const;

// Weight mapping
const weightMap = {
  thin: "100",
  light: "300",
  normal: "400",
  medium: "500",
  semibold: "600",
  bold: "700",
  extrabold: "800",
  black: "900",
} as const;

// Line height mapping
const leadingMap = {
  none: 1,
  tight: 1.2,
  snug: 1.3,
  normal: 1.5,
  relaxed: 1.7,
  loose: 2,
} as const;

// Letter spacing mapping
const trackingMap = {
  tighter: -0.8,
  tight: -0.4,
  normal: 0,
  wide: 0.4,
  wider: 0.8,
  widest: 1.6,
} as const;

// Variant definitions (platform-aware)
const getVariantStyles = () => {
  // get platform-specific typography
  // iOS, Android, Web (Universal)
  const typographicPlatform = (
    Platform.OS === "ios"
      ? "ios"
      : Platform.OS === "android"
        ? "android"
        : "universal"
  ) as keyof Typography;
  const platformTypography = typography[typographicPlatform] as Record<
    string,
    TextStyle
  >;

  if (!platformTypography) {
    throw new Error("Platform typography not defined");
  }

  // Define mapping based on platform
  if (typographicPlatform === "ios") {
    return {
      h1: platformTypography.largeTitle,
      h2: platformTypography.title1,
      h3: platformTypography.title2,
      h4: platformTypography.title3,
      h5: platformTypography.headline,
      h6: platformTypography.headline,
      subtitle1: platformTypography.subhead,
      subtitle2: platformTypography.footnote,
      body1: platformTypography.body,
      body2: platformTypography.callout,
      caption: platformTypography.caption1,
      overline: platformTypography.caption2,
    };
  } else if (typographicPlatform === "android") {
    return {
      h1: platformTypography.headline4, // 34px instead of 96px
      h2: platformTypography.headline5, // 24px instead of 60px
      h3: platformTypography.headline6, // 20px instead of 48px
      h4: platformTypography.subtitle1, // 16px instead of 34px
      h5: platformTypography.subtitle2, // 14px instead of 24px
      h6: platformTypography.subtitle2, // 14px - consistent with h5
      subtitle1: platformTypography.subtitle1,
      subtitle2: platformTypography.subtitle2,
      body1: platformTypography.body1,
      body2: platformTypography.body2,
      caption: platformTypography.caption,
      overline: platformTypography.overline,
    };
  } else {
    // universal
    // Map variants to universal sizes
    return {
      h1: platformTypography["4xl"],
      h2: platformTypography["3xl"],
      h3: platformTypography["2xl"],
      h4: platformTypography["xl"],
      h5: platformTypography["lg"],
      h6: platformTypography["base"],
      subtitle1: platformTypography.base,
      subtitle2: platformTypography.sm,
      body1: platformTypography.base,
      body2: platformTypography.sm,
      caption: platformTypography.xs,
      overline: platformTypography.xs,
    };
  }
};

// Text root primitive
export const TextRoot = forwardRef<RNText, TextPrimitiveProps>(
  (
    {
      variant,
      size,
      weight,
      color,
      align,
      leading,
      tracking,
      transform,
      decoration,
      italic = false,
      opacity,
      style,
      inherit = true,
      reset = false,
      children,
      ...props
    },
    ref,
  ) => {
    const { theme } = useTheme();
    const parentContext = useContext(TextContext);

    // Get variant styles
    const variantStyles = getVariantStyles() as Record<string, TextStyle>;

    // Calculate inherited values
    const inheritedContext =
      inherit && !reset && parentContext ? parentContext : {};

    // Calculate fontSize first for line height calculation
    let calculatedFontSize = inheritedContext.fontSize;

    // Apply variant font size
    if (variant && variantStyles[variant]?.fontSize) {
      calculatedFontSize = variantStyles[variant].fontSize as number;
    }

    // Apply size-based font size
    if (size) {
      calculatedFontSize = typeof size === "number" ? size : sizeMap[size];
    }

    // Use default if still undefined
    calculatedFontSize = calculatedFontSize || 16;

    // Calculate final styles
    const finalStyles: TextStyle = {
      // Start with inherited values
      fontSize: inheritedContext.fontSize,
      fontWeight: inheritedContext.fontWeight,
      //color: inheritedContext.color,
      fontFamily: inheritedContext.fontFamily,
      lineHeight: inheritedContext.lineHeight,
      textAlign: inheritedContext.textAlign,
      letterSpacing: inheritedContext.letterSpacing,
      textTransform: inheritedContext.textTransform,
      textDecorationLine:
        inheritedContext.textDecorationLine as TextStyle["textDecorationLine"],
      fontStyle: inheritedContext.fontStyle,
      opacity: inheritedContext.opacity,

      // Apply variant styles (these may override inherited)
      ...(variant && variantStyles[variant]),

      // Apply explicit prop styles (these should override inherited and variant)

      // Apply size (with corresponding line height if not explicitly set)
      ...(size && {
        fontSize: typeof size === "number" ? size : sizeMap[size],
        // Apply size-specific line height only if leading is not explicitly set
        ...(leading === undefined && {
          lineHeight:
            typeof size === "number"
              ? size // Auto line height for numeric sizes
              : sizeLineHeightMap[size],
        }),
      }),

      // Apply weight
      ...(weight && {
        fontWeight: weightMap[weight] as TextStyle["fontWeight"],
      }),

      // Apply color
      ...(color
        ? {
            color:
              color === "default"
                ? theme.colors.text
                : color === "muted"
                  ? theme.colors.textMuted
                  : color === "primary"
                    ? theme.colors.primary
                    : color === "secondary"
                      ? theme.colors.secondary
                      : color === "destructive"
                        ? theme.colors.destructive
                        : color === "success"
                          ? theme.colors.success
                          : color === "warning"
                            ? theme.colors.warning
                            : color || inheritedContext.color, // Custom color string
          }
        : { color: inheritedContext.color || theme.colors.text }),

      // Apply alignment
      ...(align && {
        textAlign: align,
      }),

      // Apply line height
      ...(leading && {
        lineHeight:
          typeof leading === "number"
            ? leading
            : leadingMap[leading] * calculatedFontSize,
      }),

      // Apply letter spacing
      ...(tracking && {
        letterSpacing:
          typeof tracking === "number" ? tracking : trackingMap[tracking],
      }),

      // Apply text transform
      ...(transform &&
        transform !== "none" && {
          textTransform: transform,
        }),

      // Apply text decoration
      ...(decoration &&
        decoration !== "none" && {
          textDecorationLine: decoration,
        }),

      // Apply italic
      ...(italic && {
        fontStyle: "italic",
      }),

      // Apply opacity
      ...(opacity !== undefined && {
        opacity,
      }),
    };

    finalStyles.color = finalStyles.color as ColorValue;

    // Create context value for children
    // Process custom styles to auto-add line height for fontSize
    const processedStyle = Array.isArray(style)
      ? style
      : [style].filter(Boolean);
    const enhancedStyles = processedStyle.map((styleObj) => {
      if (styleObj && typeof styleObj === "object" && "fontSize" in styleObj) {
        const fontSize = styleObj.fontSize;
        if (typeof fontSize === "number" && !styleObj.lineHeight && !leading) {
          return {
            ...styleObj,
            lineHeight: fontSize,
          };
        }
      }
      return styleObj;
    });

    const contextValue: TextContextValue = {
      fontSize:
        typeof finalStyles.fontSize === "number"
          ? finalStyles.fontSize
          : undefined,
      fontWeight: finalStyles.fontWeight,
      color: finalStyles.color || undefined,
      fontFamily:
        typeof finalStyles.fontFamily === "string"
          ? finalStyles.fontFamily
          : undefined,
      lineHeight:
        typeof finalStyles.lineHeight === "number"
          ? finalStyles.lineHeight
          : undefined,
      textAlign: finalStyles.textAlign,
      letterSpacing: finalStyles.letterSpacing as number | undefined,
      textTransform: finalStyles.textTransform,
      textDecorationLine:
        finalStyles.textDecorationLine as TextStyle["textDecorationLine"],
      fontStyle: finalStyles.fontStyle,
      opacity: finalStyles.opacity as number | undefined,
    };

    return (
      <TextContext.Provider value={contextValue}>
        <RNText ref={ref} style={[finalStyles, ...enhancedStyles]} {...props}>
          {children}
        </RNText>
      </TextContext.Provider>
    );
  },
);

TextRoot.displayName = "TextRoot";

// Text span primitive (inherits from parent but doesn't create new context)
export const TextSpan = forwardRef<RNText, Omit<TextPrimitiveProps, "reset">>(
  ({ children, ...props }, ref) => {
    return (
      <TextRoot ref={ref as any} inherit={true} {...props}>
        {children}
      </TextRoot>
    );
  },
);

TextSpan.displayName = "TextSpan";

// Text block primitive (always creates new context)
export const TextBlock = forwardRef<RNText, TextPrimitiveProps>(
  ({ children, reset = true, ...props }, ref) => {
    return (
      <TextRoot ref={ref as any} reset={reset} {...props}>
        {children}
      </TextRoot>
    );
  },
);

TextBlock.displayName = "TextBlock";

// Hook to access current text context
export function useTextContext(): TextContextValue | null {
  return useContext(TextContext);
}

// Utility function to create text styles
export function createTextStyle(
  props: Omit<TextPrimitiveProps, "children" | "style" | "ref">,
): TextStyle {
  // This is a utility function that can be used to generate styles
  // without rendering a component
  const style: TextStyle = {};

  if (props.size) {
    style.fontSize =
      typeof props.size === "number" ? props.size : sizeMap[props.size];
    // Apply size-specific line height only if leading is not explicitly set
    if (props.leading === undefined) {
      style.lineHeight =
        typeof props.size === "number"
          ? props.size
          : sizeLineHeightMap[props.size];
    }
  }

  if (props.weight) {
    style.fontWeight = weightMap[props.weight] as TextStyle["fontWeight"];
  }

  if (props.align) {
    style.textAlign = props.align;
  }

  if (props.leading) {
    const fontSize = style.fontSize || 16; // default font size
    style.lineHeight =
      typeof props.leading === "number"
        ? props.leading
        : leadingMap[props.leading] * fontSize;
  }

  if (props.tracking) {
    style.letterSpacing =
      typeof props.tracking === "number"
        ? props.tracking
        : trackingMap[props.tracking];
  }

  if (props.transform && props.transform !== "none") {
    style.textTransform = props.transform;
  }

  if (props.decoration && props.decoration !== "none") {
    style.textDecorationLine = props.decoration;
  }

  if (props.italic) {
    style.fontStyle = "italic";
  }

  if (props.opacity !== undefined) {
    style.opacity = props.opacity;
  }

  return style;
}

// Export primitive collection
export const TextPrimitive: {
  Root: typeof TextRoot;
  Span: typeof TextSpan;
  Block: typeof TextBlock;
  Context: typeof TextContext;
  useContext: typeof useTextContext;
  createStyle: typeof createTextStyle;
} = {
  Root: TextRoot,
  Span: TextSpan,
  Block: TextBlock,
  Context: TextContext,
  useContext: useTextContext,
  createStyle: createTextStyle,
};
