import { zero } from "@streamplace/components";
import { Platform, Text, View } from "react-native";

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
      <View style={[layout.flex.center]}>
        <Text style={{ color: "white", fontSize: 18 }}>
          Dashboard Components Coming Soon
        </Text>
        <Text style={{ color: "gray", fontSize: 14, marginTop: 8 }}>
          Live Status: {isLive ? "LIVE" : "OFFLINE"}
        </Text>
      </View>
    </View>
  );
}
