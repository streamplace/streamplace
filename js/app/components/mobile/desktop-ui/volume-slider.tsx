import {
  Slider,
  useMuted,
  useSetMuted,
  useSetVolume,
  useVolume,
  View,
  zero,
} from "@streamplace/components";
import { Volume2, VolumeX } from "lucide-react-native";
import { useCallback } from "react";
import { Pressable } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";

const { layout, p, r } = zero;

export function VolumeSlider() {
  const muted = useMuted();
  const setMuted = useSetMuted();
  const volume = useVolume();
  const setVolume = useSetVolume();

  const fadeAnim = useSharedValue(0);
  const widthAnim = useSharedValue(0);

  const onVolumeHover = useCallback(() => {
    fadeAnim.value = withTiming(1, { duration: 200 });
    widthAnim.value = withTiming(200, { duration: 200 });
  }, [fadeAnim, widthAnim]);

  // Toggle mute state
  const handleMuteToggle = useCallback(() => {
    setMuted(!muted);
  }, [muted, setMuted]);

  const VolumeIcon = muted ? VolumeX : Volume2;

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: fadeAnim.value,
    width: widthAnim.value,
  }));

  // Convert volume (0-1) to percentage (0-100) for slider
  const sliderValue = (muted ? 0 : volume) * 100;
  return (
    <View
      onPointerEnter={onVolumeHover}
      style={[layout.flex.row, layout.flex.alignCenter, { height: 50 }]}
    >
      <Pressable onPress={handleMuteToggle} style={[p[2], r[1]]}>
        <VolumeIcon size={20} color="white" />
      </Pressable>

      <Animated.View style={[{ height: 30 }, animatedStyle]}>
        <Slider.Root
          style={{
            position: "relative",
            display: "flex",
            alignItems: "center",
            flex: 1,
            width: 200,
            height: 20,
          }}
          value={sliderValue}
          min={0}
          max={100} // Slider max value is 100 for percentage
          onValueChange={(vals) => {
            const newVolume = vals[0] / 100; // Convert back to 0-1 range
            setVolume(newVolume);
            if (newVolume === 0) {
              setMuted(true);
            } else {
              setMuted(false);
            }
          }}
          asChild
        >
          <Slider.Track
            style={{
              flexGrow: 1,
              height: 30,
              position: "relative",
              flex: 1,
            }}
          >
            <Slider.Range
              style={{
                position: "absolute",
                backgroundColor: "white",
                borderRadius: 999,
                height: 3,
                flex: 1,
                width: "100%",
                transform: [{ translateY: 14 }],
              }}
            />
            <Slider.Thumb
              style={{
                position: "absolute",
                width: 16,
                height: 16,
                borderRadius: 8,
                backgroundColor: "white",
                boxShadow: "0 2px 10px rgba(0, 0, 0, 0.2)",
                transform: [{ translateX: -8 }, { translateY: 7 }],
              }}
            />
          </Slider.Track>
        </Slider.Root>
      </Animated.View>
    </View>
  );
}
