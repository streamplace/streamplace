import { Image } from "expo-image";
import { useWindowDimensions, View } from "react-native";
import { zero } from "../..";
import { useAuthor } from "../../hooks/useAuthor";
import { useAvatar } from "../../hooks/useAvatar";
import { useTitle } from "../../hooks/useTitle";
import { useUrl } from "../../streamplace-store";
import { gap, layout, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { Viewers } from "../mobile-player/ui/viewers";
import { ShareSheet } from "../share/sharesheet";
import { Text } from "../ui/text";
import { LikeButton } from "./like-button";

// Below this width we drop the avatar and the view count so the title, like
// and share controls don't crowd on small phones. Above it (tablets/desktop)
// the full row shows, matching what the desktop metadata bar used to carry.
const NARROW_BREAKPOINT = 480;

// rkeyFromAturi pulls the record key off an at:// URI
// (at://<did>/place.stream.video/<rkey>).
function rkeyFromAturi(aturi: string): string {
  return aturi.split("/").pop() ?? "";
}

export function VodMobileMetadata() {
  const video = useVideoStore((x) => x.video);
  const aturi = useVideoStore((x) => x.aturi);
  const title = useTitle();
  const author = useAuthor();
  const avatar = useAvatar();
  const baseUrl = useUrl();
  const { width } = useWindowDimensions();
  const { theme } = useTheme();

  if (!video || !aturi) return null;

  const wide = width >= NARROW_BREAKPOINT;
  const handle = author?.handle ?? video.author.handle;
  const did = author?.did ?? video.author.did;
  const userSeg = handle || did;
  const rkey = rkeyFromAturi(aturi);

  // The video's canonical page + embed URLs, so Share actually links to this
  // VOD rather than the bare site root.
  const shareTarget = {
    url: `${baseUrl}/${userSeg}/video/${rkey}`,
    embedUrl: `${baseUrl}/embed/${userSeg}/video/${rkey}`,
    message: `Check out "${title || "this video"}" on Streamplace!`,
  };

  return (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        zero.layout.flex.justify.between,
        gap.all[3],
      ]}
    >
      {/* Left: avatar + title/author */}
      <View
        style={[
          layout.flex.row,
          layout.flex.alignCenter,
          gap.all[3],
          zero.flex[1],
        ]}
      >
        {wide && (
          <View
            style={{
              width: 40,
              height: 40,
              borderRadius: 999,
              overflow: "hidden",
              backgroundColor: theme.colors.muted,
              flexShrink: 0,
            }}
          >
            {avatar ? (
              <Image
                source={{ uri: avatar }}
                style={{ width: "100%", height: "100%" }}
                contentFit="cover"
              />
            ) : null}
          </View>
        )}
        <View style={zero.flex[1]}>
          <Text weight="semibold" numberOfLines={1}>
            {title || "Untitled"}
          </Text>
          <Text
            size="sm"
            style={{ color: theme.colors.textMuted }}
            numberOfLines={1}
          >
            {handle ? `@${handle}` : did}
          </Text>
        </View>
      </View>

      {/* Right: like + views + share */}
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}>
        {/* Use the server-canonical (DID-based) video.uri, not the store's
            aturi — when the page is reached via a handle URL the aturi's
            authority is the handle, and a like keyed on a handle subject
            won't match the DID-keyed video record. */}
        <LikeButton subjectUri={video.uri} />
        {wide && <Viewers />}
        <ShareSheet target={shareTarget} />
      </View>
    </View>
  );
}
