import {
  Panda,
  Rabbit,
  Snail,
  StepBack,
  StepForward,
} from "lucide-react-native";
import { useEffect } from "react";
import { layout, layouts, spacing, zIndex } from "../../../lib/theme/atoms";
import { borderRadius, playerActionFeedback } from "../../../lib/theme/tokens";
import { usePlayerStore } from "../../../player-store";
import { Text, View } from "../../ui";

const FEEDBACK_DURATION_MS = 1500;

export function ActionFeedback() {
  const feedback = usePlayerStore((state) => state.feedback);
  const clearFeedback = usePlayerStore((state) => state.clearFeedback);

  useEffect(() => {
    if (!feedback) return;
    const timeout = setTimeout(clearFeedback, FEEDBACK_DURATION_MS);
    return () => clearTimeout(timeout);
  }, [clearFeedback, feedback]);

  if (!feedback) return null;

  const Icon =
    feedback.type === "speed"
      ? feedback.playbackRate > 1
        ? Rabbit
        : feedback.playbackRate < 1
          ? Snail
          : Panda
      : feedback.direction === "forward"
        ? StepForward
        : StepBack;
  const message =
    feedback.type === "speed"
      ? `Playback speed: ${feedback.playbackRate}×`
      : feedback.direction === "forward"
        ? "Frame forward"
        : "Frame back";

  return (
    <View
      pointerEvents="none"
      style={[layouts.fullScreen, layout.flex.center, zIndex[20]]}
    >
      <View style={{ alignItems: "center", gap: spacing[2] }}>
        <View
          style={{
            alignItems: "center",
            justifyContent: "center",
            padding: spacing[2],
            borderRadius: borderRadius.full,
            backgroundColor: playerActionFeedback.background,
          }}
        >
          <View style={{ opacity: playerActionFeedback.foregroundOpacity }}>
            <Icon size={spacing[12]} color={playerActionFeedback.foreground} />
          </View>
        </View>
        <View
          style={{
            paddingHorizontal: spacing[3],
            paddingVertical: spacing[1],
            borderRadius: borderRadius.md,
            backgroundColor: playerActionFeedback.background,
          }}
        >
          <Text
            size="sm"
            style={{
              color: playerActionFeedback.foreground,
              opacity: playerActionFeedback.foregroundOpacity,
              fontVariant: ["tabular-nums"],
            }}
          >
            {message}
          </Text>
        </View>
      </View>
    </View>
  );
}
