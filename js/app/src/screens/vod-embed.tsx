import {
  LivestreamProvider,
  VideoProvider,
  View,
  VodPlayer,
  zero,
} from "@streamplace/components";
import { Redirect } from "components/aqlink";
import { BottomControlBar } from "components/mobile/desktop-ui/bottom-controls";
import { TopControlBar } from "components/mobile/desktop-ui/top-controls";
import { useEffect } from "react";
import { useStore } from "store";

const { layout, h, w, position, px, py } = zero;

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
        <View style={[zero.flex.values[1], { backgroundColor: "#000" }]}>
          <VodPlayer src={aturi}>
            <View
              style={[layout.position.absolute, h.percent[100], w.percent[100]]}
              pointerEvents="box-none"
            >
              <View
                style={[
                  layout.position.absolute,
                  {
                    top: 0,
                    left: 0,
                    right: 0,
                    padding: 16,
                    paddingTop: 20,
                    backgroundColor: "rgba(0,0,0,0.6)",
                  },
                ]}
              >
                <TopControlBar
                  offline={false}
                  isActivelyLive={false}
                  ingest={null}
                  isChatOpen={false}
                  onToggleChat={() => {}}
                  embedded={true}
                />
              </View>
              <View
                style={[
                  layout.position.absolute,
                  position.bottom[0],
                  w.percent[100],
                ]}
              >
                <View
                  style={{
                    backgroundColor: "rgba(0,0,0,0.7)",
                    paddingBottom: 4,
                  }}
                >
                  <BottomControlBar
                    ingest={null}
                    pipSupported={false}
                    pipActive={false}
                    onHandlePip={() => {}}
                    showChat={false}
                  />
                </View>
              </View>
            </View>
          </VodPlayer>
        </View>
      </LivestreamProvider>
    </VideoProvider>
  );
}
