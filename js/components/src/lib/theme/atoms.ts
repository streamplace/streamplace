/**
 * Theme atoms - Enhanced exports with pairify function for array-style syntax
 * These provide direct access to static design tokens and support composition
 */

import { Platform } from "react-native";
import {
  animations,
  borderRadius,
  colors as rawColors,
  shadows,
  spacing,
  touchTargets,
  typography,
} from "./tokens";

// Type for style objects that can be spread
type StyleValue = Record<string, any>;

/**
 * Pairify function - converts nested objects into key-value pairs that return style objects
 * This allows for array-style syntax like style={[a.borders.green[300], a.shadows.xl]}
 */
function pairify<T extends Record<string, any>>(
  obj: T,
  styleKeyPrefix: string,
): Record<keyof T, StyleValue> {
  const result: Record<string, StyleValue> = {};

  for (const [key, value] of Object.entries(obj)) {
    if (typeof value === "object" && value !== null && !Array.isArray(value)) {
      // For nested objects (like color scales), create another level
      result[key] = {};
      for (const [nestedKey, nestedValue] of Object.entries(value)) {
        result[key][nestedKey] = { [styleKeyPrefix]: nestedValue };
      }
    } else {
      // For simple values, create the style object directly
      result[key] = { [styleKeyPrefix]: value };
    }
  }

  return result as Record<keyof T, StyleValue>;
}

/**
 * Create pairified style atoms for easy composition
 */

// Re-export static design tokens that don't change with theme
export { animations, borderRadius, shadows, spacing, touchTargets };

// Export raw color tokens for advanced use cases
export const colors = rawColors;

// Platform-aware typography helper
export const getPlatformTypography = () => {
  if (Platform.OS === "ios") {
    return typography.ios;
  } else if (Platform.OS === "android") {
    return typography.android;
  }
  return typography.universal;
};

// Export all typography scales
export const typographyAtoms = {
  platform: getPlatformTypography(),
  universal: typography.universal,
  ios: typography.ios,
  android: typography.android,
};

// Static icon sizes (colors are handled by theme)
export const iconSizes = {
  sm: 16,
  md: 20,
  lg: 24,
  xl: 32,
};

// Common layout utilities
export const layout = {
  flex: {
    center: {
      justifyContent: "center" as const,
      alignItems: "center" as const,
    },
    alignCenter: {
      alignItems: "center" as const,
    },
    justifyCenter: {
      justifyContent: "center" as const,
    },
    row: {
      flexDirection: "row" as const,
    },
    column: {
      flexDirection: "column" as const,
    },
    spaceBetween: {
      justifyContent: "space-between" as const,
    },
    spaceAround: {
      justifyContent: "space-around" as const,
    },
    spaceEvenly: {
      justifyContent: "space-evenly" as const,
    },
    direction: pairify(
      {
        row: "row",
        column: "column",
        "row-reverse": "row-reverse",
        "column-reverse": "column-reverse",
      },
      "flexDirection",
    ),
    justify: pairify(
      {
        start: "flex-start",
        end: "flex-end",
        center: "center",
        between: "space-between",
        around: "space-around",
        evenly: "space-evenly",
      },
      "justifyContent",
    ),
    align: pairify(
      {
        start: "flex-start",
        end: "flex-end",
        center: "center",
        stretch: "stretch",
        baseline: "baseline",
      },
      "alignItems",
    ),
    wrap: pairify(
      {
        wrap: "wrap",
        nowrap: "nowrap",
        reverse: "wrap-reverse",
      },
      "flexWrap",
    ),
  },
  position: pairify(
    {
      absolute: "absolute",
      relative: "relative",
      static: "static",
    },
    "position",
  ),
};

// Enhanced border utilities with pairified colors and widths
export const borders = {
  width: pairify(
    {
      thin: 1,
      medium: 2,
      thick: 4,
    },
    "borderWidth",
  ),

  style: pairify(
    {
      solid: "solid",
      dashed: "dashed",
      dotted: "dotted",
    },
    "borderStyle",
  ),

  // Pairified color borders
  color: pairify(rawColors, "borderColor"),

  // Top border utilities
  top: {
    width: pairify(
      {
        thin: 1,
        medium: 2,
        thick: 4,
      },
      "borderTopWidth",
    ),
    color: pairify(rawColors, "borderTopColor"),
  },

  // Bottom border utilities
  bottom: {
    width: pairify(
      {
        thin: 1,
        medium: 2,
        thick: 4,
      },
      "borderBottomWidth",
    ),
    color: pairify(rawColors, "borderBottomColor"),
  },

  // Left border utilities
  left: {
    width: pairify(
      {
        thin: 1,
        medium: 2,
        thick: 4,
      },
      "borderLeftWidth",
    ),
    color: pairify(rawColors, "borderLeftColor"),
  },

  // Right border utilities
  right: {
    width: pairify(
      {
        thin: 1,
        medium: 2,
        thick: 4,
      },
      "borderRightWidth",
    ),
    color: pairify(rawColors, "borderRightColor"),
  },
};

