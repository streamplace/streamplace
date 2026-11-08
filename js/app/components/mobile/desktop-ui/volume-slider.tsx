import {
  IconButton,
  Slider,
  useMuted,
  useSetMuted,
  useSetVolume,
  useVolume,
  View,
  zero,
} from "@streamplace/components";
import {
  borderRadius,
  colors,
  motion,
  spacing,
  textAlphas,
  touchTargets,
} from "@streamplace/components/src/lib/theme/tokens";
import { Volume2, VolumeX } from "lucide-react-native";
import { useCallback } from "react";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";

const { layout } = zero;

export function VolumeSlider({ sliderWidth = 200 }: { sliderWidth?: number }) {
  const muted = useMuted();
  const setMuted = useSetMuted();
  const volume = useVolume();
  const setVolume = useSetVolume();

  const fadeAnim = useSharedValue(0);
  const widthAnim = useSharedValue(0);

  const fadeOut = useCallback(() => {
    fadeAnim.value = withTiming(0, { duration: motion.slow });
    widthAnim.value = withTiming(0, { duration: motion.slow });
  }, [fadeAnim, widthAnim]);

  const onVolumeHover = useCallback(() => {
    fadeAnim.value = withTiming(1, { duration: motion.base });
    widthAnim.value = withTiming(sliderWidth, { duration: motion.base });
  }, [fadeAnim, sliderWidth, widthAnim]);

  // Toggle mute state
  const handleMuteToggle = useCallback(() => {
    setMuted(!muted);
  }, [muted, setMuted]);

  const VolumeIcon = muted ? VolumeX : Volume2;

  const animatedStyle = useAnimatedStyle(() => ({
    opacity: fadeAnim.value,
    width: widthAnim.value,
  }));

  // Convert logarithmic volume (0-1) to linear slider value (0-100)
  const sliderValue = muted ? 0 : Math.round(Math.pow(volume, 0.5) * 100);

  return (
    <View
      onPointerEnter={onVolumeHover}
      onPointerLeave={fadeOut}
      onFocus={onVolumeHover}
      onBlur={fadeOut}
      style={[
        layout.flex.row,
        layout.flex.alignCenter,
        { height: touchTargets.minimum },
      ]}
    >
      <IconButton
        accessibilityLabel={muted ? "Unmute" : "Mute"}
        onPress={handleMuteToggle}
        style={{
          width: touchTargets.minimum,
          height: touchTargets.minimum,
        }}
      >
        <VolumeIcon size={spacing[6]} color={colors.white} />
      </IconButton>

      <Animated.View
        style={[
          { height: touchTargets.minimum, overflow: "hidden" },
          animatedStyle,
        ]}
      >
        <Slider.Root
          style={{
            position: "relative",
            display: "flex",
            flexDirection: "row",
            alignItems: "center",
            flex: 1,
            width: sliderWidth - spacing[4],
            marginHorizontal: spacing[2],
            height: touchTargets.minimum,
          }}
          value={sliderValue}
          min={0}
          max={100} // Slider max value is 100 for percentage
          onValueChange={(vals) => {
            // Convert back to logarithmic volume
            const newVolume = Math.pow(vals[0] / 100, 2);
            setVolume(newVolume);
            if (newVolume === 0) {
              setMuted(true);
            } else {
              setMuted(false);
            }
          }}
        >
          <Slider.Track
            testID="player-volume-track"
            style={{
              height: spacing[1],
              borderRadius: borderRadius.full,
              backgroundColor: textAlphas.dark[4],
              position: "relative",
              flex: 1,
            }}
          >
            <Slider.Range
              style={{
                position: "absolute",
                backgroundColor: colors.white,
                borderRadius: borderRadius.full,
                height: spacing[1],
              }}
            />
          </Slider.Track>
          <Slider.Thumb
            accessibilityLabel="Volume"
            style={{
              width: spacing[3],
              height: spacing[3],
              borderRadius: borderRadius.full,
              backgroundColor: colors.white,
            }}
          />
        </Slider.Root>
      </Animated.View>
    </View>
  );
}
