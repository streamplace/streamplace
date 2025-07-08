/**
 * @streamplace/components/ui - Streamplace ZeroCSS
 *
 * Clean export path for ZeroCSS styling utilities, design tokens, and atomic styles.
 * ZeroCSS provides a zero-config, atomic styling system optimized for React Native.
 */

// Export the most commonly used ZeroCSS utilities
export {
  // Core atoms object
  atoms,
  // Common shorthand utilities
  bg,
  // Border utilities
  borders,
  bottom,
  // Flex utilities
  flex,
  // Gap utilities (React Native 0.71+)
  gap,
  h,
  // Layout utilities
  layout,
  left,
  m,
  mb,
  ml,
  mr,
  mt,
  mx,
  my,
  p,
  pb,
  pl,
  // Position utilities
  position,
  pr,
  pt,
  px,
  py,
  r,
  right,
  text,
  top,
  w,
} from "../lib/theme/atoms";

// Export ZeroCSS design tokens
export {
  borderRadius,
  breakpoints,
  colors,
  shadows,
  spacing,
  typography,
} from "../lib/theme/tokens";

// Export ZeroCSS utility functions
export {
  debounce,
  mergeStyles,
  platformStyle,
  responsiveValue,
} from "../lib/utils";

// Export ZeroCSS theme system
export {
  ThemeProvider,
  createThemeColors,
  createThemeIcons,
  createThemeStyles,
  createThemedStyles,
  darkTheme,
  lightTheme,
  usePlatformTypography,
  useTheme,
  type Theme,
  type ThemeIcons,
  type ThemeStyles,
} from "../lib/theme/theme";

// Namespace exports for power users
export * as theme from "../lib/theme";
export * as atomsNS from "../lib/theme/atoms";
export * as tokens from "../lib/theme/tokens";
export * as utils from "../lib/utils";
