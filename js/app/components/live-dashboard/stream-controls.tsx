import { zero } from "@streamplace/components";
import { Play, Square } from "@tamagui/lucide-icons";
import { Pressable, Text, View } from "react-native";

const { flex, bg, r, borders, p, px, py, text, layout, gap } = zero;

// Mock data - replace with real data from your stores
const mockStats = {
  uptime: "02:34:12",
};

interface StreamControlsProps {
  isLive: boolean;
}

export default function StreamControls({ isLive }: StreamControlsProps) {
  return (
    <View
      style={[
        flex.values[1],
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        p[4],
        layout.flex.row,
        layout.flex.alignCenter,
        gap.all[3],
      ]}
    >
      <Pressable
        style={[
          bg[isLive ? "red" : "green"][500],
          r[2],
          px[4],
          py[3],
          layout.flex.row,
          layout.flex.alignCenter,
          gap.all[2],
        ]}
      >
        {isLive ? (
          <Square size={20} color="white" />
        ) : (
          <Play size={20} color="white" />
        )}
        <Text style={[text.white, { fontSize: 16, fontWeight: "600" }]}>
          {isLive ? "Stop Stream" : "Go Live"}
        </Text>
      </Pressable>

      <Text style={[text.gray[400], { fontSize: 14 }]}>
        Uptime: {mockStats.uptime}
      </Text>
    </View>
  );
}
