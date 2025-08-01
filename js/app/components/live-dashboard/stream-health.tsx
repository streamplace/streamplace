import { zero } from "@streamplace/components";
import { Text, View } from "react-native";

const { flex, bg, r, borders, p, text, mb, gap, layout } = zero;

// Mock data - replace with real data from your stores
const mockStats = {
  bitrate: "2500 kbps",
  fps: 30,
  resolution: "1920x1080",
};

export default function StreamHealth() {
  return (
    <View
      style={[
        flex.values[1],
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        p[4],
      ]}
    >
      <Text style={[text.white, mb[2], { fontSize: 16, fontWeight: "600" }]}>
        Stream Health
      </Text>
      <View style={[gap.all[1], layout.flex.row]}>
        <Text style={[text.gray[300], { fontSize: 12 }]}>
          Bitrate: {mockStats.bitrate}
        </Text>
        <Text style={[text.gray[300], { fontSize: 12 }]}>
          FPS: {mockStats.fps}
        </Text>
        <Text style={[text.gray[300], { fontSize: 12 }]}>
          Resolution: {mockStats.resolution}
        </Text>
      </View>
    </View>
  );
}
