import { PortalHost } from "@rn-primitives/portal";
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { Platform, useColorScheme } from "react-native";
import {
  contrastForeground,
  defaultChrome,
  deriveChrome,
  parseHexColor,
  withAlpha,
  type ChromeColors,
  type DerivedChrome,
} from "./chrome";
import {
  animations,
  borderRadius,
  colors,
  motion,
  scrims,
  shadows,
  spacing,
  statusColors,
  touchTargets,
  typography,
} from "./tokens";

/**
 * Branded accent and status colors. Each is a hex color; invalid or missing
 * values keep the token defaults. The scheme-specific variants win for that
 * scheme when set. Foreground-on-color tokens are derived by luminance.
 */
export interface BrandColors {
  secondary?: string;
  danger?: string;
  success?: string;
  warning?: string;
  info?: string;
  live?: string;
  secondaryLight?: string;
  dangerLight?: string;
  successLight?: string;
  warningLight?: string;
  infoLight?: string;
}

const validHex = (v: string | undefined) =>
  v && parseHexColor(v) ? v.trim() : undefined;

// pick the branded value for this scheme, if any
function brandFor(
  brand: BrandColors | undefined,
  key: "secondary" | "danger" | "success" | "warning" | "info",
  isDark: boolean,
): string | undefined {
  if (!brand) return undefined;
  const light = validHex(brand[`${key}Light`]);
  if (!isDark && light) return light;
  return validHex(brand[key]);
}

import { GestureHandlerRootView } from "react-native-gesture-handler";
import { ToastProvider } from "../../components/ui/toast";

// Import pairify function for generating theme tokens
function pairify<T extends Record<string, any>>(
  obj: T,
  styleKeyPrefix: string,
): Record<keyof T, any> {
  const result: Record<string, any> = {};
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
  return result as Record<keyof T, any>;
}

// Theme interfaces
export interface Theme {
  colors: {
    // Core semantic colors
    background: string;
    foreground: string;

    // Card/surface colors
    card: string;
    cardForeground: string;

    // Popover colors
    popover: string;
    popoverForeground: string;

    // Primary colors
    primary: string;
    primaryForeground: string;

    // Secondary colors
    secondary: string;
    secondaryForeground: string;

    // Muted colors
    muted: string;
    mutedForeground: string;

    // Accent colors
    accent: string;
    accentForeground: string;

    // Destructive colors
    destructive: string;
    destructiveForeground: string;

    // Success colors
    success: string;
    successForeground: string;

    // Warning colors
    warning: string;
    warningForeground: string;

    // Info colors
    info: string;
    infoForeground: string;

    // Border and input colors
    border: string;
    input: string;
    ring: string;

    // Text colors
    text: string;
    textMuted: string;
    textDisabled: string;

    // Surface scale — base app background → raised → overlay → highest.
    // Surfaces separate with hairline borders, not shadows.
    surface0: string;
    surface1: string;
    surface2: string;
    surface3: string;
    surfaceHover: string;

    // 4-step text scale: primary / secondary / tertiary / disabled
    text1: string;
    text2: string;
    text3: string;
    text4: string;

    // Hairline borders (border = default strength)
    borderSubtle: string;
    borderStrong: string;

    // LIVE — reserved for the live state, never errors
    live: string;
    liveDim: string;
    liveForeground: string;

    // Modal scrim + focus ring
    overlay: string;
    focus: string;

    // Danger (alias family of destructive, tuned per scheme)
    danger: string;
    // Low-alpha danger tint for red-ink destructive button hover/press
    dangerSoft: string;

    // Ink & Paper — the monochrome high-contrast pole, theme-adaptive.
    // `inverse` is the primary-button fill (Paper on dark, Ink on light);
    // `inverseForeground` is the text/icon on it. Contrast, not hue, carries
    // primary emphasis — pink is reserved for state (see `accent`/`primary`).
    inverse: string;
    inverseForeground: string;
  };
  spacing: typeof spacing;
  borderRadius: typeof borderRadius;
  typography: typeof typography;
  shadows: typeof shadows;
  touchTargets: typeof touchTargets;
  animations: typeof animations;
  motion: typeof motion;
}

