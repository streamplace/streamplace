import { memo, useEffect, useState } from "react";
import { LayoutChangeEvent, StyleSheet } from "react-native";
import Animated, {
  Easing,
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withTiming,
} from "react-native-reanimated";
import { ChatMessageViewHydrated } from "streamplace";
import { Text } from "../ui";
import { baseDuration, mapRange, MAX_DURATION, MIN_DURATION } from "./math";

interface DanmuMessageProps {
  message: ChatMessageViewHydrated;
  lane: number;
  laneHeight: number;
  videoTop: number;
  opacity: number;
  fontSize: number;
  speed: number;
  containerWidth: number;
  containerHeight: number;
  onComplete: (messageId: string) => void;
  onWidthMeasured?: (messageId: string, width: number) => void;
}

export const DanmuMessage = memo(
  ({
    message,
    lane,
    laneHeight,
    videoTop,
    opacity,
    fontSize,
    speed,
    containerWidth,
    containerHeight,
    onComplete,
    onWidthMeasured,
  }: DanmuMessageProps) => {
    const translateX = useSharedValue(containerWidth);
    const [messageWidth, setMessageWidth] = useState(0);
    const [animationStartTime, setAnimationStartTime] = useState<number | null>(
      null,
    );
    const [totalDuration, setTotalDuration] = useState(0);

    const getRgbColor = (
      color: {
        red: number;
        green: number;
        blue: number;
      } = { red: 123, green: 123, blue: 123 },
    ) => {
      const red = mapRange(color.red, 0, 255, 100, 230);
      const green = mapRange(color.green, 0, 255, 100, 230);
      const blue = mapRange(color.blue, 0, 255, 100, 230);

      return `rgb(${Math.round(red)}, ${Math.round(green)}, ${Math.round(blue)})`;
    };

    const handleLayout = (event: LayoutChangeEvent) => {
      const width = event.nativeEvent.layout.width;
      if (width > 0) {
        setMessageWidth(width);
        if (onWidthMeasured) {
          onWidthMeasured(message.uri, width);
        }
      }
    };

    useEffect(() => {
      if (messageWidth === 0) return; // Wait for layout measurement

      const duration = baseDuration(message, MAX_DURATION, MIN_DURATION);

      // Calculate how much time has elapsed if animation already started
      let remainingDuration = duration;
      let startPosition = containerWidth;

      if (animationStartTime !== null) {
        const elapsed = Date.now() - animationStartTime;
        const progress = Math.min(elapsed / totalDuration, 1);
        remainingDuration = duration * (1 - progress);
        startPosition =
          containerWidth + progress * (-messageWidth - containerWidth);
      } else {
        setAnimationStartTime(Date.now());
        setTotalDuration(duration);
      }

      if (__DEV__)
        console.log(
          `[danmu] animation started: "${message.record.text}" (duration: ${duration.toFixed(0)}ms, remaining: ${remainingDuration.toFixed(0)}ms, speed: ${speed}x)`,
        );

      translateX.value = startPosition;

      translateX.value = withTiming(
        -messageWidth,
        {
          duration: remainingDuration,
          easing: Easing.linear,
        },
        (finished) => {
          if (finished) {
            runOnJS(onComplete)(message.uri);
          }
        },
      );
    }, [
      messageWidth,
      containerWidth,
      speed,
      message.uri,
      message.record.text.length,
      lane,
      onComplete,
    ]);

    const animatedStyle = useAnimatedStyle(() => {
      return {
        transform: [{ translateX: translateX.value }],
      };
    });

    return (
      <Animated.View
        style={[
          styles.container,
          {
            top: videoTop + lane * laneHeight,
            opacity: opacity / 100,
          },
          animatedStyle,
        ]}
        onLayout={handleLayout}
      >
        <Text
          style={[
            styles.text,
            {
              fontSize,
              color: getRgbColor(message.chatProfile?.color),
            },
          ]}
          numberOfLines={1}
        >
          {message.record.text}
        </Text>
      </Animated.View>
    );
  },
  (prevProps, nextProps) => {
    return (
      prevProps.message.uri === nextProps.message.uri &&
      prevProps.lane === nextProps.lane &&
      prevProps.laneHeight === nextProps.laneHeight &&
      prevProps.videoTop === nextProps.videoTop &&
      prevProps.opacity === nextProps.opacity &&
      prevProps.fontSize === nextProps.fontSize &&
      prevProps.speed === nextProps.speed &&
      prevProps.containerWidth === nextProps.containerWidth &&
      prevProps.containerHeight === nextProps.containerHeight
    );
  },
);

const styles = StyleSheet.create({
  container: {
    position: "absolute",
    left: 0,
  },
  text: {
    color: "white",
    textShadowColor: "rgba(0, 0, 0, 0.8)",
    textShadowOffset: { width: 1, height: 1 },
    textShadowRadius: 64,
    fontWeight: "600",
  },
});
