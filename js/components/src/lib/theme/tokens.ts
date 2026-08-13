/**
 * Design tokens for React Native components
 * Inspired by shadcn/ui but adapted for React Native styling
 */

import type { TextStyle } from "react-native";

export const colors = {
  // Primary colors — the product accent. A muted pink/magenta, aligned with
  // the web app's `--primary` (oklch(0.6803 0.2158 339.7)). Used sparingly:
  // interactive states, focus rings, the Go Live moment.
  primary: {
    50: "#ffd7ff",
    100: "#ffc6ff",
    200: "#ffb1fb",
    300: "#ff9ced",
    400: "#ff87e0",
    500: "#e955c2",
    600: "#c144a0",
    700: "#9b3480",
    800: "#762561",
    900: "#501541",
    950: "#2c0623",
  },

  // Secondary colors — teal, aligned with the web app's `--secondary`
  // (oklch(0.7207 0.1189 198.3)). Used for secondary/accent emphasis.
  secondary: {
    50: "#b6ffff",
    100: "#a2fafc",
    200: "#89ebee",
    300: "#6edcdf",
    400: "#50cdd1",
    500: "#1abbc0",
    600: "#159ca0",
    700: "#0f7e82",
    800: "#0a6264",
    900: "#054446",
    950: "#021f21",
  },

  // Tailwind default palettes:
  slate: {
    50: "#f8fafc",
    100: "#f1f5f9",
    200: "#e2e8f0",
    300: "#cbd5e1",
    400: "#94a3b8",
    500: "#64748b",
    600: "#475569",
    700: "#334155",
    800: "#1e293b",
    900: "#0f172a",
    950: "#020617",
  },
  gray: {
    50: "#f9fafb",
    100: "#f3f4f6",
    200: "#e5e7eb",
    300: "#d1d5db",
    400: "#9ca3af",
    500: "#6b7280",
    600: "#4b5563",
    700: "#374151",
    800: "#1f2937",
    900: "#111827",
    950: "#030712",
  },
  zinc: {
    50: "#fafafa",
    100: "#f4f4f5",
    200: "#e4e4e7",
    300: "#d4d4d8",
    400: "#a1a1aa",
    500: "#71717a",
    600: "#52525b",
    700: "#3f3f46",
    800: "#27272a",
    900: "#18181b",
    950: "#09090b",
  },
  neutral: {
    50: "#fafaf9",
    100: "#f5f5f4",
    200: "#e7e5e4",
    300: "#d6d3d1",
    400: "#a8a29e",
    500: "#78716c",
    600: "#57534e",
    700: "#44403c",
    800: "#292524",
    900: "#1c1917",
    950: "#0c0a09",
  },
  stone: {
    50: "#fafaf9",
    100: "#f5f5f4",
    200: "#e7e5e4",
    300: "#d6d3d1",
    400: "#a8a29e",
    500: "#78716c",
    600: "#57534e",
    700: "#44403c",
    800: "#292524",
    900: "#1c1917",
    950: "#0c0a09",
  },
  red: {
    50: "#fef2f2",
    100: "#fee2e2",
    200: "#fecaca",
    300: "#fca5a5",
    400: "#f87171",
    500: "#ef4444",
    600: "#dc2626",
    700: "#b91c1c",
    800: "#991b1b",
    900: "#7f1d1d",
    950: "#450a0a",
  },
  orange: {
    50: "#fff7ed",
    100: "#ffedd5",
    200: "#fed7aa",
    300: "#fdba74",
    400: "#fb923c",
    500: "#f97316",
    600: "#ea580c",
    700: "#c2410c",
    800: "#9a3412",
    900: "#7c2d12",
    950: "#431407",
  },
  amber: {
    50: "#fffbeb",
    100: "#fef3c7",
    200: "#fde68a",
    300: "#fcd34d",
    400: "#fbbf24",
    500: "#f59e0b",
    600: "#d97706",
    700: "#b45309",
    800: "#92400e",
    900: "#78350f",
    950: "#451a03",
  },
  yellow: {
    50: "#fefce8",
    100: "#fef9c3",
    200: "#fef08a",
    300: "#fde047",
    400: "#facc15",
    500: "#eab308",
    600: "#ca8a04",
    700: "#a16207",
    800: "#854d0e",
    900: "#713f12",
    950: "#422006",
  },
  lime: {
    50: "#f7fee7",
    100: "#ecfccb",
    200: "#d9f99d",
    300: "#bef264",
    400: "#a3e635",
    500: "#84cc16",
    600: "#65a30d",
    700: "#4d7c0f",
    800: "#3f6212",
    900: "#365314",
    950: "#1a2e05",
  },
  green: {
    50: "#f0fdf4",
    100: "#dcfce7",
    200: "#bbf7d0",
    300: "#86efac",
    400: "#4ade80",
    500: "#22c55e",
    600: "#16a34a",
    700: "#15803d",
    800: "#166534",
    900: "#14532d",
    950: "#052e16",
  },
  emerald: {
    50: "#ecfdf5",
    100: "#d1fae5",
    200: "#a7f3d0",
    300: "#6ee7b7",
    400: "#34d399",
    500: "#10b981",
    600: "#059669",
    700: "#047857",
    800: "#065f46",
    900: "#064e3b",
    950: "#022c22",
  },
  teal: {
    50: "#f0fdfa",
    100: "#ccfbf1",
    200: "#99f6e4",
    300: "#5eead4",
    400: "#2dd4bf",
    500: "#14b8a6",
    600: "#0d9488",
    700: "#0f766e",
    800: "#115e59",
    900: "#134e4a",
    950: "#042f2e",
  },
  cyan: {
    50: "#ecfeff",
    100: "#cffafe",
    200: "#a5f3fc",
    300: "#67e8f9",
    400: "#22d3ee",
    500: "#06b6d4",
    600: "#0891b2",
    700: "#0e7490",
    800: "#155e75",
    900: "#164e63",
    950: "#083344",
  },
  sky: {
    50: "#f0f9ff",
    100: "#e0f2fe",
    200: "#bae6fd",
    300: "#7dd3fc",
    400: "#38bdf8",
    500: "#0ea5e9",
    600: "#0284c7",
    700: "#0369a1",
    800: "#075985",
    900: "#0c4a6e",
    950: "#082f49",
  },
  blue: {
    50: "#eff6ff",
    100: "#dbeafe",
    200: "#bfdbfe",
    300: "#93c5fd",
    400: "#60a5fa",
    500: "#3b82f6",
    600: "#2563eb",
    700: "#1d4ed8",
    800: "#1e40af",
    900: "#1e3a8a",
    950: "#172554",
  },
  indigo: {
    50: "#eef2ff",
    100: "#e0e7ff",
    200: "#c7d2fe",
    300: "#a5b4fc",
    400: "#818cf8",
    500: "#6366f1",
    600: "#4f46e5",
    700: "#4338ca",
    800: "#3730a3",
    900: "#312e81",
    950: "#1e1b4b",
  },
  violet: {
    50: "#f5f3ff",
    100: "#ede9fe",
    200: "#ddd6fe",
    300: "#c4b5fd",
    400: "#a78bfa",
    500: "#8b5cf6",
    600: "#7c3aed",
    700: "#6d28d9",
    800: "#5b21b6",
    900: "#4c1d95",
    950: "#2e1065",
  },
  purple: {
    50: "#faf5ff",
    100: "#f3e8ff",
    200: "#e9d5ff",
    300: "#d8b4fe",
    400: "#c084fc",
    500: "#a855f7",
    600: "#9333ea",
    700: "#7e22ce",
    800: "#6b21a8",
    900: "#581c87",
    950: "#3b0764",
  },
  fuchsia: {
    50: "#fdf4ff",
    100: "#fae8ff",
    200: "#f5d0fe",
    300: "#f0abfc",
    400: "#e879f9",
    500: "#d946ef",
    600: "#c026d3",
    700: "#a21caf",
    800: "#86198f",
    900: "#701a75",
    950: "#4a044e",
  },
  pink: {
    50: "#fdf2f8",
    100: "#fce7f3",
    200: "#fbcfe8",
    300: "#f9a8d4",
    400: "#f472b6",
    500: "#ec4899",
    600: "#db2777",
    700: "#be185d",
    800: "#9d174d",
    900: "#831843",
    950: "#500724",
  },
  rose: {
    50: "#fff1f2",
    100: "#ffe4e6",
    200: "#fecdd3",
    300: "#fda4af",
    400: "#fb7185",
    500: "#f43f5e",
    600: "#e11d48",
    700: "#be123c",
    800: "#9f1239",
    900: "#881337",
    950: "#4c0519",
  },

  // Semantic colors
  destructive: {
    50: "#fef2f2",
    100: "#fee2e2",
    200: "#fecaca",
    300: "#fca5a5",
    400: "#f87171",
    500: "#ef4444",
    600: "#dc2626",
    700: "#b91c1c",
    800: "#991b1b",
    900: "#7f1d1d",
    950: "#450a0a",
  },

  success: {
    50: "#f0fdf4",
    100: "#dcfce7",
    200: "#bbf7d0",
    300: "#86efac",
    400: "#4ade80",
    500: "#22c55e",
    600: "#16a34a",
    700: "#15803d",
    800: "#166534",
    900: "#14532d",
    950: "#052e16",
  },

  warning: {
    50: "#fffaf0",
    100: "#ffe6c7",
    200: "#ffd99e",
    300: "#ffcc75",
    400: "#ffb94e",
    500: "#ff9e1f",
    600: "#e67e00",
    700: "#cc6600",
    800: "#998c00",
    900: "#664200",
    950: "#332900",
  },

  // iOS system colors (adaptive)
  ios: {
    systemBlue: "#007AFF",
    systemGreen: "#34C759",
    systemRed: "#FF3B30",
    systemOrange: "#FF9500",
    systemYellow: "#FFCC00",
    systemPurple: "#AF52DE",
    systemPink: "#FF2D92",
    systemTeal: "#5AC8FA",
    systemIndigo: "#5856D6",
    systemGray: "#8E8E93",
    systemGray2: "#AEAEB2",
    systemGray3: "#C7C7CC",
    systemGray4: "#D1D1D6",
    systemGray5: "#E5E5EA",
    systemGray6: "#F2F2F7",
  },

  // Android Material colors
  android: {
    primary: "#6200EE",
    primaryVariant: "#3700B3",
    secondary: "#03DAC6",
    secondaryVariant: "#018786",
    background: "#FFFFFF",
    surface: "#FFFFFF",
    error: "#B00020",
    onPrimary: "#FFFFFF",
    onSecondary: "#000000",
    onBackground: "#000000",
    onSurface: "#000000",
    onError: "#FFFFFF",
  },

  // Transparent colors
  transparent: "transparent",
  black: "#000000",
  white: "#FFFFFF",
} as const;

