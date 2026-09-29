import { useNavigation } from "@react-navigation/native";
import { useBrandingAsset, useVideo } from "@streamplace/components";
import { useEffect } from "react";

export default function useTitle(user: string) {
  const navigation = useNavigation();
  const title = useBrandingAsset("siteTitle")?.data || "Streamplace";
  useEffect(() => {
    navigation.setOptions({
      title: `@${user} on ${title}`,
    });
  }, [user, navigation]);
}

// Names a video page after the video, which on web is the document title;
// react-navigation otherwise falls back to the route name ("Video"). Must be
// called inside a VideoProvider.
export function useVideoTitle() {
  const navigation = useNavigation();
  const title = useVideo()?.record.title?.trim();
  useEffect(() => {
    if (title) {
      navigation.setOptions({ title });
    }
  }, [title, navigation]);
}
