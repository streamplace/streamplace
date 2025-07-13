import { useEffect, useState } from "react";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withTiming,
} from "react-native-reanimated";
import { pt } from "../../../lib/theme/atoms";

type LoadingOverlayProps = {
  visible: boolean;
  width: number;
  height: number;
  subtitle?: string;
  messages?: string[];
  interval?: number; // in milliseconds
};

const defaultMessages = [
  "Creating your stream",
  "Uploading thumbnails",
  "Getting things ready",
  "Doing some magic",
  "Preparing something special",
  "Reticulating splines",
  "Making it nice",
  "Flipping some switches",
  "Adding good vibes",
  "Almost there",
  "Summoning your Persona",
  "Awakening our true selves",
  "Fusion in progress",
  "Equipping the right materia",
];

export function LoadingOverlay({
  visible,
  width,
  height,
  subtitle,
  messages = defaultMessages,
  interval = 3000,
}: LoadingOverlayProps) {
  const [currentIndex, setCurrentIndex] = useState(0);
  const [shouldRender, setShouldRender] = useState(visible);

  // Animation values
  const translateY = useSharedValue(0);
  const opacity = useSharedValue(1);

  const wholeOpacity = useSharedValue(0);

  // Handle fade-in and fade-out animations
  useEffect(() => {
    if (visible) {
      setShouldRender(true); // Ensure the component is mounted
      wholeOpacity.value = withTiming(1, { duration: 500 }); // Fade in
    } else {
      wholeOpacity.value = withTiming(0, { duration: 500 }, () => {
        // Unmount after fade-out
        runOnJS(setShouldRender)(false);
      });
    }
  }, [visible]);

  // Cycle messages on a timer
  useEffect(() => {
    if (!visible) {
      setCurrentIndex(0);
      return;
    }

    const timeout = setTimeout(() => {
      setCurrentIndex((prev) => (prev + 1) % messages.length);
    }, interval);

    return () => clearTimeout(timeout);
  }, [visible, currentIndex, interval, messages.length]);

  // Trigger animation on each message change
  useEffect(() => {
    if (!visible) return;

    const fadeDuration = Math.min(interval / 2, 250); // Simplified fade duration

    // Reset animation values
    translateY.value = 20;
    opacity.value = 0;

    // Sequential fade-in and fade-out
    translateY.value = withTiming(0, { duration: fadeDuration });
    opacity.value = withTiming(1, { duration: fadeDuration }, () => {
      // add a delay for interval - (fadeDuration*2)

      translateY.value = withDelay(
        interval - fadeDuration * 2,
        withTiming(-10, { duration: fadeDuration }),
      );
      opacity.value = withDelay(
        interval - fadeDuration * 2,
        withTiming(0, { duration: fadeDuration }),
      );
    });
  }, [currentIndex, visible]);

  const animatedStyle = useAnimatedStyle(() => {
    return {
      transform: [{ translateY: translateY.value }],
      opacity: opacity.value,
    };
  });

  const wholeAnimatedStyle = useAnimatedStyle(() => ({
    opacity: wholeOpacity.value,
  }));

  if (!shouldRender) return null;

  return (
    <Animated.View
      style={[
        {
          position: "absolute",
          top: 0,
          left: 0,
          width,
          height,
          backgroundColor: "rgba(0,0,0,0.7)",
          alignItems: "center",
          justifyContent: "center",
          zIndex: 1000,
        },
        wholeAnimatedStyle,
      ]}
    >
      <Animated.Text
        style={[
          {
            color: "white",
            fontSize: 24,
            fontWeight: "bold",
          },
          animatedStyle,
        ]}
      >
        {messages[currentIndex]}
      </Animated.Text>
      <Animated.Text style={[pt[5], { color: "#a0a0a0" }]}>
        {subtitle}
      </Animated.Text>
    </Animated.View>
  );
}
