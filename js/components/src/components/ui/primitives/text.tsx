import { createContext, forwardRef, useContext } from "react";
import type { ColorValue as RNColorValue } from "react-native";
import {
  AnimatableNumericValue,
  ColorValue,
  OpaqueColorValue,
  Text as RNText,
  TextProps as RNTextProps,
  TextStyle,
} from "react-native";
import { useTheme } from "../../../lib/theme/theme";
import {
  fontFamilies,
  tabularNums,
  typography,
} from "../../../lib/theme/tokens";

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

  // Tabular numerals — required for anything that counts (viewers, timers)
  tabular?: boolean;

  // Opacity
  opacity?: number;

  // Custom style
  style?: TextStyle | TextStyle[];

  // Inheritance - whether this component should inherit from parent context
  inherit?: boolean;

  // Reset inheritance - start fresh context
  reset?: boolean;
}

// Size mapping — the modular scale (12/13/14/16/20/24/32). Keys are kept
// stable; "4xl" is a deprecated alias of "3xl".
const sizeMap = {
  xs: 12,
  sm: 13,
  base: 14,
  lg: 16,
  xl: 20,
  "2xl": 24,
  "3xl": 32,
  "4xl": 32,
} as const;

// Per-size line heights — defined here, never inline
const sizeLineHeightMap = {
  xs: 16,
  sm: 18,
  base: 20,
  lg: 24,
  xl: 26,
  "2xl": 30,
  "3xl": 38,
  "4xl": 38,
} as const;

// Comfortable leading for a raw numeric fontSize. Mirrors the ratios baked into
// sizeLineHeightMap — ~1.4 for body copy, tightening toward ~1.2 for display
// sizes — so inline `fontSize` never renders at a cramped 1.0 leading (the old
// behavior, which made stacked title/subtitle pairs look jammed together).
const autoLineHeight = (fontSize: number) => {
  const ratio = fontSize <= 16 ? 1.4 : fontSize <= 24 ? 1.3 : 1.2;
  return Math.round(fontSize * ratio);
};

// Weight mapping — the design system has exactly three weights.
// Out-of-range keys are deprecated aliases clamped to the nearest weight.
const weightMap = {
  thin: "400",
  light: "400",
  normal: "400",
  medium: "500",
  semibold: "600",
  bold: "600",
  extrabold: "600",
  black: "600",
} as const;

// Static font files need the family to match the weight
const weightFamilyMap: Record<keyof typeof weightMap, string> = {
  thin: fontFamilies.regular,
  light: fontFamilies.regular,
  normal: fontFamilies.regular,
  medium: fontFamilies.medium,
  semibold: fontFamilies.semiBold,
  bold: fontFamilies.semiBold,
  extrabold: fontFamilies.semiBold,
  black: fontFamilies.semiBold,
};

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

// Variant definitions — one scale on every platform. The product should
// look identical on iOS, Android, and web.
const universal = typography.universal as Record<string, TextStyle>;
const semibold = {
  fontWeight: "600",
  fontFamily: fontFamilies.semiBold,
} as const;
const medium = {
  fontWeight: "500",
  fontFamily: fontFamilies.medium,
} as const;

const variantStylesStatic: Record<string, TextStyle> = {
  h1: universal["3xl"],
  h2: universal["2xl"],
  h3: universal.xl,
  h4: { ...universal.lg, ...semibold },
  h5: { ...universal.base, ...semibold },
  h6: { ...universal.sm, ...semibold },
  subtitle1: { ...universal.lg, ...medium },
  subtitle2: { ...universal.base, ...medium },
  body1: universal.base,
  body2: universal.sm,
  caption: universal.xs,
  overline: { ...universal.xs, ...medium, textTransform: "uppercase" },
};

const getVariantStyles = () => variantStylesStatic;

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
      tabular = false,
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
              ? autoLineHeight(size) // Comfortable leading for numeric sizes
              : sizeLineHeightMap[size],
        }),
      }),

      // Apply weight (family must track weight for static font files)
      ...(weight && {
        fontWeight: weightMap[weight] as TextStyle["fontWeight"],
        fontFamily: weightFamilyMap[weight],
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

      // Apply tabular numerals
      ...(tabular && tabularNums),

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
            lineHeight: autoLineHeight(fontSize),
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
          ? autoLineHeight(props.size)
          : sizeLineHeightMap[props.size];
    }
  }

  if (props.weight) {
    style.fontWeight = weightMap[props.weight] as TextStyle["fontWeight"];
    style.fontFamily = weightFamilyMap[props.weight];
  }

  if (props.tabular) {
    Object.assign(style, tabularNums);
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
