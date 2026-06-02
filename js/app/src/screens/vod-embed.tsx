import { VideoProvider, View, VodPlayer, zero } from "@streamplace/components";
import { Redirect } from "components/aqlink";
import { useEffect } from "react";
import { useStore } from "store";

// Chrome-less VOD player for iframe embeds (/embed/:user/video/:tid). Mirrors
// the live EmbedScreen: hide the sidebar and render just the minimal
// <VodPlayer> (expo-video's native controls handle play/scrub) with no back
// button or surrounding metadata.
export default function VodEmbedScreen({
  route,
}: {
  route?: { params?: { user?: string; tid?: string } };
}) {
  const setSidebarHidden = useStore((state) => state.setSidebarHidden);
  const setSidebarUnhidden = useStore((state) => state.setSidebarUnhidden);

  useEffect(() => {
    setSidebarHidden();
    return () => {
      setSidebarUnhidden();
    };
  }, [setSidebarHidden, setSidebarUnhidden]);

  if (!route?.params?.user || !route?.params?.tid) {
    return <Redirect to={{ screen: "HomeMain" }} />;
  }
  const aturi = `at://${route.params.user}/place.stream.video/${route.params.tid}`;

  return (
    <VideoProvider aturi={aturi}>
      <View style={[zero.flex.values[1], { backgroundColor: "#000" }]}>
        <VodPlayer src={aturi} />
      </View>
    </VideoProvider>
  );
}
