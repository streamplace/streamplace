import { useStreamplaceStore, View, zero } from "@streamplace/components";
import { colors } from "@streamplace/components/src/lib/theme/tokens";
import { Redirect } from "components/aqlink";
import { Player } from "../../components/mobile/player";

const BBB_HLS_URL = "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8";

function usePlaybackUrl(user: string, tid: string): string {
  const serverUrl = useStreamplaceStore((x) => x.url);
  return `at://${user}/place.stream.video/${tid}`;
}

export default function VideoScreen({
  route,
}: {
  route?: { params?: { user?: string; tid?: string } };
}) {
  if (!route?.params?.user || !route?.params?.tid) {
    return <Redirect to={{ screen: "HomeMain" }} />;
  }
  const url = usePlaybackUrl(route.params.user, route.params.tid);

  return (
    <View style={[zero.flex.values[1], { backgroundColor: colors.black }]}>
      <View style={{ flex: 1 }}>
        <Player src={url} mode="vod" />
      </View>
      {/*{src && (
        <View
          style={[
            zero.px[4],
            zero.py[3],
            { backgroundColor: surfaces.dark[1] },
          ]}
        >
          <Text size="sm" style={{ color: textAlphas.dark[2] }}>
            {src}
          </Text>
        </View>
      )}*/}
    </View>
  );
}
