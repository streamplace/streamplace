import { ImageStyle, StyleSheet, TextStyle, ViewStyle } from "react-native";

// React Native style utilities
type Style = ViewStyle | TextStyle | ImageStyle;

/**
 * Merges React Native styles similar to how cn() merges CSS classes
 * Handles arrays, objects, and falsy values
 */
export function mergeStyles(
  ...styles: (Style | Style[] | undefined | null | false)[]
): Style {
  const validStyles = styles.filter(Boolean).flat() as Style[];
  return StyleSheet.flatten(validStyles) || {};
}

/**
 * Creates a style merger function that includes base styles
 * Useful for component variants
 */
export function createStyleMerger(baseStyle: Style) {
  return (...styles: (Style | Style[] | undefined | null | false)[]) => {
    return mergeStyles(baseStyle, ...styles);
  };
}

/**
 * Conditionally applies styles based on boolean conditions
 */
export function conditionalStyle(
  condition: boolean,
  trueStyle: Style,
  falseStyle?: Style,
): Style | undefined {
  return condition ? trueStyle : falseStyle;
}

/**
 * Creates responsive values based on screen dimensions
 */
export function responsiveValue<T>(
  values: {
    sm?: T;
    md?: T;
    lg?: T;
    xl?: T;
    default: T;
  },
  screenWidth: number,
): T {
  if (screenWidth >= 1280 && values.xl !== undefined) return values.xl;
  if (screenWidth >= 1024 && values.lg !== undefined) return values.lg;
  if (screenWidth >= 768 && values.md !== undefined) return values.md;
  if (screenWidth >= 640 && values.sm !== undefined) return values.sm;
  return values.default;
}

/**
 * Creates platform-specific styles
 */
export function platformStyle(styles: {
  ios?: Style;
  android?: Style;
  web?: Style;
  default?: Style;
}): Style {
  const Platform = require("react-native").Platform;

  if (Platform.OS === "ios" && styles.ios) return styles.ios;
  if (Platform.OS === "android" && styles.android) return styles.android;
  if (Platform.OS === "web" && styles.web) return styles.web;
  return styles.default || {};
}

/**
 * Converts hex color to rgba
 */
export function hexToRgba(hex: string, alpha: number = 1): string {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
  if (!result) return hex;

  const r = parseInt(result[1], 16);
  const g = parseInt(result[2], 16);
  const b = parseInt(result[3], 16);

  return `rgba(${r}, ${g}, ${b}, ${alpha})`; // token-ok: color utility implementation
}

/**
 * Creates a debounced function for performance
 */
export function debounce<T extends (...args: any[]) => any>(
  func: T,
  delay: number,
): (...args: Parameters<T>) => void {
  let timeoutId: NodeJS.Timeout;

  return (...args: Parameters<T>) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => func(...args), delay);
  };
}

/**
 * Creates a throttled function for performance
 */
export function throttle<T extends (...args: any[]) => any>(
  func: T,
  delay: number,
): (...args: Parameters<T>) => void {
  let lastCall = 0;

  return (...args: Parameters<T>) => {
    const now = Date.now();
    if (now - lastCall >= delay) {
      lastCall = now;
      func(...args);
    }
  };
}

/**
 * Type-safe component prop forwarding
 */
export function forwardProps<T extends Record<string, any>>(
  props: T,
  omit: (keyof T)[],
): Omit<T, keyof T extends string ? keyof T : never> {
  const result = { ...props };
  omit.forEach((key) => delete result[key]);
  return result;
}