/**
 * Surface scale — purple-tinted dark, never pure black. Surfaces separate via subtle
 * 1px borders (see `borderAlphas`), not heavy shadows. Light theme derives
 * from the same step names so semantic tokens work in both modes.
 */
export const surfaces = {
  dark: {
    0: "#150e1c", // base: app background (web --background)
    1: "#281b28", // raised: cards, panels, inputs (web --card)
    2: "#201324", // overlay: popovers, menus, sheets (web --popover)
    3: "#292129", // highest: hovered overlay rows, tooltips (web --sidebar)
  },
  light: {
    0: "#fdf6fa",
    1: "#ffffff",
    2: "#ffffff",
    3: "#f5e8f0",
  },
} as const;

/**
 * 4-step text scale: primary / secondary / tertiary / disabled. Steps 1–2
 * mirror the web app's `--foreground` / `--muted-foreground`; 3–4 derive from
 * the foreground at fixed alphas.
 */
export const textAlphas = {
  dark: {
    1: "#f7eaf3",
    2: "#b79aae",
    3: "rgba(247,234,243,0.45)",
    4: "rgba(247,234,243,0.30)",
  },
  light: {
    1: "#3d1c44",
    2: "#9b7e92",
    3: "rgba(61,28,68,0.46)",
    4: "rgba(61,28,68,0.32)",
  },
} as const;

