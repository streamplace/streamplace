import { Play } from "lucide-react-native";
import { useEffect } from "react";
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
  const theme = useTheme();
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

  let spinner = <Loader size="large" />;
  if (status === PlayerStatus.PAUSE) {
    spinner = <Play size="$12" color={theme.styles.text.primary["color"]} />;
  }

  return (
    <Animated.View
      style={[
        {
          position: "absolute",
          width: "100%",
          height: "100%",
          zIndex: 998,
          alignItems: "center",
          justifyContent: "center",
          backgroundColor: "rgba(0,0,0,0.3)",
        },
        animatedStyle,
      ]}
    >
      {spinner}
    </Animated.View>
  );
}
