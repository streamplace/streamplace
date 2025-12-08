---
title: Streamplace ZeroCSS
description:
  Complete guide to Streamplace ZeroCSS - a React Native styling system using
  atoms and design tokens.
sidebar:
  order: 30
---

Streamplace ZeroCSS (zero-compile css) is a comprehensive styling system built
on design tokens and atomic utilities, providing a consistent and maintainable
approach to styling React Native components across all platforms.

## What is ZeroCSS?

**ZeroCSS** is Streamplace's zero-configuration, zero-configuration atomic
styling system designed specifically for React Native development. The name
reflects its core philosophy: zero setup, zero boilerplate, and zero friction
styling.

## Overview

Streamplace ZeroCSS is inspired by Tailwind CSS, and
[ALF](https://faineg.substack.com/p/how-i-accidentally-ruined-bluesky) on
[bluesky-social/social-app](https://github.com/bluesky-social/social-app/tree/main/src/alf),
featuring:

- **Atomic utilities**: Small, composable style objects
- **Design tokens**: Consistent spacing, colors, and typography
- **Platform awareness**: iOS, Android, and web-specific styles
- **Type safety**: Full TypeScript support
- **Performance**: Optimized for React Native's style system

## Core Concepts

### Atoms

Atoms are the building blocks of ZeroCSS. They provide direct access to design
tokens in a composable format:

```tsx
import { atoms } from "@streamplace/components";

const { layout, colors, spacing, borders } = atoms;

// Usage
<View
  style={[
    layout.flex.row,
    layout.flex.center,
    { backgroundColor: colors.gray[100] },
  ]}
/>;
```

### Pairify Function

The `pairify` function converts design tokens into style objects that can be
used directly:

```tsx
// Instead of writing:
{
  marginTop: 16;
}

// You can write:
mt[4]; // where 4 = 16px in the spacing scale
```

You can also directly make your own custom styles with `pairify`:

```tsx
let bw = pairify(
  {
    thin: 1,
    medium: 2,
    thick: 4,
  },
  "borderWidth",
)

// bw.thin will be { borderWidth: 1 }, etc.
<View style={[bw.thin]} />
```

## Design Tokens

### Colors

Streamplace uses a tailwind-like with semantic scales:

```tsx
// Primary colors
colors.primary[500]; // #3b82f6
colors.primary[600]; // #2563eb

// Grayscale
colors.gray[50]; // #fafafa
colors.gray[900]; // #171717

// Semantic colors
colors.destructive[500]; // #ef4444
colors.success[500]; // #22c55e
colors.warning[500]; // #f59e0b

// Platform-specific colors
colors.ios.systemBlue; // #007AFF
colors.android.materialBlue; // #2196F3
```

### Spacing

Consistent spacing scale following the 8px grid system: (em/rem is not available
in React Native)

```tsx
spacing[0]; // 0px
spacing[1]; // 4px
spacing[2]; // 8px
spacing[3]; // 12px
spacing[4]; // 16px
spacing[5]; // 20px
spacing[6]; // 24px
spacing[8]; // 32px
spacing[10]; // 40px
spacing[12]; // 48px
spacing[16]; // 64px
spacing[20]; // 80px
// ... up to spacing[96] = 384px
```

### Typography

Platform-aware typography system:

```tsx
// Universal typography (works on all platforms)
typography.universal.body.fontSize; // 16
typography.universal.body.lineHeight; // 24

// Platform-specific
typography.ios.body.fontFamily; // "SF Pro Text"
typography.android.body.fontFamily; // "Roboto"
```

## Atomic Utilities

### Layout

```tsx
import { layout } from "@streamplace/components";

// Flexbox
layout.flex.row; // { flexDirection: "row" }
layout.flex.column; // { flexDirection: "column" }
layout.flex.center; // { justifyContent: "center", alignItems: "center" }
layout.flex.spaceBetween; // { justifyContent: "space-between" }

// Positioning
layout.position.absolute; // { position: "absolute" }
layout.position.relative; // { position: "relative" }
```

### Spacing

```tsx
import {
  m,
  mt,
  mr,
  mb,
  ml,
  mx,
  my,
  p,
  pt,
  pr,
  pb,
  pl,
  px,
  py,
} from "@streamplace/components";

// Margin
m[4]; // { margin: 16 }
mt[2]; // { marginTop: 8 }
mx[3]; // { marginHorizontal: 12 }
my[4]; // { marginVertical: 16 }

// Padding
p[4]; // { padding: 16 }
pt[2]; // { paddingTop: 8 }
px[3]; // { paddingHorizontal: 12 }
py[4]; // { paddingVertical: 16 }
```

### Sizing

```tsx
import { w, h, sizes } from "@streamplace/components";

// Width and height
w[10]; // { width: 40 }
h[20]; // { height: 80 }
w.percent[50]; // { width: "50%" }
h.percent[100]; // { height: "100%" }

// Min/max sizes
sizes.minWidth[20]; // { minWidth: 80 }
sizes.maxHeight[64]; // { maxHeight: 256 }
```

### Colors

```tsx
import { bg, text, borders } from "@streamplace/components";

// Background colors
bg.gray[100]; // { backgroundColor: "#f5f5f5" }
bg.primary[500]; // { backgroundColor: "#3b82f6" }

// Text colors
text.gray[900]; // { color: "#171717" }
text.primary[600]; // { color: "#2563eb" }

// Border colors
borders.color.gray[300]; // { borderColor: "#d4d4d4" }
```

### Borders

```tsx
import { borders } from "@streamplace/components";

// Border widths
borders.width.thin; // { borderWidth: 1 }
borders.width.medium; // { borderWidth: 2 }
borders.width.thick; // { borderWidth: 4 }

// Border styles
borders.style.solid; // { borderStyle: "solid" }
borders.style.dashed; // { borderStyle: "dashed" }

// Directional borders
borders.top.width.thin; // { borderTopWidth: 1 }
borders.top.color.gray[300]; // { borderTopColor: "#d4d4d4" }
```

## Advanced Usage

### Responsive Styling

Use the `responsiveValue` utility for responsive designs:

```tsx
import { responsiveValue } from "@streamplace/components";
import { useWindowDimensions } from "react-native";

function MyComponent() {
  const { width } = useWindowDimensions();

  const padding = responsiveValue(
    {
      sm: 8,
      md: 16,
      lg: 24,
      xl: 32,
      default: 8,
    },
    width,
  );

  return <View style={{ padding }} />;
}
```

### Platform-Specific Styles

```tsx
import { platformStyle } from "@streamplace/components";

const styles = platformStyle({
  ios: {
    shadowColor: "#000",
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.1,
    shadowRadius: 4,
  },
  android: {
    elevation: 4,
  },
  web: {
    boxShadow: "0 2px 4px rgba(0,0,0,0.1)",
  },
  default: {
    borderWidth: 1,
    borderColor: "#e5e5e5",
  },
});
```

### Style Merging

```tsx
import { mergeStyles } from "@streamplace/components";

const baseStyles = { padding: 16, backgroundColor: "#fff" };
const variantStyles = { borderRadius: 8 };
const conditionalStyles = isActive ? { backgroundColor: "#f0f0f0" } : {};

const finalStyles = mergeStyles(baseStyles, variantStyles, conditionalStyles);
```

## Animations

For animations, use `react-native-reanimated` with ZeroCSS:

```tsx
import Animated, {
  useAnimatedStyle,
  useSharedValue,
} from "react-native-reanimated";
import { bg, p, r } from "@streamplace/components";

function AnimatedComponent() {
  const opacity = useSharedValue(1);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: opacity.value,
  }));

  return (
    <Animated.View style={[bg.white, p[4], r[2], animatedStyle]}>
      {/* content */}
    </Animated.View>
  );
}
```

## Best Practices

### 1. Use Atomic Utilities First

```tsx
// Good
<View style={[layout.flex.row, bg.gray[100], p[4]]} />

// Avoid
<View style={{
  flexDirection: "row",
  backgroundColor: "#f5f5f5",
  padding: 16
}} />
```

### 2. Combine Atoms with Custom Styles

```tsx
// Good
<View
  style={[
    layout.flex.center,
    bg.primary[500],
    { borderRadius: 8 }, // Custom style when needed
  ]}
/>
```

### 3. Use Style Arrays for Composition

```tsx
const baseStyles = [layout.flex.column, bg.white, p[4]];
const variantStyles = isActive
  ? [bg.primary[50], borders.color.primary[200]]
  : [];

<View style={[...baseStyles, ...variantStyles]} />;
```

### 4. Leverage TypeScript

```tsx
// All atoms are fully typed
import { atoms } from "@streamplace/components";

// TypeScript will provide autocomplete and catch errors
const { layout, colors } = atoms;
```

### 5. Follow the Design System

```tsx
// Use the spacing scale
p[4]; // 16px - follows 8px grid
mt[2]; // 8px

// Use semantic colors
bg.primary[500]; // Brand primary
text.destructive[600]; // Error text
```

## Migration Guide

### From Inline Styles

```tsx
// Before
<View style={{
  flexDirection: "row",
  justifyContent: "space-between",
  alignItems: "center",
  padding: 16,
  backgroundColor: "#fff"
}} />

// After
<View style={[
  layout.flex.row,
  layout.flex.spaceBetween,
  layout.flex.alignCenter,
  p[4],
  bg.white
]} />
```

### From StyleSheet

```tsx
// Before
const styles = StyleSheet.create({
  container: {
    flex: 1,
    padding: 16,
    backgroundColor: "#f5f5f5",
  },
});

// After
const containerStyles = [flex.values[1], p[4], bg.gray[100]];
```

## Testing

When testing components with ZeroCSS:

```tsx
import { render } from "@testing-library/react-native";
import { atoms } from "@streamplace/components";

const { layout } = atoms;

test("applies correct styles", () => {
  const { getByTestId } = render(
    <View
      testID="test-view"
      style={[layout.flex.center, { backgroundColor: "#fff" }]}
    />,
  );

  const view = getByTestId("test-view");
  expect(view).toHaveStyle({
    justifyContent: "center",
    alignItems: "center",
    backgroundColor: "#fff",
  });
});
```

## Performance Considerations

1. **Style Objects Are Cached**: Atomic utilities create cached style objects
   for optimal performance
2. **Avoid Dynamic Styles**: Use conditional arrays instead of dynamic style
   objects
3. **Use StyleSheet.create()**: For complex custom styles, still use
   StyleSheet.create()

```tsx
// Good - styles are cached
const styles = [layout.flex.row, bg.white, p[4]];

// Less optimal - creates new object each render
const styles = {
  flexDirection: "row",
  backgroundColor: colors.white,
  padding: spacing[4],
};
```

## Responsive Mobile Player Example

The responsive mobile player demonstrates advanced usage of ZeroCSS with
responsive layouts, animations, and platform-aware styling:

```tsx
// useResponsiveLayout.ts - Custom hook for responsive logic
import { useWindowDimensions } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { responsiveValue } from "@streamplace/components/src/lib/utils";

export function useResponsiveLayout() {
  const { width: screenWidth, height: screenHeight } = useWindowDimensions();
  const safeAreaInsets = useSafeAreaInsets();

  const isLandscape = screenWidth > screenHeight;
  const isMediumScreen = screenWidth >= 768;
  const shouldShowChatSidePanel = isLandscape && isMediumScreen;

  const chatPanelWidth = responsiveValue(
    {
      md: 320,
      lg: 400,
      xl: 480,
      default: 300,
    },
    screenWidth,
  );

  return {
    shouldShowChatSidePanel,
    chatPanelWidth,
    safeAreaInsets,
    // ... other responsive values
  };
}
```

```tsx
// Mobile UI Component with responsive styling
import { atoms, px, py, r } from "@streamplace/components";
import Animated, {
  useAnimatedStyle,
  withSpring,
} from "react-native-reanimated";

const { layout, borders, position, gap } = atoms;

export function MobileUi() {
  const { shouldShowChatSidePanel, chatPanelWidth, safeAreaInsets } =
    useResponsiveLayout();

  return (
    <>
      {/* Floating controls with safe area support */}
      <View
        style={[
          layout.position.absolute,
          {
            padding: 6.5,
            backgroundColor: "rgba(0, 0, 0, 0.6)",
            top: safeAreaInsets.top + 8,
          },
          r[2],
          position.left[1],
        ]}
      >
        <View style={[layout.flex.row, layout.flex.center, gap.all[2]]}>
          {/* Controls */}
        </View>
      </View>

      {/* Responsive chat panel */}
      {shouldShowChatSidePanel ? (
        <Animated.View
          style={[
            layout.position.absolute,
            position.right[0],
            {
              top: safeAreaInsets.top,
              bottom: safeAreaInsets.bottom,
              width: chatPanelWidth,
              backgroundColor: "rgba(0, 0, 0, 0.85)",
              borderLeftWidth: 1,
              borderLeftColor: "rgba(255, 255, 255, 0.1)",
            },
          ]}
        >
          <ChatPanel />
        </Animated.View>
      ) : (
        <View
          style={[
            layout.position.absolute,
            position.bottom[0],
            { width: "100%" },
          ]}
        >
          <Resizable>
            <ChatPanel />
          </Resizable>
        </View>
      )}

      {/* Desktop metadata bar */}
      {isDesktop && (
        <View
          style={[
            layout.position.absolute,
            {
              bottom: safeAreaInsets.bottom,
              left: safeAreaInsets.left,
              right: shouldShowChatSidePanel
                ? chatPanelWidth + safeAreaInsets.right
                : safeAreaInsets.right,
              height: 80,
              backgroundColor: "rgba(0, 0, 0, 0.9)",
              borderTopWidth: 1,
              borderTopColor: "rgba(255, 255, 255, 0.1)",
            },
            px[5],
            py[4],
          ]}
        >
          <View style={[layout.flex.row, layout.flex.center, gap.all[6]]}>
            {/* Metadata content */}
          </View>
        </View>
      )}
    </>
  );
}
```

This example demonstrates:

- **Responsive design**: Different layouts for mobile/tablet/desktop
- **Safe area handling**: Proper support for notched devices
- **Animations**: Smooth transitions using `react-native-reanimated`
- **Atomic styling**: Consistent use of design tokens
- **Platform awareness**: Adaptive layouts based on screen size

### 🎯 ZeroCSS Export Usage

The new `zero` export provides a clean namespace for all ZeroCSS utilities:

```tsx
// Import the zero namespace
import * as zero from "@streamplace/components/zero";

// All ZeroCSS utilities are available under the zero namespace
<View
  style={[
    zero.layout.flex.column,
    zero.bg.white,
    zero.borders.width.thin,
    zero.p[4],
    zero.r[2],
  ]}
/>;

// Responsive utilities
const padding = zero.responsiveValue(
  {
    sm: 8,
    md: 16,
    lg: 24,
    default: 8,
  },
  screenWidth,
);
```

## Customization and Overrides

ZeroCSS is designed to be extensible and customizable. Here are the main
approaches for overriding or configuring styles:

### 1. Style Array Composition (Recommended)

The most common way to override ZeroCSS styles is by using style arrays where
later styles override earlier ones:

```tsx
import { atoms, bg, p, r } from "@streamplace/components";

const { layout } = atoms;

// Base ZeroCSS styles with custom overrides
<View
  style={[
    layout.flex.column,
    bg.primary[500],
    p[4],
    r[2],
    // Custom overrides
    {
      backgroundColor: "#custom-color", // Overrides bg.primary[500]
      borderRadius: 12, // Overrides r[2]
    },
  ]}
/>;
```

### 2. Conditional Style Overrides

Use conditional logic to override styles based on state or props:

```tsx
import { bg, p, r, borders } from "@streamplace/components";

function CustomButton({ variant = "primary", disabled = false }) {
  return (
    <Pressable
      style={[
        // Base ZeroCSS styles
        bg.primary[500],
        p[4],
        r[2],
        borders.width.thin,

        // Conditional overrides
        variant === "secondary" && {
          backgroundColor: colors.gray[500],
          borderColor: colors.gray[600],
        },

        disabled && {
          backgroundColor: colors.gray[300],
          opacity: 0.6,
        },
      ]}
    >
      {/* content */}
    </Pressable>
  );
}
```

### 3. Custom Style Variants

Create your own style variants that extend ZeroCSS:

```tsx
import { atoms, bg, text, p, r } from "@streamplace/components";

const { layout } = atoms;

// Custom component variants
const buttonVariants = {
  primary: [bg.primary[500], text.white],
  secondary: [bg.gray[500], text.white],
  outline: [
    { backgroundColor: "transparent" },
    text.primary[600],
    { borderWidth: 1, borderColor: colors.primary[500] },
  ],
  ghost: [{ backgroundColor: "transparent" }, text.primary[600]],
};

function CustomButton({ variant = "primary", children, ...props }) {
  return (
    <Pressable
      style={[
        // Base ZeroCSS styles
        layout.flex.center,
        p[4],
        r[2],
        // Apply variant
        ...buttonVariants[variant],
      ]}
      {...props}
    >
      <Text>{children}</Text>
    </Pressable>
  );
}
```

### 4. Extending Design Tokens

For systematic customization, extend the design tokens:

```tsx
import { colors, spacing } from "@streamplace/components";

// Custom design tokens that extend ZeroCSS
const customTokens = {
  colors: {
    ...colors,
    brand: {
      50: "#f0f9ff",
      500: "#0ea5e9",
      900: "#0c4a6e",
    },
    custom: {
      accent: "#ff6b6b",
      warning: "#feca57",
    },
  },

  spacing: {
    ...spacing,
    // Add custom spacing values
    xs: 2, // 2px
    xxl: 96, // 96px
  },
};

// Use in components
<View
  style={[
    { backgroundColor: customTokens.colors.brand[500] },
    { padding: customTokens.spacing.xxl },
  ]}
/>;
```

### 5. Theme-Based Overrides

Create theme-based style overrides:

```tsx
import { platformStyle } from "@streamplace/components";

// Create theme-aware styles
const createThemedStyles = (isDark: boolean) => ({
  container: [bg.white, isDark && { backgroundColor: "#1a1a1a" }],

  text: [text.gray[900], isDark && { color: "#ffffff" }],

  border: [borders.color.gray[200], isDark && { borderColor: "#374151" }],
});

function ThemedComponent({ isDark = false }) {
  const styles = createThemedStyles(isDark);

  return (
    <View style={styles.container}>
      <Text style={styles.text}>Themed content</Text>
    </View>
  );
}
```

### 6. Component-Specific Style Systems

For complex components, create dedicated style systems:

```tsx
import { mergeStyles, atoms, bg, text, p, r } from "@streamplace/components";

const { layout } = atoms;

// Component-specific style system
const cardStyles = {
  base: [
    layout.flex.column,
    bg.white,
    r[3],
    p[6],
    { shadowColor: "#000", shadowOpacity: 0.1 },
  ],

  variants: {
    elevated: [{ shadowOffset: { width: 0, height: 4 } }],
    flat: [{ shadowOpacity: 0 }],
    outlined: [
      { backgroundColor: "transparent" },
      borders.width.thin,
      borders.color.gray[200],
    ],
  },

  sizes: {
    sm: [p[4], r[2]],
    md: [p[6], r[3]],
    lg: [p[8], r[4]],
  },
};

function Card({ variant = "elevated", size = "md", style, children }) {
  const cardStyle = mergeStyles(
    ...cardStyles.base,
    ...cardStyles.variants[variant],
    ...cardStyles.sizes[size],
    style, // Allow external style overrides
  );

  return <View style={cardStyle}>{children}</View>;
}
```

### 7. Global Style Overrides

For app-wide style overrides, create a style override system:

```tsx
import { colors, spacing } from "@streamplace/components";

// Global style overrides
const globalOverrides = {
  // Override default colors
  colors: {
    primary: {
      ...colors.primary,
      500: "#your-brand-color",
    },
  },

  // Override default spacing
  spacing: {
    ...spacing,
    // Make default padding larger
    4: 20, // instead of 16
  },

  // Add custom shadows
  shadows: {
    card: {
      shadowColor: "#000",
      shadowOffset: { width: 0, height: 2 },
      shadowOpacity: 0.25,
      shadowRadius: 3.84,
      elevation: 5,
    },
  },
};

// Use throughout your app
export { globalOverrides as customTheme };
```

### 8. Runtime Style Configuration

For dynamic style configuration:

````tsx
import { responsiveValue } from "@streamplace/components";

// Runtime configuration
const styleConfig = {
  brandColor: "#your-color",
  borderRadius: 8,
  spacing: {
    small: 8,
    medium: 16,
    large: 24,
  }
};

function ConfigurableComponent({ config = styleConfig }) {
  return (
    <View style={[

## Theme Customization

ZeroCSS includes a comprehensive theme system that allows you to customize the default design tokens and create consistent theming across your app.

### ThemeProvider Setup

Wrap your app with the `ThemeProvider` to enable theming:

```tsx
import { ThemeProvider } from "@streamplace/components";

function App() {
  return (
    <ThemeProvider defaultTheme="system">
      {/* Your app content */}
    </ThemeProvider>
  );
}
````

### Basic Theme Configuration

The ThemeProvider accepts several configuration options:

```tsx
<ThemeProvider
  defaultTheme="system" // "light" | "dark" | "system"
  forcedTheme="dark" // Force a specific theme
>
  <YourApp />
</ThemeProvider>
```

### Using Theme in Components

Access the current theme using the `useTheme` hook:

```tsx
// Option 1: Import from main components
import { useTheme } from "@streamplace/components";

// Option 2: Import from ZeroCSS namespace
import * as zero from "@streamplace/components/zero";
const { useTheme } = zero;

function ThemedComponent() {
  const { theme, isDark, setTheme } = useTheme();

  return (
    <View
      style={{
        backgroundColor: theme.colors.background,
        borderColor: theme.colors.border,
        padding: theme.spacing[4],
        borderRadius: theme.borderRadius.md,
      }}
    >
      <Text style={{ color: theme.colors.text }}>
        Current theme: {isDark ? "Dark" : "Light"}
      </Text>

      <Pressable onPress={() => setTheme(isDark ? "light" : "dark")}>
        <Text>Toggle Theme</Text>
      </Pressable>
    </View>
  );
}
```

### Creating Custom Themes

You can create custom theme configurations by extending the base theme:

```tsx
import { lightTheme, darkTheme } from "@streamplace/components";

// Custom light theme
const customLightTheme = {
  ...lightTheme,
  colors: {
    ...lightTheme.colors,
    primary: "#your-brand-color",
    background: "#fafafa",
    // Override any color tokens
  },
  spacing: {
    ...lightTheme.spacing,
    // Add custom spacing values
    xs: 2,
    xxl: 96,
  },
};

// Custom dark theme
const customDarkTheme = {
  ...darkTheme,
  colors: {
    ...darkTheme.colors,
    primary: "#your-brand-color-dark",
    background: "#0a0a0a",
  },
};
```

### Injecting Custom Themes into ThemeProvider

To use custom themes with the `useTheme` hook, you need to create a custom
ThemeProvider wrapper:

```tsx
import React, { createContext, useContext, useMemo, useState } from "react";
import { useColorScheme } from "react-native";
import {
  ThemeProvider as BaseThemeProvider,
  useTheme as useBaseTheme,
} from "@streamplace/components";

// Create custom theme context
const CustomThemeContext = createContext(null);

// Custom theme provider that injects your themes
export function CustomThemeProvider({
  children,
  lightTheme: customLight,
  darkTheme: customDark,
  defaultTheme = "system",
}) {
  const systemColorScheme = useColorScheme();
  const [currentTheme, setCurrentTheme] = useState(defaultTheme);

  // Determine active theme
  const activeTheme = useMemo(() => {
    const isDark =
      currentTheme === "dark" ||
      (currentTheme === "system" && systemColorScheme === "dark");

    return isDark ? customDark : customLight;
  }, [currentTheme, systemColorScheme, customLight, customDark]);

  const value = {
    theme: activeTheme,
    isDark: activeTheme === customDark,
    currentTheme,
    setTheme: setCurrentTheme,
    toggleTheme: () => {
      setCurrentTheme((prev) => (prev === "light" ? "dark" : "light"));
    },
  };

  return (
    <CustomThemeContext.Provider value={value}>
      <BaseThemeProvider defaultTheme={currentTheme}>
        {children}
      </BaseThemeProvider>
    </CustomThemeContext.Provider>
  );
}

// Custom hook to access your custom themes
export function useCustomTheme() {
  const context = useContext(CustomThemeContext);
  if (!context) {
    throw new Error("useCustomTheme must be used within CustomThemeProvider");
  }
  return context;
}
```

### Using Custom Themes in Your App

```tsx
import { CustomThemeProvider, useCustomTheme } from "./path/to/your/theme";

// Define your custom themes
const myLightTheme = {
  colors: {
    primary: "#ff6b6b",
    background: "#ffffff",
    text: "#333333",
    // ... other colors
  },
  spacing: {
    /* custom spacing */
  },
  // ... other tokens
};

const myDarkTheme = {
  colors: {
    primary: "#ff8787",
    background: "#1a1a1a",
    text: "#ffffff",
    // ... other colors
  },
  spacing: {
    /* custom spacing */
  },
  // ... other tokens
};

// App setup
function App() {
  return (
    <CustomThemeProvider
      lightTheme={myLightTheme}
      darkTheme={myDarkTheme}
      defaultTheme="system"
    >
      <YourApp />
    </CustomThemeProvider>
  );
}

// Using custom theme in components
function ThemedComponent() {
  const { theme, isDark, setTheme } = useCustomTheme();

  return (
    <View
      style={{
        backgroundColor: theme.colors.background,
        padding: theme.spacing?.[4] || 16,
      }}
    >
      <Text style={{ color: theme.colors.text }}>
        Using custom {isDark ? "dark" : "light"} theme!
      </Text>

      <Pressable onPress={() => setTheme(isDark ? "light" : "dark")}>
        <Text style={{ color: theme.colors.primary }}>Switch Theme</Text>
      </Pressable>
    </View>
  );
}
```

### Alternative: Theme Override Pattern

For simpler customization, you can override themes at the component level:

```tsx
import { useTheme } from "@streamplace/components";
// or: import * as zero from "@streamplace/components/zero";

function ComponentWithCustomTheme() {
  const { theme: baseTheme, isDark } = useTheme();

  // Override specific theme values
  const customTheme = useMemo(
    () => ({
      ...baseTheme,
      colors: {
        ...baseTheme.colors,
        primary: "#your-brand-color",
        background: isDark ? "#0a0a0a" : "#fafafa",
      },
    }),
    [baseTheme, isDark],
  );

  return (
    <View
      style={{
        backgroundColor: customTheme.colors.background,
        borderColor: customTheme.colors.primary,
      }}
    >
      <Text style={{ color: customTheme.colors.text }}>
        Custom themed component
      </Text>
    </View>
  );
}
```

### Theme-Aware Component Styles

Create components that automatically adapt to theme changes:

```tsx
// Option 1: Import from main components
import { createThemedStyles } from "@streamplace/components";

// Option 2: Import from ZeroCSS namespace
import * as zero from "@streamplace/components/zero";
const { createThemedStyles } = zero;

const useButtonStyles = createThemedStyles((theme, styles, icons) => ({
  primary: {
    backgroundColor: theme.colors.primary,
    borderRadius: theme.borderRadius.md,
    paddingHorizontal: theme.spacing[4],
    paddingVertical: theme.spacing[3],
    ...styles.shadow.sm,
  },

  secondary: {
    backgroundColor: theme.colors.secondary,
    borderWidth: 1,
    borderColor: theme.colors.border,
    borderRadius: theme.borderRadius.md,
    paddingHorizontal: theme.spacing[4],
    paddingVertical: theme.spacing[3],
  },

  text: {
    color: theme.colors.primaryForeground,
    fontSize: theme.typography.universal.body.fontSize,
    fontWeight: theme.typography.universal.body.fontWeight,
  },
}));

function ThemedButton({ variant = "primary", children }) {
  const styles = useButtonStyles();

  return (
    <Pressable style={styles[variant]}>
      <Text style={styles.text}>{children}</Text>
    </Pressable>
  );
}
```

### Platform-Specific Theme Customization

Customize themes for specific platforms:

```tsx
import { Platform } from "react-native";
import { colors } from "@streamplace/components";

const platformTheme = {
  colors: {
    primary: Platform.select({
      ios: colors.ios.systemBlue,
      android: colors.android.materialBlue,
      default: colors.primary[500],
    }),

    destructive: Platform.select({
      ios: colors.ios.systemRed,
      android: colors.destructive[500],
      default: colors.destructive[500],
    }),
  },
};
```

### Runtime Theme Switching

Implement dynamic theme switching in your app:

```tsx
import { useTheme } from "@streamplace/components";
// or: import * as zero from "@streamplace/components/zero";

function ThemeToggler() {
  const { currentTheme, setTheme, toggleTheme } = useTheme();

  return (
    <View>
      <Text>Current: {currentTheme}</Text>

      <Pressable onPress={() => setTheme("light")}>
        <Text>Light</Text>
      </Pressable>

      <Pressable onPress={() => setTheme("dark")}>
        <Text>Dark</Text>
      </Pressable>

      <Pressable onPress={() => setTheme("system")}>
        <Text>System</Text>
      </Pressable>

      <Pressable onPress={toggleTheme}>
        <Text>Toggle</Text>
      </Pressable>
    </View>
  );
}
```

### Accessing Theme Outside Components

For utilities or functions that need theme access outside of React components:

```tsx
import { lightTheme, darkTheme } from "@streamplace/components";
// or: import * as zero from "@streamplace/components/zero";

// Utility function that uses theme
function createUtilityStyles(isDark = false) {
  const theme = isDark ? darkTheme : lightTheme;

  return {
    container: {
      backgroundColor: theme.colors.background,
      padding: theme.spacing[4],
    },
    text: {
      color: theme.colors.text,
      fontSize: theme.typography.universal.body.fontSize,
    },
  };
}
```

The theme system provides complete control over your app's visual design while
maintaining the convenience and consistency of ZeroCSS atomic utilities.

Streamplace ZeroCSS provides a powerful, consistent, and maintainable approach
to styling React Native components. By leveraging atomic utilities and design
tokens, you can build interfaces that are both beautiful and performant across
all platforms.
