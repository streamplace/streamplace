// Viewer caption display settings, after the FCC's caption display
// requirements (47 CFR 79.103(c)): text size, font, text color and
// opacity, edge style, background color and opacity, and window color.
//
// The colors here are the viewer's choices, not design tokens: they are
// the eight caption colors of CEA-708 and the iOS/Android caption
// settings, and they must render exactly as picked. UI chrome around
// captions still comes from the theme.

export type CaptionColor =
  | "white"
  | "black"
  | "red"
  | "green"
  | "blue"
  | "yellow"
  | "magenta"
  | "cyan";

export const CAPTION_COLORS: Record<CaptionColor, [number, number, number]> = {
  white: [255, 255, 255],
  black: [0, 0, 0],
  red: [255, 0, 0],
  green: [0, 255, 0],
  blue: [0, 0, 255],
  yellow: [255, 255, 0],
  magenta: [255, 0, 255],
  cyan: [0, 255, 255],
};

/** 79.103(c)(4): the seven caption font styles of CEA-708. */
export type CaptionFont =
  | "proportionalSans"
  | "monospacedSans"
  | "proportionalSerif"
  | "monospacedSerif"
  | "casual"
  | "cursive"
  | "smallCaps";

export const CAPTION_FONTS: CaptionFont[] = [
  "proportionalSans",
  "monospacedSans",
  "proportionalSerif",
  "monospacedSerif",
  "casual",
  "cursive",
  "smallCaps",
];

/** CSS font stacks for each caption font, for web renderers. */
export const CAPTION_WEB_FONTS: Record<CaptionFont, string> = {
  proportionalSans: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
  monospacedSans:
    "ui-monospace, 'SF Mono', Menlo, Consolas, 'Liberation Mono', monospace",
  proportionalSerif: "Georgia, 'Times New Roman', serif",
  monospacedSerif: "'Courier New', Courier, monospace",
  casual: "'Comic Sans MS', 'Comic Neue', casual, cursive",
  cursive: "'Brush Script MT', 'Segoe Script', cursive",
  smallCaps: "system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
};

/** 79.103(c)(7): character edge attributes. */
export type CaptionEdge =
  | "none"
  | "raised"
  | "depressed"
  | "uniform"
  | "dropShadow";

export const CAPTION_EDGES: CaptionEdge[] = [
  "none",
  "raised",
  "depressed",
  "uniform",
  "dropShadow",
];

/** Text size choices, percent of the default caption size. */
export const CAPTION_SIZES = [50, 75, 100, 150, 200, 300] as const;
export type CaptionSize = (typeof CAPTION_SIZES)[number];

/** Opacity choices, percent. */
export const CAPTION_OPACITIES = [0, 25, 50, 75, 100] as const;
export type CaptionOpacity = (typeof CAPTION_OPACITIES)[number];

export interface CaptionDisplayPrefs {
  size: CaptionSize;
  font: CaptionFont;
  textColor: CaptionColor;
  textOpacity: CaptionOpacity;
  edge: CaptionEdge;
  backgroundColor: CaptionColor;
  backgroundOpacity: CaptionOpacity;
  windowColor: CaptionColor;
  windowOpacity: CaptionOpacity;
}

/**
 * White text on a 75% black box, no window: legible over any video, and
 * the default of the platform caption settings viewers already know.
 */
export const DEFAULT_CAPTION_PREFS: CaptionDisplayPrefs = {
  size: 100,
  font: "proportionalSans",
  textColor: "white",
  textOpacity: 100,
  edge: "none",
  backgroundColor: "black",
  backgroundOpacity: 75,
  windowColor: "black",
  windowOpacity: 0,
};

function oneOf<T>(choices: readonly T[], value: unknown, fallback: T): T {
  return choices.includes(value as T) ? (value as T) : fallback;
}

const COLOR_NAMES = Object.keys(CAPTION_COLORS) as CaptionColor[];

/**
 * Reads stored prefs (a JSON string, possibly from an older or newer
 * build), keeping each valid field and defaulting the rest.
 */
