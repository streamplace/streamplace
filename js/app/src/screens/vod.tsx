import { VideoProvider, View, VodPlayer, zero } from "@streamplace/components";
import { Redirect } from "components/aqlink";

// A pared-down VOD route (/:user/vod/:tid) that drops the unified live+vod
// player in favor of the minimal <VodPlayer>. VideoProvider supplies the video
// context (title/author/etc.) for anything we want to render around it; the
// player itself is just a thin wrapper over the <Video> element.
export default function VodScreen({
  route,
}: {
  route?: { params?: { user?: string; tid?: string } };
}) {
  if (!route?.params?.user || !route?.params?.tid) {
    return <Redirect to={{ screen: "HomeMain" }} />;
  }
  const aturi = `at://${route.params.user}/place.stream.video/${route.params.tid}`;

  return (
    <VideoProvider aturi={aturi}>
      <View style={[zero.flex.values[1], { backgroundColor: "#000" }]}>
        <View style={{ flex: 1 }}>
          <VodPlayer src={aturi} />
        </View>
      </View>
    </VideoProvider>
  );
}
