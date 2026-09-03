import { useMemo, type ReactNode } from "react";
import {
  useAccentColor,
  useBrandingAsset,
  usePrimaryColor,
  useStreamplaceStore,
} from "../../streamplace-store";
import { ThemeProvider, type Theme } from "./theme";

interface BrandedThemeProviderProps {
  children: ReactNode;
  defaultTheme?: "light" | "dark" | "system";
  forcedTheme?: "light" | "dark";
}

/**
 * ThemeProvider wrapper that automatically applies branding colors from the
 * broadcaster's branding configuration.
 */
export function BrandedThemeProvider({
  children,
  defaultTheme,
  forcedTheme,
}: BrandedThemeProviderProps) {
  const primaryColor = usePrimaryColor();
  const accentColor = useAccentColor();
  const brandingLoading = useStreamplaceStore((state) => state.brandingLoading);

  // Chrome: the node's background/foreground pair per scheme, from which the
  // theme derives surfaces, text and borders. Empty values leave defaults.
  const bgDark = useBrandingAsset("backgroundColor")?.data;
  const fgDark = useBrandingAsset("foregroundColor")?.data;
  const bgLight = useBrandingAsset("backgroundColorLight")?.data;
  const fgLight = useBrandingAsset("foregroundColorLight")?.data;
  const chromeColors = useMemo(
    () => ({
      dark: { background: bgDark, foreground: fgDark },
      light: { background: bgLight, foreground: fgLight },
    }),
    [bgDark, fgDark, bgLight, fgLight],
  );

  // Build color theme overrides from branding
  const colorTheme = useMemo<Partial<Theme["colors"]>>(() => {
    // don't override until branding is loaded
    if (brandingLoading) {
      return {};
    }

    const overrides: Partial<Theme["colors"]> = {};

    if (primaryColor) {
      overrides.primary = primaryColor;
      overrides.ring = primaryColor;
    }

    if (accentColor) {
      overrides.accent = accentColor;
    }

    return overrides;
  }, [primaryColor, accentColor, brandingLoading]);

  return (
    <ThemeProvider
      defaultTheme={defaultTheme}
      forcedTheme={forcedTheme}
      colorTheme={colorTheme}
      chromeColors={chromeColors}
    >
      {children}
    </ThemeProvider>
  );
}
