import { Text, useTheme } from "@streamplace/components";
import {
  borderAlphas,
  colors,
  fontFamilies,
  surfaces,
} from "@streamplace/components/src/lib/theme/tokens";
import { View, type ViewProps } from "react-native";
import Svg, { G, Path } from "react-native-svg";

/**
 * Streamplace mark, drawn on a 24-unit grid.
 *
 * An S built from two plays. A single rounded square holds two triangular
 * play voids in 180-degree rotational symmetry: one opens through the right
 * edge, its mirror opens through the left. The ink that remains reads as an
 * S (Streamplace); the two voids read as play (video). One points out, its
 * mirror points back — watch and broadcast, the two places of a stream.
 *
 * The whole mark is one solid color; the plays are negative space. It is
 * pure figure-ground in the Logo Modernism spirit, and holds down to
 * favicon size because the dominant form fills the field.
 */
const TILE = { x: 3, y: 3, size: 18, radius: 3.5 };

// Full play triangles (used as construction guides). Upper opens right;
// lower is its exact 180-degree rotation about the center (12, 12).
const UPPER_PLAY: [number, number][] = [
  [10.2, 5.4],
  [10.2, 11.8],
  [23, 8.6],
];
const LOWER_PLAY: [number, number][] = UPPER_PLAY.map(
  ([x, y]) => [24 - x, 24 - y] as [number, number],
);

// The rounded-square field, as a path so it can carry evenodd holes. Its
// 3.5u corners are tightened toward the sharp play voids so the whole mark
// shares one crisp corner language rather than reading half-soft, half-sharp.
const TILE_PATH =
  "M 6.5 3 H 17.5 A 3.5 3.5 0 0 1 21 6.5 V 17.5 A 3.5 3.5 0 0 1 17.5 21 H 6.5 A 3.5 3.5 0 0 1 3 17.5 V 6.5 A 3.5 3.5 0 0 1 6.5 3 Z";

// The play voids, pre-clipped to the field's edge so each breaches cleanly
// (a ~1u opening) with no overhang under an evenodd fill.
const UPPER_HOLE = "M 10.2 5.4 L 10.2 11.8 L 21 9.1 L 21 8.1 Z";
const LOWER_HOLE = "M 13.8 18.6 L 13.8 12.2 L 3 14.9 L 3 15.9 Z";

export const MARK = {
  grid: 24,
  tile: TILE,
  center: [12, 12] as [number, number],
  tilePath: TILE_PATH,
  upperPlay: UPPER_PLAY,
  lowerPlay: LOWER_PLAY,
  upperHole: UPPER_HOLE,
  lowerHole: LOWER_HOLE,
};

export const MARK_WITH_HOLE = `${TILE_PATH} ${UPPER_HOLE} ${LOWER_HOLE}`;

// Standalone SVG source for the "copy as SVG" brand menu. Mono ink by default
// (reads on light surfaces / code editors); mirrors js/app/public/brand/*.svg.
// token-ok: SVG export ink, mirrors public/brand/*.svg
export function markSvgString(color = "#0A0A0B") {
  // token-ok
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="${color}" fill-rule="evenodd" d="${MARK_WITH_HOLE}"/></svg>`;
}

export function wordmarkSvgString(color = "#0A0A0B") {
  // token-ok
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 420 96"><text x="0" y="68" fill="${color}" font-family="Geist, Inter, Arial, sans-serif" font-size="72" font-weight="600" letter-spacing="-1.44">stream.place</text></svg>`;
}

function MarkPath({ color }: { color: string }) {
  return <Path d={MARK_WITH_HOLE} fill={color} fillRule="evenodd" />;
}

export function LogoMark({
  size = 24,
  color,
}: {
  size?: number;
  color?: string;
}) {
  const { theme } = useTheme();
  // Monochrome brand: the mark defaults to the ink/paper text color and
  // matches the wordmark exactly. Indigo is a secondary accent, never the
  // mark's own color — pass `color` explicitly for the rare colored variant.
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24">
      <MarkPath color={color ?? theme.colors.text1} />
    </Svg>
  );
}

/**
 * App icon / avatar tile: the mark reversed to paper inside a
 * continuous-curvature square, with enough inset to survive icon masks.
 */
export function LogoTile({
  size = 32,
  background,
  foreground,
}: {
  size?: number;
  background?: string;
  foreground?: string;
}) {
  // Mono brand: a near-black tile with the paper-white S. A faint hairline
  // keeps the dark tile legible on near-black surfaces. Indigo lives in the
  // running UI (buttons, links, focus) — never in the brand mark itself.
  const squircle =
    "M 0 16 C 0 4 4 0 16 0 C 28 0 32 4 32 16 C 32 28 28 32 16 32 C 4 32 0 28 0 16 Z";
  const bg = background ?? surfaces.dark[1];
  const fg = foreground ?? colors.white;
  return (
    <Svg width={size} height={size} viewBox="0 0 32 32">
      <Path d={squircle} fill={bg} />
      <Path
        d={squircle}
        fill="none"
        stroke={borderAlphas.dark.strong}
        strokeWidth={1}
      />
      <G transform="translate(4 4)">
        <MarkPath color={fg} />
      </G>
    </Svg>
  );
}

export function Wordmark({
  size = 20,
  color,
  dotColor,
  weight = "semibold",
  letterSpacing,
}: {
  size?: number;
  color?: string;
  /** Optional accent color for the "." — omit to keep the wordmark mono. */
  dotColor?: string;
  /** Wordmark weight — "semibold" for the public signature, "medium" for chrome like the nav. */
  weight?: "medium" | "semibold";
  /** Explicit tracking in px; falls back to a weight-based default. */
  letterSpacing?: number;
}) {
  const { theme } = useTheme();
  const isMedium = weight === "medium";
  const base = {
    fontSize: size,
    lineHeight: Math.round(size * 1.16),
    letterSpacing: letterSpacing ?? (isMedium ? -0.025 : -0.02) * size,
    fontFamily: isMedium ? fontFamilies.medium : fontFamilies.semiBold,
    fontWeight: (isMedium ? "500" : "600") as "500" | "600",
    color: color ?? theme.colors.text1,
  };
  return (
    <Text style={base} selectable={false}>
      stream
      <Text style={{ ...base, color: dotColor ?? base.color }}>.</Text>
      place
    </Text>
  );
}

export function LogoLockup({
  size = 20,
  color,
  markColor,
  dotColor,
  weight = "semibold",
  letterSpacing,
  ...props
}: ViewProps & {
  size?: number;
  color?: string;
  markColor?: string;
  dotColor?: string;
  weight?: "medium" | "semibold";
  letterSpacing?: number;
}) {
  const markSize = Math.round(size * 1.3);
  const gap = Math.round(markSize / 4);
  return (
    <View
      {...props}
      style={[
        {
          flexDirection: "row",
          alignItems: "center",
          gap,
        },
        props.style,
      ]}
    >
      <LogoMark size={markSize} color={markColor} />
      <Wordmark
        size={size}
        color={color}
        dotColor={dotColor}
        weight={weight}
        letterSpacing={letterSpacing}
      />
    </View>
  );
}
