import { Text, useTheme } from "@streamplace/components";
import { fontFamilies } from "@streamplace/components/src/lib/theme/tokens";
import { Fragment } from "react";
import { View, type ViewProps } from "react-native";
import { SvgXml } from "react-native-svg";
import { BRAND } from "../../assets/generated/brand";

/**
 * Brand logo components. All artwork comes from the generated brand module
 * (js/app/assets/generated/brand.ts), which js/brand/generate.mjs derives
 * from the active brand directory — see brand/README.md. Nothing here is
 * specific to any one identity; forks swap the brand dir, not this file.
 */

export type { Brand, BrandStory } from "../../assets/generated/brand";
export { BRAND };

// Monochrome marks are authored with fill="currentColor"; tint by
// substitution. A multi-color mark has no currentColor, so this no-ops and
// the art renders as drawn.
const tinted = (svg: string, color: string) =>
  svg.replaceAll("currentColor", color);

/** Standalone SVG source for the "copy as SVG" brand menu. */
export function markSvgString(color = BRAND.colors.ink) {
  return tinted(BRAND.markSvg, color);
}

export function wordmarkSvgString(color = BRAND.colors.ink) {
  return tinted(BRAND.wordmarkSvg, color);
}

export function LogoMark({
  size = 24,
  color,
}: {
  size?: number;
  color?: string;
}) {
  const { theme } = useTheme();
  // Monochrome brands default the mark to the ink/paper text color so it
  // matches the wordmark exactly; pass `color` explicitly for the rare
  // colored variant.
  return (
    <SvgXml
      xml={tinted(BRAND.markSvg, color ?? theme.colors.text1)}
      width={size}
      height={size}
    />
  );
}

/**
 * App icon / avatar tile: the mark inside a continuous-curvature square,
 * with enough inset to survive icon masks. Colors come from the brand's
 * tile palette, not the running theme, so it matches the installed icon.
 */
export function LogoTile({ size = 32 }: { size?: number }) {
  return <SvgXml xml={BRAND.tileSvg} width={size} height={size} />;
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
  /** Optional accent color for any "." — omit to keep the wordmark mono. */
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
  const parts = BRAND.wordmark.split(".");
  return (
    <Text style={base} selectable={false}>
      {parts.map((part, i) => (
        <Fragment key={i}>
          {i > 0 && (
            <Text style={{ ...base, color: dotColor ?? base.color }}>.</Text>
          )}
          {part}
        </Fragment>
      ))}
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