/** 1px hairline borders that separate surfaces. Dark values align with the
 * web app's `--border` (10% foreground) and `--input` (15% foreground). */
export const borderAlphas = {
  dark: {
    subtle: "rgba(247,234,243,0.06)",
    default: "rgba(247,234,243,0.10)",
    strong: "rgba(247,234,243,0.15)",
  },
  light: {
    subtle: "rgba(61,28,68,0.06)",
    default: "#ead5e3",
    strong: "rgba(61,28,68,0.13)",
  },
} as const;

/**
 * Status colors, tuned per scheme. `live` is reserved exclusively for the
 * LIVE state (badges, live rings, on-air indicators) — never for errors.
 */
export const statusColors = {
  live: "#f23041",
  liveDim: "rgba(242,48,65,0.16)",
  dark: {
    success: "#3dd68c",
    warning: "#ffb224",
    danger: "#ff3b5c",
    // Low-alpha danger tint — hover/press wash for red-ink destructive buttons.
    dangerSoft: "rgba(255,59,92,0.14)",
  },
  light: {
    success: "#18794e",
    warning: "#ad5700",
    danger: "#ff3b5c",
    dangerSoft: "rgba(255,59,92,0.10)",
  },
} as const;

/** Modal/photo scrims. */
export const scrims = {
  dark: "rgba(0,0,0,0.72)",
  light: "rgba(0,0,0,0.55)",
} as const;

