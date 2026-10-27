import { zero } from "@streamplace/components";
import QRCode from "qrcode";
import { useMemo } from "react";
import Svg, { Path, Rect } from "react-native-svg";

const QUIET_ZONE = 4; // modules of margin, as the QR spec asks

// Black on white whatever the color scheme: phone cameras read that best.
export function LoginQr({ value, size }: { value: string; size: number }) {
  const { path, extent } = useMemo(() => {
    const { modules } = QRCode.create(value, { errorCorrectionLevel: "M" });
    let d = "";
    for (let y = 0; y < modules.size; y++) {
      for (let x = 0; x < modules.size; x++) {
        if (modules.get(y, x))
          d += `M${x + QUIET_ZONE} ${y + QUIET_ZONE}h1v1h-1z`;
      }
    }
    return { path: d, extent: modules.size + 2 * QUIET_ZONE };
  }, [value]);
  return (
    <Svg width={size} height={size} viewBox={`0 0 ${extent} ${extent}`}>
      <Rect width={extent} height={extent} fill={zero.tokens.colors.white} />
      <Path d={path} fill={zero.tokens.colors.black} />
    </Svg>
  );
}
