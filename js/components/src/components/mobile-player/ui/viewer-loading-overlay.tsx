import { useEffect } from "react";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { KeepAwake, Loader, PlayerStatus, usePlayerStore } from "../../..";

export function ViewerLoadingOverlay() {
  const status = usePlayerStore((x) => x.status);
  const opacity = useSharedValue(0);

  useEffect(() => {
    if (status === PlayerStatus.PLAYING || status === PlayerStatus.SUSPEND) {
      opacity.value = withTiming(0, { duration: 300 });
    } else {
      opacity.value = withTiming(1, { duration: 300 });
    }
  }, [status, opacity]);

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: opacity.value,
  }));

  if (status === PlayerStatus.PLAYING) {
    return <KeepAwake />;
  }

  if (status === PlayerStatus.SUSPEND || status === PlayerStatus.PAUSE) {
    return null;
  }

  return (
    <Animated.View
      style={[
        {
          position: "absolute",
          width: "100%",
          height: "100%",
          alignItems: "center",
          justifyContent: "center",
          pointerEvents: "none",
        },
        animatedStyle,
      ]}
    >
      <Loader size="large" />
    </Animated.View>
  );
}
