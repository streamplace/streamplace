import { useMemo, type ReactNode } from "react";
import { useBrandingAsset } from "../../streamplace-store";
import {
  ThemeProvider,
  type BrandColors,
  type Theme,
  type Typeface,
} from "./theme";

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
  // Raw values, undefined until the node's branding is known (injected
  // meta, the cache, or the fetch). Overrides apply as soon as a value is
  // known rather than waiting for the fetch to finish: gating on the
  // fetch flashed the default palette on every load even when the colors
  // were already at hand.
  const primaryColor = useBrandingAsset("primaryColor")?.data || undefined;
  const accentColor = useBrandingAsset("accentColor")?.data || undefined;

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
    const overrides: Partial<Theme["colors"]> = {};

    if (primaryColor) {
      overrides.primary = primaryColor;
      overrides.ring = primaryColor;
    }

    if (accentColor) {
      overrides.accent = accentColor;
    }

    return overrides;
  }, [primaryColor, accentColor]);

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
  const typefaceAsset = useBrandingAsset("typeface")?.data;
  const typeface: Typeface | undefined =
    typefaceAsset === "inter" ? "inter" : undefined;
  const brandColors = useMemo<BrandColors | undefined>(() => {
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
      typeface={typeface}
      // The brand paints the document, not the unbranded root above it.
      paintDocument
    >
      {children}
    </ThemeProvider>
  );
}
