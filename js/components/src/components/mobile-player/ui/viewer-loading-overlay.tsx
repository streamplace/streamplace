import { Play } from "lucide-react-native";
import { useEffect } from "react";
import { Pressable } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import {
  KeepAwake,
  Loader,
  PlayerStatus,
  usePlayerStore,
  useTheme,
} from "../../..";

export function ViewerLoadingOverlay() {
  const status = usePlayerStore((x) => x.status);
  const togglePlayPause = usePlayerStore((x) => x.togglePlayPause);
  const { theme, zero: zt } = useTheme();
  const opacity = useSharedValue(0);

  useEffect(() => {
    if (status === PlayerStatus.PLAYING || status === PlayerStatus.SUSPEND) {
      opacity.value = withTiming(0, { duration: 300 });
    } else {
      opacity.value = withTiming(1, { duration: 300 });
    }
  }, [status, opacity]);

  const animatedStyle = useAnimatedStyle(() => {
    return {
      opacity: opacity.value,
    };
  });

  if (status === PlayerStatus.PLAYING) {
    return <KeepAwake />;
  }

  if (status === PlayerStatus.SUSPEND) {
    return null; // No overlay when stopped
  }

  const isPaused = status === PlayerStatus.PAUSE;

  return (
    <>
      <Animated.View
        style={[
          {
            position: "absolute",
            width: "100%",
            height: "100%",
            alignItems: "center",
            justifyContent: "center",
            backgroundColor: "rgba(0,0,0,0.3)",
            pointerEvents: "none",
          },
          animatedStyle,
        ]}
      >
        {!isPaused && <Loader size="large" />}
      </Animated.View>
      {isPaused && (
        <Pressable
          onPress={togglePlayPause}
          style={{
            position: "absolute",
            alignSelf: "center",
            top: "50%",
            marginTop: -32,
            backgroundColor: "rgba(0,0,0,0.45)",
            borderRadius: 999,
            padding: 16,
          }}
        >
          <Play
            size={32}
            color={theme.colors.foreground}
            fill={theme.colors.foreground}
          />
        </Pressable>
      )}
    </>
  );
}