// Pairified spacing utilities
export const spacingAtoms = {
  margin: pairify(spacing, "margin"),
  marginTop: pairify(spacing, "marginTop"),
  marginRight: pairify(spacing, "marginRight"),
  marginBottom: pairify(spacing, "marginBottom"),
  marginLeft: pairify(spacing, "marginLeft"),
  marginHorizontal: pairify(spacing, "marginHorizontal"),
  marginVertical: pairify(spacing, "marginVertical"),

  padding: pairify(spacing, "padding"),
  paddingTop: pairify(spacing, "paddingTop"),
  paddingRight: pairify(spacing, "paddingRight"),
  paddingBottom: pairify(spacing, "paddingBottom"),
  paddingLeft: pairify(spacing, "paddingLeft"),
  paddingHorizontal: pairify(spacing, "paddingHorizontal"),
  paddingVertical: pairify(spacing, "paddingVertical"),
};

// Pairified border radius utilities
export const radiusAtoms = {
  all: pairify(borderRadius, "borderRadius"),
  top: pairify(borderRadius, "borderTopLeftRadius"),
  topRight: pairify(borderRadius, "borderTopRightRadius"),
  bottom: pairify(borderRadius, "borderBottomLeftRadius"),
  bottomRight: pairify(borderRadius, "borderBottomRightRadius"),
  left: pairify(borderRadius, "borderTopLeftRadius"),
  right: pairify(borderRadius, "borderTopRightRadius"),
};

// Background color utilities
export const backgrounds = pairify(rawColors, "backgroundColor");

// Text color utilities
export const textColors = pairify(rawColors, "color");

// Percentage-based sizes
const percentageSizes = {
  "10": "10%",
  "20": "20%",
  "25": "25%",
  "30": "30%",
  "33": "33.333333%",
  "40": "40%",
  "50": "50%",
  "60": "60%",
  "66": "66.666667%",
  "70": "70%",
  "75": "75%",
  "80": "80%",
  "90": "90%",
  "100": "100%",
} as const;

// Size utilities (width and height)
export const sizes = {
  width: {
    ...pairify(spacing, "width"),
    percent: pairify(percentageSizes, "width"),
  },
  height: {
    ...pairify(spacing, "height"),
    percent: pairify(percentageSizes, "height"),
  },
  minWidth: {
    ...pairify(spacing, "minWidth"),
    percent: pairify(percentageSizes, "minWidth"),
  },
  minHeight: {
    ...pairify(spacing, "minHeight"),
    percent: pairify(percentageSizes, "minHeight"),
  },
  maxWidth: {
    ...pairify(spacing, "maxWidth"),
    percent: pairify(percentageSizes, "maxWidth"),
  },
  maxHeight: {
    ...pairify(spacing, "maxHeight"),
    percent: pairify(percentageSizes, "maxHeight"),
  },
};

// Flex utilities
export const flex = {
  values: pairify(
    {
      0: 0,
      1: 1,
      2: 2,
      3: 3,
      4: 4,
      5: 5,
    },
    "flex",
  ),

  grow: pairify(
    {
      0: 0,
      1: 1,
    },
    "flexGrow",
  ),

  shrink: pairify(
    {
      0: 0,
      1: 1,
    },
    "flexShrink",
  ),

  basis: {
    ...pairify(spacing, "flexBasis"),
    ...pairify(percentageSizes, "flexBasis"),
    auto: { flexBasis: "auto" },
  },
};

// Opacity utilities
export const opacity = pairify(
  {
    0: 0,
    5: 0.05,
    10: 0.1,
    20: 0.2,
    25: 0.25,
    30: 0.3,
    40: 0.4,
    50: 0.5,
    60: 0.6,
    70: 0.7,
    75: 0.75,
    80: 0.8,
    90: 0.9,
    95: 0.95,
    100: 1,
  },
  "opacity",
);

