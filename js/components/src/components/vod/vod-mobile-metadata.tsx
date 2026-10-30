import { formatHandle, formatHandleWithAt } from "@streamplace/core";
import { Share2 } from "lucide-react-native";
import { useWindowDimensions, View } from "react-native";
import { useAuthor } from "../../hooks/useAuthor";
import { useAvatar } from "../../hooks/useAvatar";
import { useTitle } from "../../hooks/useTitle";
import { spacing } from "../../lib/theme/tokens";
import { useUrl } from "../../streamplace-store";
import { gap, layout, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { ShareSheet } from "../share/sharesheet";
import { Avatar, Button } from "../ui";
import { Text } from "../ui/text";
import { LikeButton } from "./like-button";

const NARROW_BREAKPOINT = 480;

// rkeyFromAturi pulls the record key off an at:// URI
// (at://<did>/place.stream.video/<rkey>).
function rkeyFromAturi(aturi: string): string {
  return aturi.split("/").pop() ?? "";
}

// A title and creator handle with the video's own like and share actions.
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
  const profile = author ?? video.author;
  const userSeg = formatHandle(profile);
  const rkey = rkeyFromAturi(aturi);

  // The video's canonical page + embed URLs, so Share actually links to this
  // VOD rather than the bare site root.
  const shareTarget = {
    url: `${baseUrl}/${userSeg}/video/${rkey}`,
    embedUrl: `${baseUrl}/embed/${userSeg}/video/${rkey}`,
    message: `Check out "${title || "this video"}" on Streamplace!`,
  };

  const titleEl = (
    <Text size="base" weight="semibold" numberOfLines={2}>
      {title || "Untitled"}
    </Text>
  );

  const channel = (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[2],
        { flexShrink: 1, minWidth: 0 },
      ]}
    >
      <Avatar src={avatar} name={userSeg} size="md" />
      <Text
        size="sm"
        weight="medium"
        numberOfLines={1}
        style={{ flexShrink: 1, minWidth: 0 }}
      >
        {formatHandleWithAt(profile)}
      </Text>
    </View>
  );

  const actions = (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[4],
        { flexShrink: 0 },
      ]}
    >
      <LikeButton subjectUri={video.uri} />
      <ShareSheet target={shareTarget}>
        <Button variant="secondary" size="sm" width="min">
          <Share2 size={spacing[4]} color={theme.colors.foreground} />
          <Text size="sm">Share</Text>
        </Button>
      </ShareSheet>
    </View>
  );

  if (wide) {
    return (
      <View style={{ gap: spacing[2] }}>
        {titleEl}
        <View
          style={[
            layout.flex.row,
            layout.flex.alignCenter,
            layout.flex.spaceBetween,
            gap.all[3],
          ]}
        >
          {channel}
          {actions}
        </View>
      </View>
    );
  }

  return (
    <View style={{ gap: spacing[2] }}>
      {titleEl}
      {channel}
      {actions}
    </View>
  );
}