// Theme-aware zero interface (like atoms but with theme colors)
export interface ThemeZero {
  // Colors using pairify
  bg: Record<string, any>;
  text: Record<string, any>;
  border: Record<string, any>;

  // Static design tokens (same as atoms)
  shadow: {
    sm: typeof shadows.sm;
    md: typeof shadows.md;
    lg: typeof shadows.lg;
    xl: typeof shadows.xl;
  };

  // Common button styles
  button: {
    primary: object;
    secondary: object;
    outline: object;
    ghost: object;
  };

  // Input styles
  input: {
    base: object;
    focused: object;
    error: object;
  };

  // Card styles
  card: {
    base: object;
  };
}

// Icon utilities interface
export interface ThemeIcons {
  color: {
    default: string;
    muted: string;
    primary: string;
    secondary: string;
    destructive: string;
    success: string;
    warning: string;
  };
  size: {
    sm: number;
    md: number;
    lg: number;
    xl: number;
  };
}

// Create theme colors based on dark mode
const createThemeColors = (
  isDark: boolean,
  lightTheme?: ColorPalette | Theme["colors"],
  darkTheme?: ColorPalette | Theme["colors"],
  colorTheme?: Partial<Theme["colors"]>,
  chrome?: { dark?: DerivedChrome | null; light?: DerivedChrome | null },
  brand?: BrandColors,
): Theme["colors"] => {
  let baseColors: Theme["colors"];

  if (isDark && darkTheme) {
    // Use dark theme
    baseColors = isColorPalette(darkTheme)
      ? generateThemeColorsFromPalette(darkTheme, true, chrome, brand)
      : darkTheme;
  } else if (!isDark && lightTheme) {
    // Use light theme
    baseColors = isColorPalette(lightTheme)
      ? generateThemeColorsFromPalette(lightTheme, false, chrome, brand)
      : lightTheme;
  } else {
    // Fall back to default gray theme
    const defaultPalette = colors.neutral;
    baseColors = generateThemeColorsFromPalette(
      defaultPalette,
      isDark,
      chrome,
      brand,
    );
  }

  // Merge with custom color overrides if provided. Focus rings follow the
  // ring color so broadcaster branding recolors them too.
  const merged = {
    ...baseColors,
    ...colorTheme,
  };
  if (colorTheme?.ring && !colorTheme.focus) {
    merged.focus = colorTheme.ring;
  }
  // A branded primary picks its own text color, so a light brand color
  // doesn't get white-on-pastel buttons.
  if (colorTheme?.primary && !colorTheme.primaryForeground) {
    merged.primaryForeground = contrastForeground(colorTheme.primary);
  }
  return merged;
};

// Create theme-aware zero tokens using pairify
const createThemeZero = (themeColors: Theme["colors"]): ThemeZero => ({
  // Theme-aware colors using pairify
  bg: pairify(themeColors, "backgroundColor"),
  text: pairify(themeColors, "color"),
  border: {
    ...pairify(themeColors, "borderColor"),
    default: { borderColor: themeColors.border },
  },

  // Static design tokens
  shadow: {
    sm: shadows.sm,
    md: shadows.md,
    lg: shadows.lg,
    xl: shadows.xl,
  },

  // Common button styles
  button: {
    primary: {
      backgroundColor: themeColors.primary,
      borderWidth: 0,
      ...shadows.sm,
    },
    secondary: {
      backgroundColor: themeColors.secondary,
      borderWidth: 0,
    },
    outline: {
      backgroundColor: "transparent",
      borderWidth: 1,
      borderColor: themeColors.border,
    },
    ghost: {
      backgroundColor: "transparent",
      borderWidth: 0,
    },
  },

  // Input styles
  input: {
    base: {
      backgroundColor: themeColors.background,
      borderWidth: 1,
      borderColor: themeColors.border,
      borderRadius: borderRadius.md,
      paddingHorizontal: spacing[3],
      paddingVertical: spacing[3],
      minHeight: touchTargets.minimum,
    },
    focused: {
      borderColor: themeColors.ring,
      borderWidth: 2,
    },
    error: {
      borderColor: themeColors.destructive,
      borderWidth: 2,
    },
  },

  // Card styles
  card: {
    base: {
      backgroundColor: themeColors.card,
      borderRadius: borderRadius.lg,
      ...shadows.sm,
    },
  },
});