export function parseCaptionPrefs(stored: string | null): CaptionDisplayPrefs {
  let raw: Record<string, unknown> = {};
  try {
    const parsed: unknown = stored ? JSON.parse(stored) : null;
    if (parsed && typeof parsed === "object") {
      raw = parsed as Record<string, unknown>;
    }
  } catch {
    // Corrupt storage: fall back to the defaults.
  }
  const d = DEFAULT_CAPTION_PREFS;
  return {
    size: oneOf(CAPTION_SIZES, raw.size, d.size),
    font: oneOf(CAPTION_FONTS, raw.font, d.font),
    textColor: oneOf(COLOR_NAMES, raw.textColor, d.textColor),
    textOpacity: oneOf(CAPTION_OPACITIES, raw.textOpacity, d.textOpacity),
    edge: oneOf(CAPTION_EDGES, raw.edge, d.edge),
    backgroundColor: oneOf(COLOR_NAMES, raw.backgroundColor, d.backgroundColor),
    backgroundOpacity: oneOf(
      CAPTION_OPACITIES,
      raw.backgroundOpacity,
      d.backgroundOpacity,
    ),
    windowColor: oneOf(COLOR_NAMES, raw.windowColor, d.windowColor),
    windowOpacity: oneOf(CAPTION_OPACITIES, raw.windowOpacity, d.windowOpacity),
  };
}

/** The CSS/React Native color string for a caption color at an opacity. */
export function captionColor(
  color: CaptionColor,
  opacityPercent: number,
): string {
  const [r, g, b] = CAPTION_COLORS[color];
  return `rgba(${r}, ${g}, ${b}, ${opacityPercent / 100})`;
}

/**
 * Default caption text height as a fraction of the player's height,
 * close to the default caption size of the iOS and Android players.
 */
const CAPTION_HEIGHT_FRACTION = 1 / 22;

/**
 * Caption font size in px for a player of the given height, scaled by
 * the viewer's size choice and floored at `minPx` so captions stay
 * readable in small embeds.
 */
export function captionFontSize(
  prefs: CaptionDisplayPrefs,
  playerHeight: number,
  minPx: number,
): number {
  const base = Math.max(minPx, playerHeight * CAPTION_HEIGHT_FRACTION);
  return Math.round((base * prefs.size) / 100);
}

export interface CaptionShadow {
  offsetX: number;
  offsetY: number;
  blur: number;
  color: string;
}

/**
 * Shadows that draw an edge style, in px relative to the font size. The
 * edge color contrasts with the text: dark edges for light text and the
 * reverse. The most visible shadow comes first, for renderers that draw
 * only one (React Native).
 */
export function captionEdgeShadows(
  prefs: CaptionDisplayPrefs,
  fontSize: number,
): CaptionShadow[] {
  const [r, g, b] = CAPTION_COLORS[prefs.textColor];
  const lightText = r * 0.299 + g * 0.587 + b * 0.114 > 128;
  const alpha = prefs.textOpacity;
  const dark = captionColor(lightText ? "black" : "white", alpha);
  const light = captionColor(lightText ? "white" : "black", alpha);
  const w = Math.max(1, Math.round(fontSize / 16));
  switch (prefs.edge) {
    case "raised":
      return [
        { offsetX: w, offsetY: w, blur: 0, color: dark },
        { offsetX: -w, offsetY: -w, blur: 0, color: light },
      ];
    case "depressed":
      return [
        { offsetX: -w, offsetY: -w, blur: 0, color: dark },
        { offsetX: w, offsetY: w, blur: 0, color: light },
      ];
    case "uniform":
      return [
        { offsetX: 0, offsetY: 0, blur: w * 2, color: dark },
        { offsetX: w, offsetY: 0, blur: 0, color: dark },
        { offsetX: -w, offsetY: 0, blur: 0, color: dark },
        { offsetX: 0, offsetY: w, blur: 0, color: dark },
        { offsetX: 0, offsetY: -w, blur: 0, color: dark },
      ];
    case "dropShadow":
      return [{ offsetX: w, offsetY: w, blur: w * 2, color: dark }];
    default:
      return [];
  }
}

/** CSS text-shadow for an edge style. */
export function captionTextShadowCss(shadows: CaptionShadow[]): string {
  return shadows
    .map((s) => `${s.offsetX}px ${s.offsetY}px ${s.blur}px ${s.color}`)
    .join(", ");
}
