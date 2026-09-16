import { useBrandingAsset } from "@streamplace/components";
import { decodeDataUrlText } from "components/brand/logo";
import QRCode from "qrcode";
import { useMemo } from "react";
import { Image, View } from "react-native";
import Svg, { Circle, Path, Rect, SvgXml } from "react-native-svg";

/**
 * A QR code drawn by the app itself, at whatever size the layout gives it,
 * rather than the small raster image the identity provider hands out.
 * Styled like the network's own sign-in page: round data modules, rounded
 * finder patterns, the network's icon in the middle (the modules under it
 * are left out, which the H error-correction level covers). Always dark on
 * white whatever the theme: that is what phone cameras read best.
 */
export function QrCode({
  value,
  size,
  accessibilityLabel,
}: {
  value: string;
  size: number;
  accessibilityLabel?: string;
}) {
  const icon = useBrandingAsset("networkIcon");
  const logo = useMemo(() => {
    const data = icon?.data;
    if (!data || !data.startsWith("data:")) return null;
    const mime = icon?.mimeType ?? "";
    if (mime.includes("svg") || data.startsWith("data:image/svg")) {
      const xml = decodeDataUrlText(data);
      if (xml && xml.includes("<svg")) return { xml };
    }
    return { uri: data };
  }, [icon?.data, icon?.mimeType]);

  const drawing = useMemo(() => {
    const code = QRCode.create(value, { errorCorrectionLevel: "H" });
    const n = code.modules.size;
    const quiet = 2; // modules of white around the code
    const cell = size / (n + 2 * quiet);
    const at = (i: number) => (i + quiet) * cell;
    // The three finder patterns are drawn as shapes, not modules.
    const finders = [
      [0, 0],
      [n - 7, 0],
      [0, n - 7],
    ];
    const inFinder = (r: number, c: number) =>
      finders.some(
        ([fr, fc]) => r >= fr && r < fr + 7 && c >= fc && c < fc + 7,
      );
    // A square hole in the middle for the icon: about a fifth of the side,
    // rounded up to whole modules, plus a module of breathing room.
    const logoModules = logo ? Math.ceil(n * 0.2) + 2 : 0;
    const holeStart = Math.floor((n - logoModules) / 2);
    const inHole = (r: number, c: number) =>
      logoModules > 0 &&
      r >= holeStart &&
      r < holeStart + logoModules &&
      c >= holeStart &&
      c < holeStart + logoModules;
    const dots: { cx: number; cy: number }[] = [];
    for (let r = 0; r < n; r++) {
      for (let c = 0; c < n; c++) {
        if (!code.modules.get(r, c) || inFinder(r, c) || inHole(r, c)) continue;
        dots.push({ cx: at(c) + cell / 2, cy: at(r) + cell / 2 });
      }
    }
    return { n, cell, at, finders, dots, logoModules, holeStart };
  }, [value, size, logo]);

  const { cell, at, finders, dots, logoModules, holeStart } = drawing;
  const ink = "#111111";
  const logoSize = logoModules > 0 ? (logoModules - 2) * cell : 0;
  const logoOffset = at(holeStart) + cell;
  // One path for every dot keeps the SVG small at 30-odd modules a side.
  const dotPath = dots
    .map(
      ({ cx, cy }) =>
        `M${cx - cell * 0.42},${cy}a${cell * 0.42},${cell * 0.42} 0 1,0 ${cell * 0.84},0a${cell * 0.42},${cell * 0.42} 0 1,0 -${cell * 0.84},0`,
    )
    .join("");

  return (
    <View
      style={{ width: size, height: size }}
      accessibilityRole="image"
      accessibilityLabel={accessibilityLabel}
    >
      <Svg width={size} height={size} viewBox={`0 0 ${size} ${size}`}>
        <Rect
          x={0}
          y={0}
          width={size}
          height={size}
          rx={cell * 2}
          fill="#ffffff"
        />
        <Path d={dotPath} fill={ink} />
        {finders.map(([r, c]) => (
          <Rect
            key={`${r}-${c}`}
            x={at(c) + cell / 2}
            y={at(r) + cell / 2}
            width={cell * 6}
            height={cell * 6}
            rx={cell * 2}
            fill="none"
            stroke={ink}
            strokeWidth={cell}
          />
        ))}
        {finders.map(([r, c]) => (
          <Circle
            key={`d${r}-${c}`}
            cx={at(c) + cell * 3.5}
            cy={at(r) + cell * 3.5}
            r={cell * 1.5}
            fill={ink}
          />
        ))}
      </Svg>
      {logo && logoSize > 0 && (
        <View
          pointerEvents="none"
          style={{
            position: "absolute",
            left: logoOffset,
            top: logoOffset,
            width: logoSize,
            height: logoSize,
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          {logo.xml ? (
            <SvgXml xml={logo.xml} width={logoSize} height={logoSize} />
          ) : (
            <Image
              source={{ uri: logo.uri }}
              style={{ width: logoSize, height: logoSize }}
              resizeMode="contain"
            />
          )}
        </View>
      )}
    </View>
  );
}
