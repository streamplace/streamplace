import { useMemo, type ReactNode } from "react";
import {
  useAccentColor,
  useBrandingAsset,
  usePrimaryColor,
  useStreamplaceStore,
} from "../../streamplace-store";
import { ThemeProvider, type BrandColors, type Theme } from "./theme";

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

  // Accent doubles as the secondary color (the only accent-ish token the
  // app actually renders); status and live colors are their own keys.
  const dangerColor = useBrandingAsset("dangerColor")?.data;
  const successColor = useBrandingAsset("successColor")?.data;
  const warningColor = useBrandingAsset("warningColor")?.data;
  const infoColor = useBrandingAsset("infoColor")?.data;
  const liveColor = useBrandingAsset("liveColor")?.data;
  const accentLight = useBrandingAsset("accentColorLight")?.data;
  const dangerLight = useBrandingAsset("dangerColorLight")?.data;
  const successLight = useBrandingAsset("successColorLight")?.data;
  const warningLight = useBrandingAsset("warningColorLight")?.data;
  const infoLight = useBrandingAsset("infoColorLight")?.data;
  const brandColors = useMemo<BrandColors | undefined>(() => {
    if (brandingLoading) return undefined;
    return {
      secondary: accentColor,
      danger: dangerColor,
      success: successColor,
      warning: warningColor,
      info: infoColor,
      live: liveColor,
      secondaryLight: accentLight,
      dangerLight,
      successLight,
      warningLight,
      infoLight,
    };
  }, [
    brandingLoading,
    accentColor,
    dangerColor,
    successColor,
    warningColor,
    infoColor,
    liveColor,
    accentLight,
    dangerLight,
    successLight,
    warningLight,
    infoLight,
  ]);

  return (
    <ThemeProvider
      defaultTheme={defaultTheme}
      forcedTheme={forcedTheme}
      colorTheme={colorTheme}
      chromeColors={chromeColors}
      brandColors={brandColors}
    >
      {children}
    </ThemeProvider>
  );
}