// Z-index utilities
export const zIndex = pairify(
  {
    0: 0,
    10: 10,
    20: 20,
    30: 30,
    40: 40,
    50: 50,
    auto: "auto",
  },
  "zIndex",
);

// Overflow utilities
export const overflow = pairify(
  {
    visible: "visible",
    hidden: "hidden",
    scroll: "scroll",
  },
  "overflow",
);

// Text alignment utilities
export const textAlign = pairify(
  {
    left: "left",
    center: "center",
    right: "right",
    justify: "justify",
    auto: "auto",
  },
  "textAlign",
);

// Font weight utilities
export const fontWeight = pairify(
  {
    thin: "100",
    extralight: "200",
    light: "300",
    normal: "400",
    medium: "500",
    semibold: "600",
    bold: "700",
    extrabold: "800",
    black: "900",
  },
  "fontWeight",
);

// Font size utilities (separate from typography for quick access)
export const fontSize = pairify(
  {
    xs: 12,
    sm: 14,
    base: 16,
    lg: 18,
    xl: 20,
    "2xl": 24,
    "3xl": 30,
    "4xl": 36,
    "5xl": 48,
    "6xl": 60,
    "7xl": 72,
    "8xl": 96,
    "9xl": 128,
  },
  "fontSize",
);

// Line height utilities
export const lineHeight = pairify(
  {
    none: 1,
    tight: 1.25,
    snug: 1.375,
    normal: 1.5,
    relaxed: 1.625,
    loose: 2,
    3: 12,
    4: 16,
    5: 20,
    6: 24,
    7: 28,
    8: 32,
    9: 36,
    10: 40,
  },
  "lineHeight",
);

// Letter spacing utilities
export const letterSpacing = pairify(
  {
    tighter: -0.5,
    tight: -0.25,
    normal: 0,
    wide: 0.25,
    wider: 0.5,
    widest: 1,
  },
  "letterSpacing",
);

// Text transform utilities
export const textTransform = pairify(
  {
    uppercase: "uppercase",
    lowercase: "lowercase",
    capitalize: "capitalize",
    none: "none",
  },
  "textTransform",
);

// Text decoration utilities
export const textDecoration = pairify(
  {
    none: "none",
    underline: "underline",
    lineThrough: "line-through",
    underlineLineThrough: "underline line-through",
  },
  "textDecorationLine",
);

// Text align vertical utilities (React Native specific)
export const textAlignVertical = pairify(
  {
    auto: "auto",
    top: "top",
    bottom: "bottom",
    center: "center",
  },
  "textAlignVertical",
);

// Transform utilities
export const transforms = {
  rotate: pairify(
    {
      0: 0,
      1: 1,
      2: 2,
      3: 3,
      6: 6,
      12: 12,
      45: 45,
      90: 90,
      180: 180,
      270: 270,
    },
    "rotate",
  ),

  scale: pairify(
    {
      0: 0,
      50: 0.5,
      75: 0.75,
      90: 0.9,
      95: 0.95,
      100: 1,
      105: 1.05,
      110: 1.1,
      125: 1.25,
      150: 1.5,
      200: 2,
    },
    "scale",
  ),

  scaleX: pairify(
    {
      0: 0,
      50: 0.5,
      75: 0.75,
      90: 0.9,
      95: 0.95,
      100: 1,
      105: 1.05,
      110: 1.1,
      125: 1.25,
      150: 1.5,
      200: 2,
    },
    "scaleX",
  ),

  scaleY: pairify(
    {
      0: 0,
      50: 0.5,
      75: 0.75,
      90: 0.9,
      95: 0.95,
      100: 1,
      105: 1.05,
      110: 1.1,
      125: 1.25,
      150: 1.5,
      200: 2,
    },
    "scaleY",
  ),

  translateX: pairify(spacing, "translateX"),
  translateY: pairify(spacing, "translateY"),
};