/**
 * Spacing — 4px base grid. Canonical steps: 4, 8, 12, 16, 24, 32, 48, 64
 * (keys 1, 2, 3, 4, 6, 8, 12, 16). Off-grid keys are deprecated and will be
 * removed once all usages are swept to the canonical set.
 */
export const spacing = {
  0: 0,
  1: 4,
  2: 8,
  3: 12,
  4: 16,
  /** @deprecated off the 4/8/12/16/24/32/48/64 grid — use 4 or 6 */
  5: 20,
  6: 24,
  /** @deprecated off-grid — use 6 or 8 */
  7: 28,
  8: 32,
  /** @deprecated off-grid — use 8 */
  9: 36,
  /** @deprecated off-grid — use 8 or 12 */
  10: 40,
  /** @deprecated off-grid — use 12 */
  11: 44,
  12: 48,
  /** @deprecated off-grid — use 12 or 16 */
  14: 56,
  16: 64,
  20: 80,
  24: 96,
  28: 112,
  32: 128,
  36: 144,
  40: 160,
  44: 176,
  48: 192,
  52: 208,
  56: 224,
  60: 240,
  64: 256,
  72: 288,
  80: 320,
  96: 384,
  auto: "auto",
} as const;

/**
 * Radii: sm (small controls), md (cards, inputs), lg (thumbnails, modals),
 * full (avatars, pills). xl/2xl/3xl are deprecated aliases of lg.
 */
export const borderRadius = {
  none: 0,
  sm: 4,
  md: 8,
  lg: 12,
  /** @deprecated use lg */
  xl: 12,
  /** @deprecated use lg */
  "2xl": 12,
  /** @deprecated use lg */
  "3xl": 12,
  full: 999,
} as const;

/**
 * Typography — one typeface (Atkinson Hyperlegible Next), one modular scale:
 * 12 / 13 / 14 / 16 / 20 / 24 / 32. Weights 400 / 500 / 600 only.
 * Line heights live here, never inline. Sizes ≥20 get tight letter-spacing.
 *
 * `typeScale` is the canonical scale. The `ios` / `android` / `universal` objects
 * are deprecated compatibility remaps onto the same scale and will be removed;
 * new code should use `typeScale` (usually via the Text component's variants).
 */
export const typeScale = {
  xs: {
    fontSize: 12,
    lineHeight: 16,
    fontWeight: "400" as const,
    fontFamily: "AtkinsonHyperlegibleNext-Regular",
  },
  sm: {
    fontSize: 13,
    lineHeight: 18,
    fontWeight: "400" as const,
    fontFamily: "AtkinsonHyperlegibleNext-Regular",
  },
  base: {
    fontSize: 14,
    lineHeight: 20,
    fontWeight: "400" as const,
    fontFamily: "AtkinsonHyperlegibleNext-Regular",
  },
  md: {
    fontSize: 16,
    lineHeight: 24,
    fontWeight: "400" as const,
    fontFamily: "AtkinsonHyperlegibleNext-Regular",
  },
  lg: {
    fontSize: 20,
    lineHeight: 26,
    letterSpacing: -0.2,
    fontWeight: "500" as const,
    fontFamily: "AtkinsonHyperlegibleNext-Medium",
  },
  xl: {
    fontSize: 24,
    lineHeight: 30,
    letterSpacing: -0.3,
    fontWeight: "600" as const,
    fontFamily: "AtkinsonHyperlegibleNext-SemiBold",
  },
  xxl: {
    fontSize: 32,
    lineHeight: 38,
    letterSpacing: -0.5,
    fontWeight: "600" as const,
    fontFamily: "AtkinsonHyperlegibleNext-SemiBold",
  },
} as const;

