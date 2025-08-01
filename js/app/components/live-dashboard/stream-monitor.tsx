import { Player, zero } from "@streamplace/components";
import { Camera } from "@tamagui/lucide-icons";
import { Text, View } from "react-native";

const { flex, bg, r, borders, layout, p, text, w, h } = zero;

interface StreamMonitorProps {
  userProfile: any;
  isLive: boolean;
  videoRef: any;
}

export default function StreamMonitor({
  userProfile,
  isLive,
  videoRef,
}: StreamMonitorProps) {
  return (
    <View
      style={[
        flex.values[2],
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        layout.flex.column,
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          p[4],
          borders.bottom.width.thin,
          borders.bottom.color.gray[700],
        ]}
      >
        <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
          Stream Monitor
        </Text>
        <View style={[layout.flex.row, layout.flex.alignCenter, { gap: 8 }]}>
          <View style={[w[2], h[2], r[1], bg[isLive ? "green" : "red"][500]]} />
          <Text style={[text.gray[400], { fontSize: 14 }]}>
            {isLive ? "LIVE" : "OFFLINE"}
          </Text>
        </View>
      </View>

      <View style={[flex.values[1], layout.flex.center, bg.gray[900]]}>
        {isLive ? (
          <Player src={userProfile.did} name={userProfile.handle} />
        ) : (
          <View style={[layout.flex.center, { gap: 12 }]}>
            <Camera size={48} color="#6b7280" />
            <Text style={[text.gray[400]]}>Stream Offline</Text>
          </View>
        )}
      </View>
    </View>
  );
}