// Absolute positioning utilities
export const position = {
  top: pairify(spacing, "top"),
  right: pairify(spacing, "right"),
  bottom: pairify(spacing, "bottom"),
  left: pairify(spacing, "left"),

  // Common position combinations
  topLeft: (top: number, left: number) => ({
    position: "absolute" as const,
    top,
    left,
  }),
  topRight: (top: number, right: number) => ({
    position: "absolute" as const,
    top,
    right,
  }),
  bottomLeft: (bottom: number, left: number) => ({
    position: "absolute" as const,
    bottom,
    left,
  }),
  bottomRight: (bottom: number, right: number) => ({
    position: "absolute" as const,
    bottom,
    right,
  }),

  // Percentage-based positioning
  percent: {
    top: pairify(
      {
        0: "0%",
        25: "25%",
        50: "50%",
        75: "75%",
        100: "100%",
      },
      "top",
    ),
    right: pairify(
      {
        0: "0%",
        25: "25%",
        50: "50%",
        75: "75%",
        100: "100%",
      },
      "right",
    ),
    bottom: pairify(
      {
        0: "0%",
        25: "25%",
        50: "50%",
        75: "75%",
        100: "100%",
      },
      "bottom",
    ),
    left: pairify(
      {
        0: "0%",
        25: "25%",
        50: "50%",
        75: "75%",
        100: "100%",
      },
      "left",
    ),
  },
};

// Aspect ratio utilities (React Native 0.71+)
export const aspectRatio = pairify(
  {
    square: 1,
    video: 16 / 9,
    photo: 4 / 3,
    portrait: 3 / 4,
    wide: 21 / 9,
    ultrawide: 32 / 9,
    "1/1": 1,
    "3/2": 3 / 2,
    "4/3": 4 / 3,
    "16/9": 16 / 9,
    "21/9": 21 / 9,
  },
  "aspectRatio",
);

// Gap utilities (React Native 0.71+)
export const gap = {
  row: pairify(spacing, "rowGap"),
  column: pairify(spacing, "columnGap"),
  all: pairify(spacing, "gap"),
};

// Common layout patterns
export const layouts = {
  // Full screen
  fullScreen: {
    position: "absolute" as const,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
  },

  // Centered content
  centered: {
    flex: 1,
    justifyContent: "center" as const,
    alignItems: "center" as const,
  },

  // Centered modal/overlay
  overlay: {
    position: "absolute" as const,
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: "center" as const,
    alignItems: "center" as const,
    backgroundColor: "rgba(0, 0, 0, 0.5)",
  },

  // Safe area friendly
  safeContainer: {
    flex: 1,
    paddingTop: Platform.OS === "ios" ? 44 : 0, // Status bar height
  },

  // Row with space between
  spaceBetweenRow: {
    flexDirection: "row" as const,
    justifyContent: "space-between" as const,
    alignItems: "center" as const,
  },

  // Sticky header
  stickyHeader: {
    position: "absolute" as const,
    top: 0,
    left: 0,
    right: 0,
    zIndex: 10,
  },

  // Bottom sheet style
  bottomSheet: {
    position: "absolute" as const,
    bottom: 0,
    left: 0,
    right: 0,
    borderTopLeftRadius: 16,
    borderTopRightRadius: 16,
  },
};

// Export everything as a combined atoms object for convenience
export const atoms = {
  colors: rawColors,
  spacing,
  borderRadius,
  radius: radiusAtoms,
  typography: typographyAtoms,
  shadows,
  touchTargets,
  animations,
  iconSizes,
  layout,
  borders,
  backgrounds,
  textColors,
  spacingAtoms,
  sizes,
  flex,
  opacity,
  zIndex,
  overflow,
  textAlign,
  fontWeight,
  fontSize,
  lineHeight,
  letterSpacing,
  textTransform,
  textDecoration,
  textAlignVertical,
  transforms,
  position,
  aspectRatio,
  gap,
  layouts,
};

// Convenient shorthand aliases
export const a = atoms;
export const bg = backgrounds;
export const text = textColors;
export const m = spacingAtoms.margin;
export const mt = spacingAtoms.marginTop;
export const mr = spacingAtoms.marginRight;
export const mb = spacingAtoms.marginBottom;
export const ml = spacingAtoms.marginLeft;
export const mx = spacingAtoms.marginHorizontal;
export const my = spacingAtoms.marginVertical;
export const p = spacingAtoms.padding;
export const pt = spacingAtoms.paddingTop;
export const pr = spacingAtoms.paddingRight;
export const pb = spacingAtoms.paddingBottom;
export const pl = spacingAtoms.paddingLeft;
export const px = spacingAtoms.paddingHorizontal;
export const py = spacingAtoms.paddingVertical;
export const w = sizes.width;
export const h = sizes.height;
export const r = radiusAtoms.all;
export const top = position.top;
export const right = position.right;
export const bottom = position.bottom;
export const left = position.left;
export const rotate = transforms.rotate;
export const scale = transforms.scale;
export const translateX = transforms.translateX;
export const translateY = transforms.translateY;
