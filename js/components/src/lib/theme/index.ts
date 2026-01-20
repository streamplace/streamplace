// Main theme system exports
export {
  ThemeProvider,
  createThemeColors,
  createThemeIcons,
  createThemedStyles,
  darkTheme,
  lightTheme,
  usePlatformTypography,
  useTheme,
  type Theme,
  type ThemeIcons,
} from "./theme";

// Branded theme provider
export { BrandedThemeProvider } from "./branded-theme-provider";

// Design tokens
export {
  animations,
  borderRadius,
  breakpoints,
  colors,
  shadows,
  spacing,
  touchTargets,
  typography,
  type Animations,
  type BorderRadius,
  type Breakpoints,
  type Colors,
  type Shadows,
  type Spacing,
  type TouchTargets,
  type Typography,
} from "./tokens";

// Utility atoms
export {
  borders,
  getPlatformTypography,
  iconSizes,
  layout,
  typographyAtoms,
} from "./atoms";

// Convenience re-exports
export * as atoms from "./atoms";
export * as tokens from "./tokens";