// Create theme icons based on colors
const createThemeIcons = (themeColors: Theme["colors"]): ThemeIcons => ({
  color: {
    default: themeColors.text,
    muted: themeColors.textMuted,
    primary: themeColors.primary,
    secondary: themeColors.secondary,
    destructive: themeColors.destructive,
    success: themeColors.success,
    warning: themeColors.warning,
  },
  size: {
    sm: 16,
    md: 20,
    lg: 24,
    xl: 32,
  },
});

// Theme context interface
interface ThemeContextType {
  theme: Theme;
  zero: ThemeZero;
  icons: ThemeIcons;
  isDark: boolean;
  currentTheme: "light" | "dark" | "system";
  systemTheme: "light" | "dark";
  setTheme: (theme: "light" | "dark" | "system") => void;
  toggleTheme: () => void;
}

// Create the theme context
const ThemeContext = createContext<ThemeContextType | null>(null);

// Color palette type
type ColorPalette = {
  50: string;
  100: string;
  200: string;
  300: string;
  400: string;
  500: string;
  600: string;
  700: string;
  800: string;
  900: string;
  950: string;
};

// Helper function to check if input is a ColorPalette or Theme["colors"]
function isColorPalette(
  input: ColorPalette | Theme["colors"],
): input is ColorPalette {
  return "50" in input && "100" in input && "950" in input;
}

// Helper function to generate Theme["colors"] from ColorPalette.
// The neutral chrome (surfaces, text, borders) comes from the fixed
// dark-first design tokens; the palette parameter is kept for API
// compatibility with consumers passing custom palettes and only tints
// palette-derived slots (secondary/muted/input).
function generateThemeColorsFromPalette(
  palette: ColorPalette,
  isDark: boolean,
  chrome?: { dark?: DerivedChrome | null; light?: DerivedChrome | null },
  brand?: BrandColors,
): Theme["colors"] {
  // The neutral chrome comes from the node's branded background/foreground
  // pair when it has one (see chrome.ts), else the design tokens.
  const own = (isDark ? chrome?.dark : chrome?.light) ?? defaultChrome(isDark);
  const other =
    (isDark ? chrome?.light : chrome?.dark) ?? defaultChrome(!isDark);
  const surface = own.surface;
  const text = own.text;
  const border = own.border;
  const tokenStatus = isDark ? statusColors.dark : statusColors.light;
  const isDefaultPalette = palette === colors.neutral;

  // Branded status colors, with their soft tints re-derived so a custom
  // danger red gets a matching hover wash.
  const danger = brandFor(brand, "danger", isDark);
  const success = brandFor(brand, "success", isDark);
  const warning = brandFor(brand, "warning", isDark);
  const info = brandFor(brand, "info", isDark);
  const live = validHex(brand?.live);
  const status = {
    danger: danger ?? tokenStatus.danger,
    dangerSoft: danger
      ? withAlpha(danger, isDark ? 0.14 : 0.1)
      : tokenStatus.dangerSoft,
    success: success ?? tokenStatus.success,
    warning: warning ?? tokenStatus.warning,
  };
  const brandedSecondary = brandFor(brand, "secondary", isDark);

  return {
    background: surface[0],
    foreground: text[1],

    card: surface[1],
    cardForeground: text[1],

    popover: surface[2],
    popoverForeground: text[1],

    // One accent, used sparingly. (No per-platform accent split — the
    // product looks the same everywhere.) Pink/magenta, aligned with the
    // web app's `--primary`.
    primary: colors.primary[500],
    primaryForeground: colors.white,

    // Teal, aligned with the web app's `--secondary`; a node's accentColor
    // branding replaces it.
    secondary:
      brandedSecondary ??
      (isDefaultPalette
        ? colors.secondary[500]
        : isDark
          ? palette[800]
          : palette[100]),
    secondaryForeground: brandedSecondary
      ? contrastForeground(brandedSecondary)
      : isDark
        ? surface[0]
        : colors.white,

    muted: isDefaultPalette ? surface[2] : isDark ? palette[800] : palette[100],
    mutedForeground: text[2],

    accent: isDefaultPalette
      ? isDark
        ? colors.secondary[500]
        : surface[2]
      : isDark
        ? palette[800]
        : palette[100],
    accentForeground: isDark ? surface[0] : text[1],

    destructive: status.danger,
    destructiveForeground: colors.white,

    success: status.success,
    successForeground: success
      ? contrastForeground(success)
      : isDark
        ? surface[0]
        : colors.white,

    warning: status.warning,
    warningForeground: warning
      ? contrastForeground(warning)
      : isDark
        ? surface[0]
        : colors.white,

    // Info is a blue, distinct from the pink primary. Aligned with the web's
    // `--color-info` (chart-3); the light value reads on dark, the dark on light.
    info: info ?? (isDark ? "#88c0f9" : "#335b83"),
    infoForeground: text[1],

    border: border.default,
    input: surface[1],
    ring: colors.primary[500],

    text: text[1],
    textMuted: text[2],
    textDisabled: text[4],

    // New-scale tokens
    surface0: surface[0],
    surface1: surface[1],
    surface2: surface[2],
    surface3: surface[3],
    surfaceHover: surface[3],

    text1: text[1],
    text2: text[2],
    text3: text[3],
    text4: text[4],

    borderSubtle: border.subtle,
    borderStrong: border.strong,

    live: live ?? statusColors.live,
    liveDim: live ? withAlpha(live, 0.16) : statusColors.liveDim,
    liveForeground: live ? contrastForeground(live) : colors.white,

    overlay: isDark ? scrims.dark : scrims.light,
    focus: colors.primary[500],

    danger: status.danger,
    dangerSoft: status.dangerSoft,

    // Paper on dark, Ink on light — the opposite scheme's raised surface, with
    // the current scheme's base surface as its text.
    inverse: other.surface[1],
    inverseForeground: surface[0],
  };
}

