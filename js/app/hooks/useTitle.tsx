import { useNavigation } from "@react-navigation/native";
import { useBrandingAsset, useVideo } from "@streamplace/components";
import { formatHandleWithAt } from "@streamplace/core";
import { useEffect } from "react";

export default function useTitle(user: string) {
  const navigation = useNavigation();
  const title = useBrandingAsset("siteTitle")?.data || "Streamplace";
  useEffect(() => {
    // the route names the streamer by handle or bare DID; DIDs never get an @
    navigation.setOptions({
      title: `${user?.startsWith("did:") ? user : `@${user}`} on ${title}`,
    });
  }, [user, navigation]);
}

// Names a video page after the video, which on web is the document title;
// react-navigation otherwise falls back to the route name ("Video"). Must be
// called inside a VideoProvider. An untitled video is named after its author,
// as a stream page is.
export function useVideoTitle() {
  const navigation = useNavigation();
  const video = useVideo();
  const siteTitle = useBrandingAsset("siteTitle")?.data || "Streamplace";
  const title =
    video?.record.title?.trim() ||
    (video ? `${formatHandleWithAt(video.author)} on ${siteTitle}` : undefined);
  useEffect(() => {
    if (title) {
      navigation.setOptions({ title });
    }
  }, [title, navigation]);
}
