import { Image } from "expo-image";
import { ScrollView, useWindowDimensions, View } from "react-native";
import { useAuthor } from "../../hooks/useAuthor";
import { useAvatar } from "../../hooks/useAvatar";
import { useTitle } from "../../hooks/useTitle";
import { useUrl } from "../../streamplace-store";
import { gap, layout, useTheme } from "../../ui";
import { useVideoStore } from "../../video-store/video-store";
import { Viewers } from "../mobile-player/ui/viewers";
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

  const handleText = (
    <Text style={{ color: theme.colors.textMuted, flexShrink: 0 }}>
      {handle ? `@${handle}` : did}
    </Text>
  );

  const actions = (
    <View
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[3],
        { flexShrink: 0 },
      ]}
    >
      <Button variant="secondary" size="pill" width="min">
        <LikeButton subjectUri={video.uri} />
      </Button>

      {wide ? (
        <Button variant="secondary" size="pill" width="min">
          <Viewers />
        </Button>
      ) : null}
      <ShareSheet target={shareTarget} />
    </View>
  );

  if (wide) {
    return (
      <View style={{ gap: 4 }}>
        <View style={[layout.flex.row, layout.flex.justify.start, gap.all[3]]}>
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
          <View>
            <View
              style={[layout.flex.row, layout.flex.alignCenter, gap.all[3]]}
            >
              <Text
                weight="semibold"
                numberOfLines={1}
                ellipsizeMode="tail"
                style={{ flex: 1, minWidth: 0 }}
              >
                {title || "Untitled"}
              </Text>
            </View>
            <View
              style={[
                layout.flex.row,
                layout.flex.alignCenter,
                { flexWrap: "wrap", gap: 6 },
              ]}
            >
              {handleText}
              {actions}
            </View>
          </View>
        </View>
      </View>
    );
  }

  return (
    <View style={{ gap: 6 }}>
      <Text weight="semibold" numberOfLines={1} ellipsizeMode="tail">
        {title || "Untitled"}
      </Text>
      {handleText}
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={{ gap: 6, alignItems: "center" }}
      >
        {actions}
      </ScrollView>
    </View>
  );
}