// Theme provider props
interface ThemeProviderProps {
  children: ReactNode;
  defaultTheme?: "light" | "dark" | "system";
  forcedTheme?: "light" | "dark";
  colorTheme?: Partial<Theme["colors"]>;
  lightTheme?: ColorPalette | Theme["colors"];
  darkTheme?: ColorPalette | Theme["colors"];
  /**
   * Branded chrome: a background + foreground pair per scheme from which the
   * surface, text and border ramps are derived. Missing or invalid pairs
   * keep the design-token defaults for that scheme.
   */
  chromeColors?: {
    dark?: Partial<ChromeColors>;
    light?: Partial<ChromeColors>;
  };
  /** Branded accent and status colors; see BrandColors. */
  brandColors?: BrandColors;
}

// Theme provider component
// Should be surrounded by SafeAreaProvider at the root
export function ThemeProvider({
  children,
  defaultTheme = "system",
  forcedTheme,
  colorTheme,
  lightTheme,
  darkTheme,
  chromeColors,
  brandColors,
}: ThemeProviderProps) {
  const systemColorScheme = useColorScheme();
  const chrome = useMemo(
    () => ({
      dark: deriveChrome(chromeColors?.dark, true),
      light: deriveChrome(chromeColors?.light, false),
    }),
    [
      chromeColors?.dark?.background,
      chromeColors?.dark?.foreground,
      chromeColors?.light?.background,
      chromeColors?.light?.foreground,
    ],
  );
  const [currentTheme, setCurrentTheme] = useState<"light" | "dark" | "system">(
    defaultTheme,
  );

  // Determine if dark mode should be active
  const isDark = useMemo(() => {
    if (forcedTheme === "light") return false;
    if (forcedTheme === "dark") return true;
    if (currentTheme === "light") return false;
    if (currentTheme === "dark") return true;
    if (currentTheme === "system") return systemColorScheme === "dark";
    return systemColorScheme === "dark";
  }, [forcedTheme, currentTheme, systemColorScheme]);

  // Create theme based on dark mode
  const theme = useMemo<Theme>(() => {
    const themeColors = createThemeColors(
      isDark,
      lightTheme,
      darkTheme,
      colorTheme,
      chrome,
      brandColors,
    );
    return {
      colors: themeColors,
      spacing,
      borderRadius,
      typography,
      shadows,
      touchTargets,
      animations,
      motion,
    };
  }, [isDark, lightTheme, darkTheme, colorTheme, chrome, brandColors]);

  // Create theme-aware zero tokens
  const zero = useMemo<ThemeZero>(() => {
    return createThemeZero(theme.colors);
  }, [theme.colors]);

  // Create icon utilities
  const icons = useMemo<ThemeIcons>(() => {
    return createThemeIcons(theme.colors);
  }, [theme.colors]);

  // Theme controls
  const setTheme = (newTheme: "light" | "dark" | "system") => {
    if (!forcedTheme) {
      setCurrentTheme(newTheme);
    }
  };

  const toggleTheme = () => {
    if (!forcedTheme) {
      setCurrentTheme((prev) => {
        if (prev === "light") return "dark";
        if (prev === "dark") return "system";
        return "light";
      });
    }
  };

  const value = useMemo<ThemeContextType>(
    () => ({
      theme,
      zero,
      icons,
      isDark,
      currentTheme: forcedTheme || currentTheme,
      systemTheme: (systemColorScheme as "light" | "dark") || "light",
      setTheme,
      toggleTheme,
    }),
    [
      theme,
      zero,
      icons,
      isDark,
      forcedTheme,
      currentTheme,
      systemColorScheme,
      setTheme,
      toggleTheme,
    ],
  );

  const parentTheme = useContext(ThemeContext);
  const isRoot = !parentTheme;

  // Web keyboard navigation: one global :focus-visible rule (2px ring,
  // 2px offset) instead of per-component focus tracking. Mouse/touch
  // interactions don't show the ring; keyboard focus always does.
  useEffect(() => {
    if (!isRoot || Platform.OS !== "web" || typeof document === "undefined") {
      return;
    }
    let el = document.getElementById("sp-focus-ring");
    if (!el) {
      el = document.createElement("style");
      el.id = "sp-focus-ring";
      document.head.appendChild(el);
    }
    el.textContent = [
      `:focus { outline: none; }`,
      `:focus-visible { outline: 2px solid ${theme.colors.focus}; outline-offset: 2px; }`,
    ].join("\n");
  }, [isRoot, theme.colors.focus]);

  return (
    <ThemeContext.Provider value={value}>
      {isRoot ? (
        <GestureHandlerRootView>
          {children}
          <PortalHost />
          <ToastProvider />
        </GestureHandlerRootView>
      ) : (
        children
      )}
    </ThemeContext.Provider>
  );
}

