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
  borderAlphas,
  borderRadius,
  breakpoints,
  colors,
  fontFamilies,
  fontWeights,
  motion,
  scrims,
  shadows,
  spacing,
  statusColors,
  surfaces,
  tabularNums,
  textAlphas,
  touchTargets,
  typeScale,
  typography,
  type Animations,
  type BorderAlphas,
  type BorderRadius,
  type Breakpoints,
  type Colors,
  type FontWeights,
  type Motion,
  type Shadows,
  type Spacing,
  type StatusColors,
  type Surfaces,
  type TextAlphas,
  type TouchTargets,
  type TypeScale,
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

// Branded chrome derivation
export {
  DEFAULT_CHROME,
  deriveChrome,
  parseHexColor,
  type ChromeColors,
  type DerivedChrome,
} from "./chrome";
