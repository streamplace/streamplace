import { AppBskyActorDefs } from "@atproto/api";
import { useMemo } from "react";
import { Image } from "react-native";
import Svg, { Circle, Path, SvgXml } from "react-native-svg";
import { useTheme } from "../../lib/theme/theme";
import { colors } from "../../lib/theme/tokens";
import { useBrandingAsset } from "../../streamplace-store/branding";
import { View } from "../ui";

// The upstream social app's verified check.
export function VerifiedCheck({
  size = 16,
  color,
}: {
  size?: number;
  color: string;
}) {
  return (
    <Svg width={size} height={size} viewBox="0 0 24 24" fill="none">
      <Circle cx="12" cy="12" r="11.5" fill={color} />
      <Path
        fill={colors.white}
        fillRule="evenodd"
        clipRule="evenodd"
        d="M17.659 8.175a1.361 1.361 0 0 1 0 1.925l-6.224 6.223a1.361 1.361 0 0 1-1.925 0L6.4 13.212a1.361 1.361 0 0 1 1.925-1.925l2.149 2.148 5.26-5.26a1.361 1.361 0 0 1 1.925 0Z"
      />
    </Svg>
  );
}

function decodeDataUrlText(dataUrl: string): string | null {
  const comma = dataUrl.indexOf(",");
  if (comma < 0) return null;
  try {
    if (/;base64/i.test(dataUrl.slice(0, comma))) {
      const bin = atob(dataUrl.slice(comma + 1));
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      return new TextDecoder().decode(bytes);
    }
    return decodeURIComponent(dataUrl.slice(comma + 1));
  } catch {
    return null;
  }
}

/** The node's own verified badge: the uploaded verifiedIcon, else the check
 *  in the brand's primary color. */
function NodeVerifiedIcon({ size }: { size: number }) {
  const { theme } = useTheme();
  const asset = useBrandingAsset("verifiedIcon");
  const data = asset?.data;
  const svg = useMemo(() => {
    if (!data || !data.startsWith("data:")) return null;
    const mime = asset?.mimeType ?? "";
    if (!mime.includes("svg") && !data.startsWith("data:image/svg")) {
      return null;
    }
    const xml = decodeDataUrlText(data);
    return xml && xml.includes("<svg") ? xml : null;
  }, [data, asset?.mimeType]);
  if (svg) {
    return (
      <SvgXml
        xml={svg.replaceAll("currentColor", theme.colors.primary)}
        width={size}
        height={size}
      />
    );
  }
  if (data) {
    return (
      <Image
        source={{ uri: data }}
        style={{ width: size, height: size }}
        resizeMode="contain"
      />
    );
  }
  return <VerifiedCheck size={size} color={theme.colors.primary} />;
}

/**
 * The verified mark beside a chat name: the node's verdict, from the
 * verifiers and labelers the streamer's chat access rules allow, reported on
 * the message author itself. Nothing else counts: a network's own check is
 * not one of the streamer's rules.
 */
export function VerifiedBadge({
  author,
  size = 16,
  style,
}: {
  author: { verification?: { verifiedStatus?: string } | null };
  profile?: AppBskyActorDefs.ProfileViewDetailed | null;
  size?: number;
  style?: any;
}) {
  if (author.verification?.verifiedStatus !== "valid") return null;
  return (
    <View
      style={[{ marginLeft: 4, alignItems: "center", flexShrink: 0 }, style]}
      accessibilityLabel="Verified"
    >
      <NodeVerifiedIcon size={size} />
    </View>
  );
}
