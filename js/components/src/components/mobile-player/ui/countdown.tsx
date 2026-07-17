import { useEffect, useState } from "react";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useFrameCallback,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { scrims } from "../../../lib/theme/tokens";

type CountdownOverlayProps = {
  visible: boolean;
  width: number;
  height: number;
  startFrom?: number;
  onDone?: () => void;
};

export function CountdownOverlay({
  visible,
  width,
  height,
  startFrom = 3,
  onDone,
}: CountdownOverlayProps) {
  const [countdown, setCountdown] = useState(startFrom);

  const startTimestamp = useSharedValue<number | null>(null);
  const done = useSharedValue(false);

  // Animation values
  const scale = useSharedValue(1);
  const opacity = useSharedValue(1);

  const updateCountdown = (value: number) => {
    setCountdown(value);
  };

  const handleDone = () => {
    if (onDone) onDone();
  };

  // Accurate countdown using useFrameCallback
  useFrameCallback(({ timestamp }) => {
    if (!visible) return;

    // Set start timestamp on first frame
    if (startTimestamp.value === null) {
      startTimestamp.value = timestamp;
      return;
    }

    const elapsed = (timestamp - startTimestamp.value) / 1000; // Convert to seconds
    const remaining = Math.max(0, startFrom - Math.floor(elapsed));

    // Use runOnJS to call JavaScript functions from worklet
    runOnJS(updateCountdown)(remaining);

    if (remaining === 0 && !done.value) {
      done.value = true;
      runOnJS(handleDone)();
    }
  });

  useEffect(() => {
    if (visible) {
      startTimestamp.value = null; // Will be set on first frame
      setCountdown(startFrom);
      done.value = false;
    } else {
      setCountdown(startFrom);
    }
  }, [visible, startFrom]);

  // Animate scale and opacity on countdown change
  useEffect(() => {
    if (visible && countdown > 0) {
      scale.value = 1;
      opacity.value = 1;
      scale.value = withTiming(1.5, { duration: 1000 });
      opacity.value = withTiming(0, { duration: 1000 });
    }
  }, [countdown, visible, scale, opacity]);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
    opacity: opacity.value,
  }));

  if (!visible || countdown === 0) return null;

  return (
    <Animated.View
      style={{
        position: "absolute",
        top: 0,
        left: 0,
        width,
        height,
        backgroundColor: scrims.dark,
        alignItems: "center",
        justifyContent: "center",
        zIndex: 1000,
      }}
    >
      <Animated.Text
        style={[
          {
            color: "white",
            fontSize: 120,
            fontWeight: "bold",
          },
          animatedStyle,
        ]}
      >
        {typeof countdown === "number" ? countdown : ""}
      </Animated.Text>
    </Animated.View>
  );
}
