import { Image } from "react-native";
import { SvgXml } from "react-native-svg";
import IconBsky from "../../icons/icon-bsky";
import { useBrandingAsset } from "../../streamplace-store/branding";

// Decode the payload of a base64 data: URL as UTF-8 text.
function decodeDataUrlText(dataUrl: string): string | null {
  const comma = dataUrl.indexOf(",");
  if (comma < 0) return null;
  const meta = dataUrl.slice(0, comma);
  const payload = dataUrl.slice(comma + 1);
  try {
    if (/;base64/i.test(meta)) {
      const bin = atob(payload);
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      return new TextDecoder().decode(bytes);
    }
    return decodeURIComponent(payload);
  } catch {
    return null;
  }
}

/**
 * The icon on links to a user's profile on the network the node belongs to
 * (branding key networkIcon): an uploaded SVG renders as drawn (or tinted
 * where it uses currentColor), a raster upload as an image, and with
 * nothing uploaded the Bluesky butterfly.
 */
export function NetworkIcon({
  size = 18,
  color,
}: {
  size?: number;
  color?: string;
}) {
  const asset = useBrandingAsset("networkIcon");
  const data = asset?.data;
  if (data && data.startsWith("data:")) {
    const mime = asset?.mimeType ?? "";
    if (mime.includes("svg") || data.startsWith("data:image/svg")) {
      const xml = decodeDataUrlText(data);
      if (xml && xml.includes("<svg")) {
        return (
          <SvgXml
            xml={color ? xml.replaceAll("currentColor", color) : xml}
            width={size}
            height={size}
          />
        );
      }
    }
    return (
      <Image
        source={{ uri: data }}
        style={{ width: size, height: size }}
        resizeMode="contain"
      />
    );
  }
  return <IconBsky size={size} />;
}
