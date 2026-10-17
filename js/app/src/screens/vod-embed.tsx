import {
  LivestreamProvider,
  VideoProvider,
  View,
  VodPlayer,
  zero,
} from "@streamplace/components";
import { colors } from "@streamplace/components/src/lib/theme/tokens";
import { parseTimeParam } from "@streamplace/core";
import { Redirect } from "components/aqlink";
import { DesktopUi } from "components/mobile/desktop-ui";
import { useEffect } from "react";
import { useStore } from "store";

export default function VodEmbedScreen({
  route,
}: {
  route?: { params?: { user?: string; tid?: string; t?: string } };
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
  // Deep links carry the playback start as `?t=` (seconds, or `1h2m3s`).
  const startTime = parseTimeParam(route.params.t) ?? undefined;

  return (
    <VideoProvider aturi={aturi}>
      <LivestreamProvider src={aturi}>
        <View style={[zero.flex.values[1], { backgroundColor: colors.black }]}>
          <VodPlayer src={aturi} embedded={true} startTime={startTime}>
            <DesktopUi />
          </VodPlayer>
        </View>
      </LivestreamProvider>
    </VideoProvider>
  );
}