/** The only three weights in the design system. */
export const fontWeights = {
  regular: "400",
  medium: "500",
  semibold: "600",
} as const;

/**
 * Tabular numerals for anything that counts: viewer counts, timers,
 * durations. Prevents layout jitter as digits change.
 */
export const tabularNums: { fontVariant: TextStyle["fontVariant"] } = {
  fontVariant: ["tabular-nums"],
};

export const typography = {
  /** @deprecated remapped onto the universal scale — use `typeScale` */
  ios: {
    largeTitle: typeScale.xxl,
    title1: typeScale.xxl,
    title2: typeScale.xl,
    title3: typeScale.lg,
    headline: {
      ...typeScale.md,
      fontWeight: "600" as const,
      fontFamily: "AtkinsonHyperlegibleNext-SemiBold",
    },
    body: typeScale.md,
    callout: typeScale.md,
    subhead: typeScale.base,
    footnote: typeScale.sm,
    caption1: typeScale.xs,
    caption2: typeScale.xs,
  },

  /** @deprecated remapped onto the universal scale — use `typeScale` */
  android: {
    headline1: typeScale.xxl,
    headline2: typeScale.xxl,
    headline3: typeScale.xxl,
    headline4: typeScale.xxl,
    headline5: typeScale.xl,
    headline6: typeScale.lg,
    subtitle1: typeScale.md,
    subtitle2: {
      ...typeScale.base,
      fontWeight: "500" as const,
      fontFamily: "AtkinsonHyperlegibleNext-Medium",
    },
    body1: typeScale.md,
    body2: typeScale.base,
    button: {
      ...typeScale.base,
      fontWeight: "500" as const,
      fontFamily: "AtkinsonHyperlegibleNext-Medium",
    },
    caption: typeScale.xs,
    overline: typeScale.xs,
  },

  // Universal typography scale (keys kept for compatibility; values snap to
  // the canonical 12/13/14/16/20/24/32 scale)
  universal: {
    xs: typeScale.xs,
    sm: typeScale.sm,
    base: typeScale.base,
    lg: typeScale.md,
    xl: typeScale.lg,
    "2xl": typeScale.xl,
    "3xl": typeScale.xxl,
    /** @deprecated use 3xl */
    "4xl": typeScale.xxl,
  },

  // Monospace typography for code and technical content (stream keys,
  // ingest URLs, diagnostics)
  mono: {
    xs: {
      fontSize: 12,
      lineHeight: 16,
      fontWeight: "400" as const,
      fontFamily: "IoskeleyMono-Regular",
    },
    sm: {
      fontSize: 13,
      lineHeight: 18,
      fontWeight: "400" as const,
      fontFamily: "IoskeleyMono-Regular",
    },
    base: {
      fontSize: 14,
      lineHeight: 20,
      fontWeight: "400" as const,
      fontFamily: "IoskeleyMono-Regular",
    },
    lg: {
      fontSize: 16,
      lineHeight: 24,
      fontWeight: "400" as const,
      fontFamily: "IoskeleyMono-Regular",
    },
    xl: {
      fontSize: 20,
      lineHeight: 26,
      fontWeight: "500" as const,
      fontFamily: "IoskeleyMono-Medium",
    },
    "2xl": {
      fontSize: 24,
      lineHeight: 30,
      fontWeight: "600" as const,
      fontFamily: "IoskeleyMono-SemiBold",
    },
    "3xl": {
      fontSize: 32,
      lineHeight: 38,
      fontWeight: "600" as const,
      fontFamily: "IoskeleyMono-SemiBold",
    },
  },
} as const;

