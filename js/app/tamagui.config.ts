import { config as configBase } from "@tamagui/config/v3";
import { createTamagui, createFont } from "tamagui";

const sizes = {
  "1": 11,
  "2": 12,
  "3": 13,
  "4": 14,
  "5": 13,
  "6": 15,
  "7": 20,
  "8": 23,
  "9": 32,
  "10": 44,
  "11": 55,
  "12": 62,
  "13": 72,
  "14": 92,
  "15": 114,
  "16": 134,
};

const lineHeights = {
  "1": 22,
  "2": 23,
  "3": 24,
  "4": 25,
  "5": 24,
  "6": 27,
  "7": 32,
  "8": 35,
  "9": 40,
  "10": 53,
  "11": 66,
  "12": 73,
  "13": 84,
  "14": 106,
  "15": 130,
  "16": 152,
};

const bodyFont = createFont({
  ...configBase.fonts.body,
  family: `FiraSans-Medium`,
  lineHeight: lineHeights,
  size: sizes,
});

const headingFont = createFont({
  ...configBase.fonts.heading,
  family: `FiraSans-Medium`,
  lineHeight: lineHeights,
  size: sizes,
});

const codeFont = createFont({
  family: `FiraCode-Medium`,
  size: sizes,
  lineHeight: lineHeights,
  weight: {
    1: "300",
    // 2 will be 300
    3: "600",
  },
  letterSpacing: {
    1: 0,
    2: -1,
    // 3 will be -1
  },
});

const aquareumConfig = {
  ...configBase,
  fonts: {
    ...configBase.fonts,
    heading: headingFont,
    body: bodyFont,
    mono: codeFont,
  },
  media: {
    xs: { maxWidth: 660 },
    gtXs: { minWidth: 660 + 1 },
    sm: { maxWidth: 860 },
    gtSm: { minWidth: 860 + 1 },
    md: { maxWidth: 980 },
    gtMd: { minWidth: 980 + 1 },
    lg: { maxWidth: 1120 },
    gtLg: { minWidth: 1120 + 1 },
    short: { maxHeight: 820 },
    tall: { minHeight: 820 },
    hoverNone: { hover: "none" },
    pointerCoarse: { pointer: "coarse" },
  },
  themes: {
    ...configBase.themes,
    dark: {
      ...configBase.themes.dark,
      accentColor: "rgb(189 110 134)",
    },
  },
};

console.log(JSON.stringify(configBase.themes.dark, null, 2));

const config = createTamagui(aquareumConfig);

export { config };
export default config;

export type Conf = typeof config;

declare module "tamagui" {
  interface TamaguiCustomConfig extends Conf {}
}
