import { PortalHost } from "@rn-primitives/portal";
import {
  createContext,
  useContext,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { Platform, useColorScheme } from "react-native";
import {
  animations,
  borderRadius,
  colors,
  shadows,
  spacing,
  touchTargets,
  typography,
} from "./tokens";

import { GestureHandlerRootView } from "react-native-gesture-handler";
import { ToastProvider } from "../../components/ui";

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
  };
  spacing: typeof spacing;
  borderRadius: typeof borderRadius;
  typography: typeof typography;
  shadows: typeof shadows;
  touchTargets: typeof touchTargets;
  animations: typeof animations;
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
): Theme["colors"] => {
  let baseColors: Theme["colors"];

  if (isDark && darkTheme) {
    // Use dark theme
    baseColors = isColorPalette(darkTheme)
      ? generateThemeColorsFromPalette(darkTheme, true)
      : darkTheme;
  } else if (!isDark && lightTheme) {
    // Use light theme
    baseColors = isColorPalette(lightTheme)
      ? generateThemeColorsFromPalette(lightTheme, false)
      : lightTheme;
  } else {
    // Fall back to default gray theme
    const defaultPalette = colors.neutral;
    baseColors = generateThemeColorsFromPalette(defaultPalette, isDark);
  }

  // Merge with custom color overrides if provided
  return {
    ...baseColors,
    ...colorTheme,
  };
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

// Helper function to generate Theme["colors"] from ColorPalette
function generateThemeColorsFromPalette(
  palette: ColorPalette,
  isDark: boolean,
): Theme["colors"] {
  return {
    background: isDark ? palette[950] : colors.white,
    foreground: isDark ? palette[50] : palette[950],

    card: isDark ? palette[900] : colors.white,
    cardForeground: isDark ? palette[50] : palette[950],

    popover: isDark ? palette[900] : colors.white,
    popoverForeground: isDark ? palette[50] : palette[950],

    primary:
      Platform.OS === "ios" ? colors.ios.systemBlue : colors.primary[500],
    primaryForeground: colors.white,

    secondary: isDark ? palette[800] : palette[100],
    secondaryForeground: isDark ? palette[50] : palette[900],

    muted: isDark ? palette[800] : palette[100],
    mutedForeground: isDark ? palette[400] : palette[500],

    accent: isDark ? palette[800] : palette[100],
    accentForeground: isDark ? palette[50] : palette[900],

    destructive: colors.destructive[700],
    destructiveForeground: colors.white,

    success: colors.success[700],
    successForeground: colors.white,

    warning: colors.warning[700],
    warningForeground: colors.white,

    info: colors.blue[700],
    infoForeground: isDark ? palette[50] : palette[900],

    border: isDark ? palette[500] + "30" : palette[200] + "30",
    input: isDark ? palette[800] : palette[200],
    ring: Platform.OS === "ios" ? colors.ios.systemBlue : colors.primary[500],

    text: isDark ? palette[50] : palette[950],
    textMuted: isDark ? palette[400] : palette[500],
    textDisabled: isDark ? palette[600] : palette[400],
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
}: ThemeProviderProps) {
  const systemColorScheme = useColorScheme();
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
    );
    return {
      colors: themeColors,
      spacing,
      borderRadius,
      typography,
      shadows,
      touchTargets,
      animations,
    };
  }, [isDark, lightTheme, darkTheme, colorTheme]);

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

  return (
    <ThemeContext.Provider value={value}>
      <GestureHandlerRootView>
        {children}
        <PortalHost />
        <ToastProvider />
      </GestureHandlerRootView>
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
};

export const darkTheme: Theme = {
  colors: createThemeColors(true),
  spacing,
  borderRadius,
  typography,
  shadows,
  touchTargets,
  animations,
};

// Export individual theme utilities for convenience
export { createThemeColors, createThemeIcons, createThemeZero };
