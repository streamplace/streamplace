import { StyleSheet } from "react-native";
import { View } from "../ui";
import { DanmuOverlay } from "./danmu-overlay";

interface DanmuOverlayOBSProps {
  channelDid?: string;
  enabled?: boolean;
  opacity?: number;
  speed?: number;
  laneCount?: number;
  maxMessages?: number;
}

export function DanmuOverlayOBS({
  channelDid,
  enabled = true,
  opacity = 80,
  speed = 1,
  laneCount = 12,
  maxMessages = 50,
}: DanmuOverlayOBSProps) {
  return (
    <View style={styles.container}>
      <DanmuOverlay
        enabled={enabled}
        opacity={opacity}
        speed={speed}
        laneCount={laneCount}
        maxMessages={maxMessages}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    position: "absolute",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: "transparent",
  },
});
