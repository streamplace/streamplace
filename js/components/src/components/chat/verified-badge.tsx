import { AppBskyActorDefs } from "@atproto/api";
import { useMemo } from "react";
import { Image } from "react-native";
import Svg, { Circle, Path, SvgXml } from "react-native-svg";
import { useTheme } from "../../lib/theme/theme";
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
        fill="#fff"
        fillRule="evenodd"
        clipRule="evenodd"
        d="M17.659 8.175a1.361 1.361 0 0 1 0 1.925l-6.224 6.223a1.361 1.361 0 0 1-1.925 0L6.4 13.212a1.361 1.361 0 0 1 1.925-1.925l2.149 2.148 5.26-5.26a1.361 1.361 0 0 1 1.925 0Z"
      />
    </Svg>
  );
}

const BLUESKY_BLUE = "#1185fe"; // token-ok: Bluesky's verification color

/** The verifier DID the node files Bluesky's blue check under when the
 *  branding key verifyBluesky is on (pkg/atproto/appview_verification.go). */
export const BLUESKY_APPVIEW_ISSUER = "did:web:api.bsky.app";

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
 * Verified marks beside a chat name. Two sources, two looks:
 *  - the node's trusted verifiers (branding key verifierDids), which the
 *    node reports on the message author itself — the branded badge;
 *  - the public app view's verification (e.g. Bluesky's), read from the
 *    profile cache, or mirrored by the node under Bluesky's app view DID
 *    when verifyBluesky is on — the network's blue check.
 * A user verified both ways gets the node's badge only.
 */
export function VerifiedBadge({
  author,
  profile,
  size = 16,
  style,
}: {
  author: {
    verification?: {
      verifiedStatus?: string;
      verifications?: { issuer: string }[];
    } | null;
  };
  profile?: AppBskyActorDefs.ProfileViewDetailed | null;
  size?: number;
  style?: any;
}) {
  const nodeIssuers = author.verification?.verifications ?? [];
  const nodeValid = author.verification?.verifiedStatus === "valid";
  // Rows filed under Bluesky's app view are Bluesky's check, not ours.
  const nodeVerified =
    nodeValid &&
    (nodeIssuers.length === 0 ||
      nodeIssuers.some((v) => v.issuer !== BLUESKY_APPVIEW_ISSUER));
  const networkVerified =
    profile?.verification?.verifiedStatus === "valid" ||
    (nodeValid && !nodeVerified);
  if (!nodeVerified && !networkVerified) return null;
  return (
    <View
      style={[{ marginLeft: 4, alignItems: "center", flexShrink: 0 }, style]}
      accessibilityLabel={nodeVerified ? "Verified" : "Verified on Bluesky"}
    >
      {nodeVerified ? (
        <NodeVerifiedIcon size={size} />
      ) : (
        <VerifiedCheck size={size} color={BLUESKY_BLUE} />
      )}
    </View>
  );
}
