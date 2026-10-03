import {
  useStreamplaceStore,
  VideoProvider,
  View,
  zero,
} from "@streamplace/components";
import { colors } from "@streamplace/components/src/lib/theme/tokens";
import { parseTimeParam } from "@streamplace/core";
import { Redirect } from "components/aqlink";
import { useVideoTitle } from "hooks/useTitle";
import { Player } from "../../components/mobile/player";

const BBB_HLS_URL = "https://test-streams.mux.dev/x36xhzz/x36xhzz.m3u8";

function usePlaybackUrl(user: string, tid: string): string {
  const serverUrl = useStreamplaceStore((x) => x.url);
  return `at://${user}/place.stream.video/${tid}`;
}

function VideoTitle() {
  useVideoTitle();
  return null;
}

export default function VideoScreen({
  route,
}: {
  route?: { params?: { user?: string; tid?: string; t?: string } };
}) {
  const url = usePlaybackUrl(
    route?.params?.user ?? "",
    route?.params?.tid ?? "",
  );
  // Deep links carry the playback start as `?t=` (seconds, or `1h2m3s`).
  const startTime = parseTimeParam(route?.params?.t) ?? undefined;
  if (!route?.params?.user || !route?.params?.tid) {
    return <Redirect to={{ screen: "HomeMain" }} />;
  }

  // The player mounts its own VideoProvider, which defers to this one; this
  // one lets the screen read the video too.
  return (
    <VideoProvider aturi={url}>
      <VideoTitle />
      <View style={[zero.flex.values[1], { backgroundColor: colors.black }]}>
        <View style={{ flex: 1 }}>
          <Player src={url} mode="vod" startTime={startTime} />
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
    </VideoProvider>
  );
}
