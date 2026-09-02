import { ChevronUp } from "lucide-react-native";
import { ComponentProps, useEffect, useState } from "react";
import { Dimensions } from "react-native";
import {
  Gesture,
  GestureDetector,
  Pressable,
} from "react-native-gesture-handler";
import Animated, {
  Easing,
  Extrapolation,
  interpolate,
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
  withTiming,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useKeyboardSlide } from "../../hooks";
import { bottom, h, layout, p, w, zIndex } from "../../lib/theme/atoms";
import { colors, motion, scrims, textAlphas } from "../../lib/theme/tokens";
import { View } from "./view";

const AnimatedView = Animated.createAnimatedComponent(View);

const { height: SCREEN_HEIGHT } = Dimensions.get("window");

const TIMING_CONFIG = {
  duration: motion.slow,
  easing: Easing.bezier(...motion.bezier),
};

type ResizableChatSheetProps = {
  startingPercentage?: number;
  isPlayerRatioGreater: boolean;
  style?: ComponentProps<typeof AnimatedView>["style"];
  children?: React.ReactNode;
  renderAbove?: (isCollapsed: boolean) => React.ReactNode;
};

export function Resizable({
  startingPercentage,
  isPlayerRatioGreater,
  style = {},
  children,
  renderAbove,
}: ResizableChatSheetProps) {
  const { slideKeyboard } = useKeyboardSlide();
  const { bottom: safeBottom } = useSafeAreaInsets();
  const MAX_HEIGHT = (SCREEN_HEIGHT - safeBottom) * 0.55;
  const MIN_HEIGHT = -(SCREEN_HEIGHT - safeBottom) * 0.2;
  const COLLAPSE_HEIGHT = (SCREEN_HEIGHT - safeBottom) * 0.1;

  const sheetHeight = useSharedValue(MIN_HEIGHT);
  const startHeight = useSharedValue(MIN_HEIGHT);
  const [isCollapsed, setIsCollapsed] = useState(true);
  const wasCollapsed = useSharedValue(true);

  useEffect(() => {
    setTimeout(() => {
      const targetHeight = startingPercentage
        ? startingPercentage * SCREEN_HEIGHT
        : MIN_HEIGHT;
      sheetHeight.value = withTiming(targetHeight, TIMING_CONFIG);
      setIsCollapsed(targetHeight < COLLAPSE_HEIGHT);
    }, 1000);
  }, []);

  const panGesture = Gesture.Pan()
    .onStart(() => {
      startHeight.value = sheetHeight.value;
    })
    .onUpdate((event) => {
      let newHeight = startHeight.value - event.translationY;
      if (newHeight > MAX_HEIGHT) newHeight = MAX_HEIGHT;
      if (newHeight < MIN_HEIGHT) newHeight = MIN_HEIGHT;
      sheetHeight.value = newHeight;

      const nowCollapsed = newHeight < COLLAPSE_HEIGHT;
      if (nowCollapsed && !wasCollapsed.value) {
        sheetHeight.value = withTiming(MIN_HEIGHT, TIMING_CONFIG);
        wasCollapsed.value = true;
        runOnJS(setIsCollapsed)(true);
      } else if (!nowCollapsed && wasCollapsed.value) {
        wasCollapsed.value = false;
        runOnJS(setIsCollapsed)(false);
      }
    });

  const animatedStyle = useAnimatedStyle(() => ({
    height: sheetHeight.value < COLLAPSE_HEIGHT ? 0 : sheetHeight.value,
    opacity: interpolate(
      sheetHeight.value,
      [MIN_HEIGHT, COLLAPSE_HEIGHT],
      [0, 1],
      Extrapolation.CLAMP,
    ),
    transform: [
      {
        translateY:
          slideKeyboard +
          Math.max(0, -sheetHeight.value) +
          (slideKeyboard < 0 ? 0 : -safeBottom) -
          (Math.abs(slideKeyboard) > 1 ? 32 : 16),
      },
    ],
  }));

  const handleAnimatedStyle = useAnimatedStyle(() => ({
    opacity: sheetHeight.value < COLLAPSE_HEIGHT ? 1 : 0,
    transform: [
      {
        translateY: sheetHeight.value < COLLAPSE_HEIGHT ? 0 : withSpring(20),
      },
    ],
  }));

  const aboveElementStyle = useAnimatedStyle(() => ({
    // show inside area when not collapsed, and show outside area when collapsed
    height: sheetHeight.value < COLLAPSE_HEIGHT ? 0 : sheetHeight.value,
    transform: [
      {
        translateY:
          sheetHeight.value < COLLAPSE_HEIGHT
            ? withSpring(-120)
            : withSpring(20),
      },
    ],
  }));

  return (
    <>
      <Animated.View
        style={[
          handleAnimatedStyle,
          layout.position.absolute,
          bottom[4],
          w.percent[100],
          layout.flex.center,
          zIndex[1],
        ]}
      >
        <Pressable
          onPress={() => {
            const isCurrentlyCollapsed = sheetHeight.value === MIN_HEIGHT;
            sheetHeight.value = isCurrentlyCollapsed
              ? withTiming(MAX_HEIGHT, TIMING_CONFIG)
              : withTiming(MIN_HEIGHT, TIMING_CONFIG);
            setIsCollapsed(!isCurrentlyCollapsed);
          }}
        >
          <View
            style={[
              p[1],
              {
                borderRadius: 999,
                backgroundColor: scrims.dark,
                overflow: "hidden",
              },
            ]}
          >
            <ChevronUp
              size={32}
              color={colors.white}
              style={{ marginBottom: 1, marginTop: -1 }}
            />
          </View>
        </Pressable>
      </Animated.View>
      <AnimatedView
        style={[
          animatedStyle,
          isPlayerRatioGreater
            ? layout.position.relative
            : layout.position.absolute,
          bottom[0],
          zIndex[1],
          w.percent[100],
          {
            backgroundColor: scrims.dark,
            overflow: "visible",
            borderTopLeftRadius: 16,
            borderTopRightRadius: 16,
            minWidth: "100%",
          },
          style,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.justifyCenter, h[2]]}>
          <View style={{ alignItems: "center", width: "100%" }}>
            <GestureDetector gesture={panGesture}>
              <View
                // Make the touch area much larger, but keep the visible handle small
                style={{
                  height: 30, // Large touch area
                  width: 120, // Wide enough for thumbs
                  alignItems: "center",
                  justifyContent: "center",
                  //backgroundColor: "rgba(0,255,255,0.1)",
                  transform: [{ translateY: -30 }],
                }}
              >
                <View
                  style={[
                    w[32],
                    {
                      height: 6,
                      backgroundColor: textAlphas.dark[3],
                      borderRadius: 999,

                      transform: [{ translateY: 5 }],
                    },
                  ]}
                />
              </View>
            </GestureDetector>
          </View>
        </View>

        {children}
      </AnimatedView>
      <Animated.View
        style={[
          aboveElementStyle,
          {
            width: "100%",
            pointerEvents: "none",
            position: "absolute",
            bottom: 0,
          },
        ]}
      >
        <View
          style={{
            pointerEvents: "auto",
            width: "100%",
            // hate doing it this way, but can't figure out
            // how to make it size to content otherwise
            minHeight: 50,
            height: "100%",
            maxHeight: 75,
            flex: 0,
          }}
        >
          {renderAbove?.(isCollapsed)}
        </View>
      </Animated.View>
    </>
  );
}

Resizable.displayName = "ResizableChatSheet";
