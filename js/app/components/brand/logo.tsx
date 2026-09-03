import { Text, useBrandingAsset, useTheme } from "@streamplace/components";
import { fontFamilies } from "@streamplace/components/src/lib/theme/tokens";
import { Fragment, useMemo } from "react";
import { Image, View, type ViewProps } from "react-native";
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

// Decode the payload of a base64 data: URL as UTF-8 text.
function decodeDataUrlText(dataUrl: string): string | null {
  const comma = dataUrl.indexOf(",");
  if (comma < 0) return null;
  const header = dataUrl.slice(0, comma);
  const payload = dataUrl.slice(comma + 1);
  try {
    if (!/;base64$/i.test(header)) return decodeURIComponent(payload);
    const bin = atob(payload);
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    return new TextDecoder().decode(bytes);
  } catch {
    return null;
  }
}

/**
 * The node's uploaded mainLogo branding asset, if any: an inline SVG when
 * the upload was SVG (so it scales and can be copied as SVG), else the data
 * URL for an <Image>. Empty when the node has no custom logo.
 */
export function useCustomMark(): { svg?: string; uri?: string } {
  const asset = useBrandingAsset("mainLogo");
  const data = asset?.data;
  const mime = asset?.mimeType;
  return useMemo(() => {
    if (!data || !data.startsWith("data:")) return {};
    if ((mime ?? "").includes("svg") || data.startsWith("data:image/svg")) {
      const svg = decodeDataUrlText(data);
      if (svg && svg.includes("<svg")) return { svg, uri: data };
    }
    return { uri: data };
  }, [data, mime]);
}

export function LogoMark({
  size = 24,
  color,
}: {
  size?: number;
  color?: string;
}) {
  const { theme } = useTheme();
  const custom = useCustomMark();
  // A node's uploaded logo renders as drawn: no tinting, so multi-color
  // artwork survives, at the same box the default mark would occupy.
  if (custom.svg) {
    return <SvgXml xml={custom.svg} width={size} height={size} />;
  }
  if (custom.uri) {
    return (
      <Image
        source={{ uri: custom.uri }}
        style={{ width: size, height: size }}
        resizeMode="contain"
        accessibilityIgnoresInvertColors
      />
    );
  }
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
  text = BRAND.wordmark,
  color,
  dotColor,
  weight = "semibold",
  letterSpacing,
}: {
  size?: number;
  /** Text to set in wordmark styling; defaults to the brand wordmark. */
  text?: string;
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
  const parts = text.split(".");
  // Long site titles truncate with an ellipsis rather than wrapping out of
  // the nav; every flex ancestor needs minWidth: 0 for that to work on web.
  return (
    <Text
      style={{ ...base, flexShrink: 1, minWidth: 0 }}
      numberOfLines={1}
      selectable={false}
    >
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
  text,
  color,
  markColor,
  dotColor,
  weight = "semibold",
  letterSpacing,
  ...props
}: ViewProps & {
  size?: number;
  text?: string;
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
          flexShrink: 1,
          minWidth: 0,
        },
        props.style,
      ]}
    >
      <LogoMark size={markSize} color={markColor} />
      <Wordmark
        size={size}
        text={text}
        color={color}
        dotColor={dotColor}
        weight={weight}
        letterSpacing={letterSpacing}
      />
    </View>
  );
}

/**
 * What this node calls itself: the operator's runtime siteTitle branding
 * when set, else the brand's defaultSiteTitle (which the first-party brand
 * points at its own wordmark, so branded nodes show the styled wordmark
 * with no runtime config). Drives the nav lockup and the browser tab.
 */
export function useNodeTitle() {
  return useBrandingAsset("siteTitle")?.data || BRAND.defaultSiteTitle;
}

/** The nav-header lockup: the mark plus the node's title. */
export function SiteTitleLockup(
  props: Omit<Parameters<typeof LogoLockup>[0], "text">,
) {
  return <LogoLockup {...props} text={useNodeTitle()} />;
}
