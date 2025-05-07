import React, { useEffect } from "react";
import { StyleSheet, View } from "react-native";
import Animated, {
  useSharedValue,
  useAnimatedStyle,
  withTiming,
  withRepeat,
} from "react-native-reanimated";

export default function LiveDot() {
  // Use useSharedValue for the animation value
  const pulseAnim = useSharedValue(1);
  const opacityAnim = useSharedValue(1);

  useEffect(() => {
    pulseAnim.value = withRepeat(withTiming(2.25, { duration: 1000 }), -1);
    opacityAnim.value = withRepeat(withTiming(0, { duration: 1000 }), -1);
  }, [pulseAnim, opacityAnim]);

  const animatedPulseStyle = useAnimatedStyle(() => {
    return {
      transform: [{ scale: pulseAnim.value }],
      opacity: opacityAnim.value,
    };
  });

  return (
    <View style={styles.container}>
      {/* Apply the animated style to the Animated.View */}
      <Animated.View
        style={[
          styles.pulseDot,
          animatedPulseStyle, // Apply the animated style
        ]}
      />
      <View style={styles.solidDot} />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    width: 24,
    height: 24,
    justifyContent: "center",
    alignItems: "center",
  },
  solidDot: {
    width: 12,
    height: 12,
    borderRadius: 999,
    backgroundColor: "red",
    position: "absolute",
  },
  pulseDot: {
    width: 10,
    height: 10,
    borderRadius: 5,
    backgroundColor: "red",
    position: "absolute",
  },
});
