import { Pause, Play, RotateCcw } from "lucide-react-native";
import { ReactNode } from "react";
import { Platform } from "react-native";
import {
  colors,
  spacing,
  tabularNums,
  touchTargets,
} from "../../../lib/theme/tokens";
import { PlayerStatus, usePlayerStore } from "../../../player-store";
import { IconButton, Text, View } from "../../ui";

function formatTime(seconds: number): string {
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (h > 0) return `${h}:${pad(m)}:${pad(sec)}`;
  return `${m}:${pad(sec)}`;
}

export function VodControls({ children }: { children?: ReactNode }) {
  const mode = usePlayerStore((x) => x.mode);
  const status = usePlayerStore((x) => x.status);
  const togglePlayPause = usePlayerStore((x) => x.togglePlayPause);

  const playTime = usePlayerStore((x) => x.playTime);
  const duration = usePlayerStore((x) => x.duration);

  if (mode !== "vod") return null;

  const isPlaying = status === PlayerStatus.PLAYING;
  const hasEnded = !isPlaying && duration > 0 && playTime >= duration;

  return (
    <View
      style={{
        flexDirection: "row",
        alignItems: "center",
        paddingHorizontal: spacing[2],
        gap: spacing[2],
        minHeight: touchTargets.minimum,
      }}
    >
      <IconButton
        size="md"
        style={
          Platform.OS === "web"
            ? {
                minWidth: touchTargets.minimum,
                minHeight: touchTargets.minimum,
              }
            : undefined
        }
        accessibilityLabel={hasEnded ? "Replay" : isPlaying ? "Pause" : "Play"}
        onPress={togglePlayPause}
      >
        {hasEnded ? (
          <RotateCcw size={spacing[6]} color={colors.white} />
        ) : isPlaying ? (
          <Pause size={spacing[6]} color={colors.white} fill={colors.white} />
        ) : (
          <Play size={spacing[6]} color={colors.white} fill={colors.white} />
        )}
      </IconButton>
      <View style={{ flexDirection: "row", gap: spacing[1] }}>
        <Text size="sm" style={[tabularNums, { color: colors.white }]}>
          {formatTime(playTime)}
        </Text>
        <Text size="sm" style={{ color: colors.white }}>
          /
        </Text>
        <Text size="sm" style={[tabularNums, { color: colors.white }]}>
          {formatTime(duration)}
        </Text>
      </View>
      {children}
    </View>
  );
}