// Hook to use theme
export function useTheme(): ThemeContextType {
  const context = useContext(ThemeContext);
  if (!context) {
    throw new Error("useTheme must be used within a ThemeProvider");
  }
  return context;
}

// Hook to get current platform's typography
export function usePlatformTypography() {
  const { theme } = useTheme();

  return useMemo(() => {
    if (Platform.OS === "ios") {
      return theme.typography.ios;
    } else if (Platform.OS === "android") {
      return theme.typography.android;
    }
    return theme.typography.universal;
  }, [theme.typography]);
}

// Utility function to create theme-aware styles
export function createThemedStyles<T extends Record<string, any>>(
  styleCreator: (theme: Theme, zero: ThemeZero, icons: ThemeIcons) => T,
) {
  return function useThemedStyles() {
    const { theme, zero, icons } = useTheme();
    return useMemo(
      () => styleCreator(theme, zero, icons),
      [theme, zero, icons],
    );
  };
}

// Create light and dark theme instances for external use
export const lightTheme: Theme = {
  colors: createThemeColors(false),
  spacing,
  borderRadius,
  typography,
  shadows,
  touchTargets,
  animations,
  motion,
};

export const darkTheme: Theme = {
  colors: createThemeColors(true),
  spacing,
  borderRadius,
  typography,
  shadows,
  touchTargets,
  animations,
  motion,
};

// Export individual theme utilities for convenience
export { createThemeColors, createThemeIcons, createThemeZero };
