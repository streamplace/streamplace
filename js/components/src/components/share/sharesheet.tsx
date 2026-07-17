import { Code, Copy, Link2, Share2 } from "lucide-react-native";
import { useCallback, useState } from "react";
import { Clipboard, Linking, Platform, View } from "react-native";
import { textAlphas } from "../../lib/theme/tokens";
import { useLivestreamStore } from "../../livestream-store";
import { useUrl } from "../../streamplace-store";
import { formatHandle } from "../../utils/format-handle";
import { BlueskyIcon } from "../icons/bluesky-icon";
import {
  DropdownMenu,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  ResponsiveDropdownMenuContent,
  Text,
} from "../ui";

// ShareTarget lets a caller (e.g. a VOD) override what gets shared. Without
// it, ShareSheet falls back to the current livestream's URLs.
export interface ShareTarget {
  /** Canonical page URL to share / copy. */
  url: string;
  /** Embed page URL used for the iframe src + "Copy Embed URL". */
  embedUrl: string;
  /** Message used for Bluesky / native share (the URL is appended). */
  message?: string;
}

export interface ShareSheetProps {
  onShare?: (action: string, success: boolean) => void;
  target?: ShareTarget;
}

export function ShareSheet({ onShare, target }: ShareSheetProps = {}) {
  const profile = useLivestreamStore((x) => x.profile);
  const [isCopying, setIsCopying] = useState(false);
  const url = useUrl();

  // Get the current stream URL
  const getStreamUrl = useCallback(() => {
    if (target) return target.url;
    return url + (profile ? `/${formatHandle(profile)}` : "");
  }, [target, url, profile]);

  // Get the embed URL
  const getEmbedUrl = useCallback(() => {
    if (target) return target.embedUrl;
    return url + (profile ? `/embed/${formatHandle(profile)}` : "");
  }, [target, url, profile]);

  // Get embed code
  const getEmbedCode = useCallback(() => {
    const embedUrl = getEmbedUrl();
    return `<iframe src="${embedUrl}" width="640" height="360" frameborder="0" allowfullscreen></iframe>`;
  }, [getEmbedUrl]);

  // Copy to clipboard handler
  const copyToClipboard = useCallback(
    async (text: string, label: string) => {
      setIsCopying(true);
      try {
        if (Platform.OS === "web") {
          await navigator.clipboard.writeText(text);
        } else {
          Clipboard.setString(text);
        }
        onShare?.(`copy_${label.toLowerCase().replace(/\s+/g, "_")}`, true);
      } catch (error) {
        onShare?.(`copy_${label.toLowerCase().replace(/\s+/g, "_")}`, false);
      } finally {
        setIsCopying(false);
      }
    },
    [onShare],
  );

  // Share to Bluesky
  const shareToBluesky = useCallback(() => {
    const streamUrl = getStreamUrl();
    const text = target?.message
      ? `${target.message} ${streamUrl}`
      : profile && profile.handle
        ? `Check out @${profile.handle} live on Streamplace! ${streamUrl}`
        : `Check out this stream on Streamplace! ${streamUrl}`;
    const blueskyUrl = `https://bsky.app/intent/compose?text=${encodeURIComponent(text)}`;
    Linking.openURL(blueskyUrl);
    onShare?.("share_bluesky", true);
  }, [profile, getStreamUrl, onShare, target]);

  // Native share (mobile)
  const nativeShare = useCallback(async () => {
    const streamUrl = getStreamUrl();
    const text = target?.message
      ? target.message
      : profile && profile.handle
        ? `Check out @${profile.handle} live on Streamplace!`
        : `Check out this stream on Streamplace!`;

    if (Platform.OS === "web" && navigator.share) {
      try {
        await navigator.share({
          title: "Streamplace",
          text: text,
          url: streamUrl,
        });
        onShare?.("share_native", true);
      } catch (error) {
        // User cancelled or error occurred
        onShare?.("share_native", false);
      }
    }
  }, [profile, getStreamUrl, onShare, target]);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger>
        <Share2 color={textAlphas.dark[1]} />
      </DropdownMenuTrigger>
      <ResponsiveDropdownMenuContent>
        <DropdownMenuGroup title="Share">
          <DropdownMenuItem onPress={shareToBluesky} closeOnPress={true}>
            <View
              style={{ flexDirection: "row", alignItems: "center", gap: 12 }}
            >
              <BlueskyIcon size={20} color={textAlphas.dark[2]} />
              <Text>Share to Bluesky</Text>
            </View>
          </DropdownMenuItem>
          {/* navigator isn't on non-web */}
          {Platform.OS !== "web" || (navigator && (navigator as any).share) ? (
            <DropdownMenuItem onPress={nativeShare}>
              <View
                style={{ flexDirection: "row", alignItems: "center", gap: 12 }}
              >
                <Share2 size={20} color={textAlphas.dark[2]} />
                <Text>More Options...</Text>
              </View>
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuGroup>
        <DropdownMenuGroup title="Copy">
          <DropdownMenuItem
            onPress={() => copyToClipboard(getStreamUrl(), "Stream link")}
            disabled={isCopying}
            closeOnPress={true}
          >
            <View
              style={{ flexDirection: "row", alignItems: "center", gap: 12 }}
            >
              <Link2 size={20} color={textAlphas.dark[2]} />
              <Text>Copy Link</Text>
            </View>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onPress={() => copyToClipboard(getEmbedCode(), "Embed code")}
            disabled={isCopying}
            closeOnPress={true}
          >
            <View
              style={{ flexDirection: "row", alignItems: "center", gap: 12 }}
            >
              <Code size={20} color={textAlphas.dark[2]} />
              <Text>Copy Embed Code</Text>
            </View>
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            closeOnPress={true}
            onPress={() => copyToClipboard(getEmbedUrl(), "Embed URL")}
            disabled={isCopying}
          >
            <View
              style={{ flexDirection: "row", alignItems: "center", gap: 12 }}
            >
              <Copy size={20} color={textAlphas.dark[2]} />
              <Text>Copy Embed URL</Text>
            </View>
          </DropdownMenuItem>
        </DropdownMenuGroup>
      </ResponsiveDropdownMenuContent>
    </DropdownMenu>
  );
}
