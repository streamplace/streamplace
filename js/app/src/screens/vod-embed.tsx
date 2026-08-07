import {
  LivestreamProvider,
  VideoProvider,
  View,
  VodPlayer,
  zero,
} from "@streamplace/components";
import { colors } from "@streamplace/components/src/lib/theme/tokens";
import { Redirect } from "components/aqlink";
import { DesktopUi } from "components/mobile/desktop-ui";
import { useEffect } from "react";
import { useStore } from "store";

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
      <LivestreamProvider src={aturi}>
        <View style={[zero.flex.values[1], { backgroundColor: colors.black }]}>
          <VodPlayer src={aturi} embedded={true}>
            <DesktopUi />
          </VodPlayer>
        </View>
      </LivestreamProvider>
    </VideoProvider>
  );
}