// Font families available in the app. The design system uses exactly three
// weights; heavier/lighter keys are deprecated aliases kept for compatibility.
export const fontFamilies = {
  // Sans serif fonts
  regular: "AtkinsonHyperlegibleNext-Regular",
  /** @deprecated weights outside 400/500/600 are not part of the design system */
  light: "AtkinsonHyperlegibleNext-Regular",
  /** @deprecated weights outside 400/500/600 are not part of the design system */
  extraLight: "AtkinsonHyperlegibleNext-Regular",
  medium: "AtkinsonHyperlegibleNext-Medium",
  semiBold: "AtkinsonHyperlegibleNext-SemiBold",
  /** @deprecated use semiBold */
  bold: "AtkinsonHyperlegibleNext-SemiBold",
  /** @deprecated use semiBold */
  extraBold: "AtkinsonHyperlegibleNext-SemiBold",

  // Monospace fonts
  monoRegular: "IoskeleyMono-Regular",
  monoMedium: "IoskeleyMono-Medium",
  monoSemiBold: "IoskeleyMono-SemiBold",
  /** @deprecated use monoSemiBold */
  monoBold: "IoskeleyMono-SemiBold",
} as const;

export const shadows = {
  none: {
    shadowColor: "transparent",
    shadowOffset: { width: 0, height: 0 },
    shadowOpacity: 0,
    shadowRadius: 0,
    elevation: 0,
  },
  sm: {
    shadowColor: colors.black,
    shadowOffset: { width: 0, height: 1 },
    shadowOpacity: 0.05,
    shadowRadius: 2,
    elevation: 2,
  },
  md: {
    shadowColor: colors.black,
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
    elevation: 4,
  },
  lg: {
    shadowColor: colors.black,
    shadowOffset: { width: 0, height: 4 },
    shadowOpacity: 0.15,
    shadowRadius: 8,
    elevation: 8,
  },
  xl: {
    shadowColor: colors.black,
    shadowOffset: { width: 0, height: 8 },
    shadowOpacity: 0.2,
    shadowRadius: 16,
    elevation: 16,
  },
} as const;

// Touch targets (iOS Human Interface Guidelines)
export const touchTargets = {
  minimum: 44, // Minimum touch target size
  comfortable: 48, // Comfortable touch target size
  large: 56, // Large touch target size
} as const;

/**
 * Motion. Three durations, one easing. Everything that appears should
 * fade + translate 4–8px — never pop. The only allowed spring is
 * `motion.sheetSpring`, for sheet presentation.
 *
 * fast  (120ms): micro — hover, press feedback
 * base  (200ms): standard — reveals, toggles, fades
 * slow  (300ms): structural — sheets, panels, layout changes
 */
export const motion = {
  fast: 120,
  base: 200,
  slow: 300,
  /** cubic-bezier args for reanimated: Easing.bezier(...motion.bezier) */
  bezier: [0.25, 0.1, 0.25, 1] as const,
  /** the same easing for web `transitionTimingFunction` */
  easingCss: "cubic-bezier(0.25, 0.1, 0.25, 1)",
  /** subtle spring, allowed ONLY for sheet presentation */
  sheetSpring: { damping: 30, stiffness: 300 },
} as const;

/** @deprecated use `motion` (fast/base/slow) */
export const animations = {
  fast: motion.fast,
  normal: motion.base,
  slow: motion.slow,
  /** @deprecated use slow */
  slower: motion.slow,
} as const;

// Breakpoints for responsive design
export const breakpoints = {
  sm: 640,
  md: 768,
  lg: 1024,
  xl: 1280,
  "2xl": 1536,
} as const;

export type Colors = typeof colors;
export type Surfaces = typeof surfaces;
export type TextAlphas = typeof textAlphas;
export type BorderAlphas = typeof borderAlphas;
export type StatusColors = typeof statusColors;
export type TypeScale = typeof typeScale;
export type FontWeights = typeof fontWeights;
export type Motion = typeof motion;
export type Spacing = typeof spacing;
export type BorderRadius = typeof borderRadius;
export type Typography = typeof typography;
export type FontFamilies = typeof fontFamilies;
export type Shadows = typeof shadows;
export type TouchTargets = typeof touchTargets;
export type Animations = typeof animations;
export type Breakpoints = typeof breakpoints;
