import { zero } from "@streamplace/components";
import { Settings } from "@tamagui/lucide-icons";
import { Pressable, Text, View } from "react-native";

const { flex, bg, r, borders, p, text, layout } = zero;

export default function QuickSettings() {
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
        layout.flex.spaceBetween,
      ]}
    >
      <Text style={[text.white, { fontSize: 16, fontWeight: "600" }]}>
        Settings
      </Text>
      <Pressable style={[bg.gray[700], r[2], p[2]]}>
        <Settings size={20} color="white" />
      </Pressable>
    </View>
  );
}
