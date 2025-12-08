---
title: ZeroCSS Quick Reference
description:
  Quick reference for Streamplace ZeroCSS - common patterns and utilities.
sidebar:
  order: 31
---

## ZeroCSS Quick Import

```tsx
// Traditional imports (current stable)
import { zero } from "@streamplace/components";
{
  atoms,
  bg,
  text,
  m, mt, mr, mb, ml, mx, my,
  p, pt, pr, pb, pl, px, py,
  w, h, r,
  layout,
  borders,
  flex,
  gap
} = zero
```

## Common Patterns

### Flex Layout

```tsx
// Traditional imports
style={[layout.flex.row, layout.flex.center]}
style={[layout.flex.column, layout.flex.spaceBetween]}
style={[flex.values[1]]}

// ZeroCSS namespace
style={[zero.layout.flex.row, zero.layout.flex.center]}
style={[zero.layout.flex.column, zero.layout.flex.spaceBetween]}
style={[zero.flex.values[1]]}
```

### Spacing

```tsx
// Margin
m[4]; // all sides: 16px
mt[2]; // top: 8px
mx[3]; // horizontal: 12px
my[4]; // vertical: 16px

// Padding
p[4]; // all sides: 16px
pt[2]; // top: 8px
px[3]; // horizontal: 12px
py[4]; // vertical: 16px

// Gap (React Native 0.71+)
gap.all[4]; // gap: 16px
gap.row[2]; // rowGap: 8px
gap.column[3]; // columnGap: 12px
```

### Colors

```tsx
// Background
bg.white; // white
bg.gray[100]; // light gray
bg.primary[500]; // brand primary
bg.destructive[500]; // error red

// Text
text.gray[900]; // dark text
text.primary[600]; // primary text
text.white; // white text

// Borders
borders.color.gray[300]; // border color
borders.width.thin; // 1px border
```

### Sizing

```tsx
// Fixed sizes
w[10]; // width: 40px
h[20]; // height: 80px

// Percentage
w.percent[100]; // width: 100%
h.percent[50]; // height: 50%

// Min/max
sizes.minWidth[20]; // minWidth: 80px
sizes.maxHeight[64]; // maxHeight: 256px
```

### Positioning

```tsx
// Absolute positioning
layout.position.absolute;
position.top[4]; // top: 16px
position.right[0]; // right: 0px
position.bottom[2]; // bottom: 8px
position.left[4]; // left: 16px

// Common layouts
layouts.fullScreen; // fill entire screen
layouts.centered; // centered content
layouts.overlay; // modal overlay
```

### Borders & Radius

```tsx
// Border radius
r[2]; // borderRadius: 8px

// Borders
borders.width.thin; // borderWidth: 1px
borders.style.solid; // borderStyle: solid
borders.color.gray[300]; // borderColor: #d4d4d4

// Directional borders
borders.top.width.thin; // borderTopWidth: 1px
borders.bottom.color.gray[200]; // borderBottomColor: #e5e5e5
```

## Spacing Scale

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
```

## Color Palette

### Grayscale

```tsx
gray[50]; // #fafafa (lightest)
gray[100]; // #f5f5f5
gray[200]; // #e5e5e5
gray[300]; // #d4d4d4
gray[400]; // #a3a3a3
gray[500]; // #737373
gray[600]; // #525252
gray[700]; // #404040
gray[800]; // #262626
gray[900]; // #171717 (darkest)
```

### Primary Colors

```tsx
primary[100]; // #dbeafe (light)
primary[500]; // #3b82f6 (main)
primary[700]; // #1d4ed8 (dark)
```

### Semantic Colors

```tsx
success[500]; // #22c55e
warning[500]; // #f59e0b
destructive[500]; // #ef4444
```

## Component Examples

### Card

```tsx
// Traditional imports
<View style={[
  bg.white,
  borders.width.thin,
  borders.color.gray[200],
  r[2],
  p[4],
  shadows.sm
]} />

// ZeroCSS namespace
<View style={[
  zero.bg.white,
  zero.borders.width.thin,
  zero.borders.color.gray[200],
  zero.r[2],
  zero.p[4],
  zero.shadows.sm
]} />
```

### Button

```tsx
// Traditional imports
<Pressable style={[
  layout.flex.center,
  bg.primary[500],
  px[4],
  py[2],
  r[1]
]}>
  <Text style={[text.white]}>Button</Text>
</Pressable>

// ZeroCSS namespace
<Pressable style={[
  zero.layout.flex.center,
  zero.bg.primary[500],
  zero.px[4],
  zero.py[2],
  zero.r[1]
]}>
  <Text style={[zero.text.white]}>Button</Text>
</Pressable>
```

### Modal

```tsx
<View style={[layouts.overlay]}>
  <View style={[bg.white, r[3], p[6], mx[4], shadows.xl]}>
    {/* Modal content */}
  </View>
</View>
```

### List Item

```tsx
<View
  style={[
    layout.flex.row,
    layout.flex.spaceBetween,
    layout.flex.alignCenter,
    px[4],
    py[3],
    borders.bottom.width.thin,
    borders.bottom.color.gray[200],
  ]}
>
  <Text>Item</Text>
  <Text style={[text.gray[500]]}>Value</Text>
</View>
```

### Header

```tsx
<View
  style={[
    layout.flex.row,
    layout.flex.spaceBetween,
    layout.flex.alignCenter,
    bg.white,
    px[4],
    py[3],
    borders.bottom.width.thin,
    borders.bottom.color.gray[200],
  ]}
>
  <Text style={[text.gray[900], fontSize.lg]}>Title</Text>
  <Pressable>
    <Text style={[text.primary[600]]}>Action</Text>
  </Pressable>
