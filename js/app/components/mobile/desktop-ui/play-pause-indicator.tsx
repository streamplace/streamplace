import { PlayerStatus, usePlayerStore } from "@streamplace/components";
import { Pause, Play } from "lucide-react-native";
import { useCallback, useEffect, useRef } from "react";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSequence,
  withTiming,
} from "react-native-reanimated";

export function PlayPauseIndicator() {
  const status = usePlayerStore((x) => x.status);
  const prevStatus = useRef(status);
  const opacity = useSharedValue(0);
  const scale = useSharedValue(0.5);

  const animate = useCallback(
    (toPlaying: boolean) => {
      opacity.value = 1;
      opacity.value = withSequence(
        withTiming(1, { duration: 100 }),
        withTiming(1, { duration: 400 }),
        withTiming(0, { duration: 300 }),
      );
      if (toPlaying) {
        scale.value = 0.8;
        scale.value = withSequence(
          withTiming(1, { duration: 100 }),
          withTiming(1, { duration: 400 }),
          withTiming(1.3, { duration: 300 }),
        );
      } else {
        scale.value = 1;
        scale.value = withSequence(
          withTiming(0.8, { duration: 100 }),
          withTiming(0.8, { duration: 500 }),
        );
      }
    },
    [opacity, scale],
  );

  useEffect(() => {
    if (status !== prevStatus.current) {
      const wasPlaying = prevStatus.current === PlayerStatus.PLAYING;
      const isPlaying = status === PlayerStatus.PLAYING;
      const isPaused = status === PlayerStatus.PAUSE;

      if (isPlaying || isPaused) {
        animate(isPlaying);
      }
      prevStatus.current = status;
    }
  }, [status, animate]);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: opacity.value,
    transform: [{ scale: scale.value }],
  }));

  const isPlaying = status === PlayerStatus.PLAYING;

  return (
    <Animated.View
      style={[
        {
          position: "absolute",
          top: "50%",
          left: "50%",
          marginLeft: -32,
          marginTop: -32,
          width: 64,
          height: 64,
          borderRadius: 32,
          backgroundColor: "rgba(0,0,0,0.5)",
          justifyContent: "center",
          alignItems: "center",
          pointerEvents: "none",
        },
        animatedStyle,
      ]}
    >
      {isPlaying ? (
        <Play size={32} color="white" fill="white" />
      ) : (
        <Pause size={32} color="white" fill="white" />
      )}
    </Animated.View>
  );
}
