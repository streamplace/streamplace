import { Image } from "expo-image";
import { ScrollView, useWindowDimensions, View } from "react-native";
import { useAuthor } from "../../hooks/useAuthor";
import { useAvatar } from "../../hooks/useAvatar";
import { useTitle } from "../../hooks/useTitle";
import { spacing } from "../../lib/theme/tokens";
import { useUrl } from "../../streamplace-store";
import { gap, layout, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { ShareSheet } from "../share/sharesheet";
import { Button } from "../ui";
import { Text } from "../ui/text";
import { LikeButton } from "./like-button";

const NARROW_BREAKPOINT = 480;

// rkeyFromAturi pulls the record key off an at:// URI
// (at://<did>/place.stream.video/<rkey>).
function rkeyFromAturi(aturi: string): string {
  return aturi.split("/").pop() ?? "";
}

// YouTube-grammar VOD header: a prominent title, then a channel identity row
// (avatar + name + handle) with the actions — like, share — on the right.
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
  const displayName = author?.displayName || handle || did;
  const userSeg = handle || did;
  const rkey = rkeyFromAturi(aturi);

  // The video's canonical page + embed URLs, so Share actually links to this
  // VOD rather than the bare site root.
  const shareTarget = {
    url: `${baseUrl}/${userSeg}/video/${rkey}`,
    embedUrl: `${baseUrl}/embed/${userSeg}/video/${rkey}`,
    message: `Check out "${title || "this video"}" on Streamplace!`,
  };

  const titleEl = (
    <Text size={wide ? "xl" : "lg"} weight="semibold" numberOfLines={2}>
      {title || "Untitled"}
    </Text>
  );

  const channel = (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[3],
        { flexShrink: 1, minWidth: 0 },
      ]}
    >
      <View
        style={{
          width: 40,
          height: 40,
          borderRadius: 999,
          overflow: "hidden",
          backgroundColor: theme.colors.surface2,
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
      <View style={{ flexShrink: 1, minWidth: 0 }}>
        <Text weight="semibold" numberOfLines={1}>
          {displayName}
        </Text>
        {handle ? (
          <Text
            size="sm"
            numberOfLines={1}
            style={{ color: theme.colors.text3 }}
          >
            @{handle}
          </Text>
        ) : null}
      </View>
    </View>
  );

  const actions = (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[2],
        { flexShrink: 0 },
      ]}
    >
      <Button variant="secondary" size="pill" width="min">
        <LikeButton subjectUri={video.uri} />
      </Button>
      <ShareSheet target={shareTarget} />
    </View>
  );

  if (wide) {
    return (
      <View style={{ gap: spacing[3] }}>
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
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={{ gap: spacing[2], alignItems: "center" }}
      >
        {actions}
      </ScrollView>
    </View>
  );
}