</View>
```

## ZeroCSS Responsive Usage

```tsx
// Traditional import
import { responsiveValue } from "@streamplace/components";
import { useWindowDimensions } from "react-native";

// ZeroCSS namespace
import * as zero from "@streamplace/components/zero";

const { width } = useWindowDimensions();

// Traditional usage
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

// ZeroCSS namespace usage
const padding = zero.responsiveValue(
  {
    sm: 8,
    md: 16,
    lg: 24,
    xl: 32,
    default: 8,
  },
  width,
);
```

## ZeroCSS Platform Specific

```tsx
// Traditional import
import { platformStyle } from "@streamplace/components";

// ZeroCSS namespace
import * as zero from "@streamplace/components/zero";

// Traditional usage
const shadowStyles = platformStyle({
  ios: { shadowColor: "#000", shadowOffset: { width: 0, height: 2 } },
  android: { elevation: 4 },
  web: { boxShadow: "0 2px 4px rgba(0,0,0,0.1)" },
  default: {},
});

// ZeroCSS namespace usage
const shadowStyles = zero.platformStyle({
  ios: { shadowColor: "#000", shadowOffset: { width: 0, height: 2 } },
  android: { elevation: 4 },
  web: { boxShadow: "0 2px 4px rgba(0,0,0,0.1)" },
  default: {},
});
```

## ZeroCSS Animation Support

```tsx
import Animated, { useAnimatedStyle } from "react-native-reanimated";

const animatedStyle = useAnimatedStyle(() => ({
  opacity: opacity.value,
}));

<Animated.View style={[bg.white, p[4], r[2], animatedStyle]} />;
```

## Debugging Tips

1. **Check style order**: Later styles override earlier ones
2. **Use arrays**: `[style1, style2, conditionalStyle]`
3. **Inspect with Flipper**: Use React Native Debugger
4. **Test on device**: Simulator styles may differ

## Performance Tips

1. **Cache style arrays**: Don't create new arrays each render
2. **Use conditional arrays**:
   `[baseStyles, ...(condition ? activeStyles : [])]`
3. **Prefer ZeroCSS atoms**: They're pre-optimized and cached
4. **Avoid dynamic styles**: Use conditional arrays instead

## ZeroCSS Theme Usage

### Theme Provider Setup

```tsx
import { ThemeProvider } from "@streamplace/components";

<ThemeProvider defaultTheme="system">
  <YourApp />
</ThemeProvider>;
```

### Using Theme Hook

```tsx
import { useTheme } from "@streamplace/components";

function ThemedComponent() {
  const { theme, isDark, setTheme } = useTheme();

  return (
    <View
      style={{
        backgroundColor: theme.colors.background,
        padding: theme.spacing[4],
      }}
    >
      <Text style={{ color: theme.colors.text }}>
        Theme: {isDark ? "Dark" : "Light"}
      </Text>
    </View>
  );
}
```

### Theme-Aware Styles

```tsx
import { createThemedStyles } from "@streamplace/components";

const useStyles = createThemedStyles((theme) => ({
  button: {
    backgroundColor: theme.colors.primary,
    borderRadius: theme.borderRadius.md,
    padding: theme.spacing[3],
  },
}));

function Button() {
  const styles = useStyles();
  return <Pressable style={styles.button} />;
}
```

### Theme Switching

```tsx
function ThemeToggle() {
  const { currentTheme, setTheme, toggleTheme } = useTheme();

  return (
    <Pressable onPress={toggleTheme}>
      <Text>Current: {currentTheme}</Text>
    </Pressable>
  );
}
```

## Custom Theme Injection

### Custom Theme Provider

```tsx
import { createContext, useContext, useMemo, useState } from "react";
import { useColorScheme } from "react-native";
import { ThemeProvider as BaseThemeProvider } from "@streamplace/components";

const CustomThemeContext = createContext(null);

export function CustomThemeProvider({
  children,
  lightTheme,
  darkTheme,
  defaultTheme = "system",
}) {
  const systemColorScheme = useColorScheme();
  const [currentTheme, setCurrentTheme] = useState(defaultTheme);

  const activeTheme = useMemo(() => {
    const isDark =
      currentTheme === "dark" ||
      (currentTheme === "system" && systemColorScheme === "dark");
    return isDark ? darkTheme : lightTheme;
  }, [currentTheme, systemColorScheme, lightTheme, darkTheme]);

  const value = {
    theme: activeTheme,
    isDark: activeTheme === darkTheme,
    currentTheme,
    setTheme: setCurrentTheme,
  };

  return (
    <CustomThemeContext.Provider value={value}>
      <BaseThemeProvider defaultTheme={currentTheme}>
        {children}
      </BaseThemeProvider>
    </CustomThemeContext.Provider>
  );
}

export function useCustomTheme() {
  return useContext(CustomThemeContext);
}
```

### Using Custom Themes

```tsx
const myLightTheme = {
  colors: { primary: "#ff6b6b", background: "#fff" },
  spacing: {
    /* custom spacing */
  },
};

const myDarkTheme = {
  colors: { primary: "#ff8787", background: "#1a1a1a" },
  spacing: {
    /* custom spacing */
  },
};

function App() {
  return (
    <CustomThemeProvider lightTheme={myLightTheme} darkTheme={myDarkTheme}>
      <YourApp />
    </CustomThemeProvider>
  );
}

function Component() {
  const { theme, isDark } = useCustomTheme();

  return (
    <View
      style={{
        backgroundColor: theme.colors.background,
        borderColor: theme.colors.primary,
      }}
    >
      <Text>Custom theme active!</Text>
    </View>
  );
}
```
