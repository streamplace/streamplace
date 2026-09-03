import { borderAlphas, surfaces, textAlphas } from "./tokens";

/**
 * Chrome colors: the neutral scaffolding of the UI (surfaces, text, hairline
 * borders) derived from one background and one foreground per color scheme.
 * The defaults in tokens.ts are exactly such a derivation (a near-black base
 * stepped toward white; white text at fixed alphas), so a node that sets its
 * own pair through branding gets the same ramp structure in its own colors.
 */
export interface ChromeColors {
  /** Base surface, e.g. "#0a0a0b". */
  background: string;
  /** Text and border color, e.g. "#ffffff". */
  foreground: string;
}

export interface DerivedChrome {
  surface: { 0: string; 1: string; 2: string; 3: string };
  text: { 1: string; 2: string; 3: string; 4: string };
  border: { subtle: string; default: string; strong: string };
}

type RGB = { r: number; g: number; b: number };

/** Parse #rgb / #rrggbb / #rrggbbaa (alpha ignored). Null when invalid. */
export function parseHexColor(input: string | undefined | null): RGB | null {
  if (!input) return null;
  const hex = input.trim().replace(/^#/, "");
  if (!/^[0-9a-fA-F]{3}$|^[0-9a-fA-F]{6}$|^[0-9a-fA-F]{8}$/.test(hex)) {
    return null;
  }
  const full =
    hex.length === 3
      ? hex
          .split("")
          .map((c) => c + c)
          .join("")
      : hex.slice(0, 6);
  return {
    r: parseInt(full.slice(0, 2), 16),
    g: parseInt(full.slice(2, 4), 16),
    b: parseInt(full.slice(4, 6), 16),
  };
}

const toHex = ({ r, g, b }: RGB) =>
  "#" +
  [r, g, b]
    .map((v) =>
      Math.round(Math.max(0, Math.min(255, v)))
        .toString(16)
        .padStart(2, "0"),
    )
    .join("");

const mix = (a: RGB, b: RGB, t: number): RGB => ({
  r: a.r + (b.r - a.r) * t,
  g: a.g + (b.g - a.g) * t,
  b: a.b + (b.b - a.b) * t,
});

const rgba = ({ r, g, b }: RGB, alpha: number) =>
  `rgba(${Math.round(r)},${Math.round(g)},${Math.round(b)},${alpha})`;

/** Relative luminance, for telling a dark pair from a light one. */
export function luminance({ r, g, b }: RGB): number {
  const lin = (c: number) => {
    const s = c / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

// How far each raised surface steps from the base toward the foreground.
// Dark schemes step further: the same 3% reads as less on a dark ground.
const SURFACE_STEPS = {
  dark: [0, 0.03, 0.06, 0.09],
  light: [0, 0.02, 0.045, 0.075],
} as const;
const TEXT_ALPHAS = [0.92, 0.65, 0.45, 0.3] as const;
const BORDER_ALPHAS = {
  dark: { subtle: 0.06, default: 0.08, strong: 0.1 },
  light: { subtle: 0.06, default: 0.09, strong: 0.13 },
} as const;

/**
 * Derive the surface / text / border ramps for one scheme. Returns null when
 * either color is not a valid hex color, so callers fall back to defaults.
 */
export function deriveChrome(
  chrome: Partial<ChromeColors> | undefined,
  isDark: boolean,
): DerivedChrome | null {
  const bg = parseHexColor(chrome?.background);
  const fg = parseHexColor(chrome?.foreground);
  if (!bg || !fg) return null;
  const steps = isDark ? SURFACE_STEPS.dark : SURFACE_STEPS.light;
  const borders = isDark ? BORDER_ALPHAS.dark : BORDER_ALPHAS.light;
  return {
    surface: {
      0: toHex(mix(bg, fg, steps[0])),
      1: toHex(mix(bg, fg, steps[1])),
      2: toHex(mix(bg, fg, steps[2])),
      3: toHex(mix(bg, fg, steps[3])),
    },
    text: {
      1: rgba(fg, TEXT_ALPHAS[0]),
      2: rgba(fg, TEXT_ALPHAS[1]),
      3: rgba(fg, TEXT_ALPHAS[2]),
      4: rgba(fg, TEXT_ALPHAS[3]),
    },
    border: {
      subtle: rgba(fg, borders.subtle),
      default: rgba(fg, borders.default),
      strong: rgba(fg, borders.strong),
    },
  };
}

/** The design-token defaults in DerivedChrome shape. */
export function defaultChrome(isDark: boolean): DerivedChrome {
  return {
    surface: isDark ? surfaces.dark : surfaces.light,
    text: isDark ? textAlphas.dark : textAlphas.light,
    border: isDark ? borderAlphas.dark : borderAlphas.light,
  };
}

/** The default pairs, as an operator would enter them. */
export const DEFAULT_CHROME: { dark: ChromeColors; light: ChromeColors } = {
  dark: { background: surfaces.dark[0], foreground: "#ffffff" },
  light: { background: surfaces.light[0], foreground: "#09090b" },
};
