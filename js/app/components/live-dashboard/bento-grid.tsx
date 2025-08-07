import { zero } from "@streamplace/components";
import { Platform, View } from "react-native";
import ChatPanel from "./chat-panel";
import Header from "./header";
import LivestreamPanel from "./livestream-panel";
import ModActions from "./mod-actions";
import StreamMonitor from "./stream-monitor";

const { flex, p, gap, layout, bg } = zero;

interface BentoGridProps {
  userProfile: any;
  isLive: boolean;
  videoRef: any;
}

export default function BentoGrid({
  userProfile,
  isLive,
  videoRef,
}: BentoGridProps) {
  const isWeb = Platform.OS === "web";

  return (
    <View style={[flex.values[1], gap.all[4], p[4], bg.black]}>
      <View style={[layout.flex.column, { minWidth: isWeb ? 400 : "100%" }]}>
        <Header isLive={isLive} />
      </View>
      <View style={[flex.values[1], layout.flex.row, gap.all[4]]}>
        <View style={[flex.values[4], gap.all[4]]}>
          <View
            style={[
              flex.values[2],
              layout.flex.row,
              gap.all[4],
              { height: isWeb ? 300 : 200 },
            ]}
          >
            <StreamMonitor
              userProfile={userProfile}
              isLive={isLive}
              videoRef={videoRef}
            />
          </View>

          <View style={[layout.flex.row, gap.all[4], flex.values[1]]}>
            <ModActions isLive={isLive} />
          </View>
        </View>

        <View
          style={[
            flex.values[2],
            layout.flex.column,
            gap.all[4],
            { maxWidth: isWeb ? 600 : "100%" },
          ]}
        >
          <ChatPanel />
        </View>
        <View
          style={[
            flex.values[2],
            layout.flex.column,
            gap.all[4],
            { maxWidth: isWeb ? 600 : "100%" },
          ]}
        >
          <LivestreamPanel />
        </View>
      </View>
    </View>
  );
}
