import { ChevronUp } from "lucide-react-native";
import { ComponentProps, useEffect } from "react";
import { Dimensions } from "react-native";
import {
  Gesture,
  GestureDetector,
  Pressable,
} from "react-native-gesture-handler";
import Animated, {
  Extrapolation,
  interpolate,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useKeyboardSlide } from "../../hooks";
import { bottom, h, layout, p, w, zIndex } from "../../lib/theme/atoms";
import { View } from "./view";

const AnimatedView = Animated.createAnimatedComponent(View);

const { height: SCREEN_HEIGHT } = Dimensions.get("window");

type ResizableChatSheetProps = {
  startingPercentage?: number;
  isPlayerRatioGreater: boolean;
  style?: ComponentProps<typeof AnimatedView>["style"];
  children?: React.ReactNode;
};

const SPRING_CONFIG = { damping: 20, stiffness: 100 };

export function Resizable({
  startingPercentage,
  isPlayerRatioGreater,
  style = {},
  children,
}: ResizableChatSheetProps) {
  const { slideKeyboard } = useKeyboardSlide();
  const { bottom: safeBottom } = useSafeAreaInsets();
  const MAX_HEIGHT = (SCREEN_HEIGHT - safeBottom) * 0.5;
  const MIN_HEIGHT = -(SCREEN_HEIGHT - safeBottom) * 0.2;
  const COLLAPSE_HEIGHT = (SCREEN_HEIGHT - safeBottom) * 0.1;

  const sheetHeight = useSharedValue(MIN_HEIGHT);
  const startHeight = useSharedValue(MIN_HEIGHT);

  useEffect(() => {
    setTimeout(() => {
      sheetHeight.value = withSpring(
        startingPercentage ? startingPercentage * SCREEN_HEIGHT : MIN_HEIGHT,
        SPRING_CONFIG,
      );
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

      if (newHeight < COLLAPSE_HEIGHT) {
        sheetHeight.value = withSpring(MIN_HEIGHT, SPRING_CONFIG);
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
          (slideKeyboard < 0 ? 0 : -safeBottom),
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
            sheetHeight.value =
              sheetHeight.value === MIN_HEIGHT
                ? withSpring(MAX_HEIGHT, SPRING_CONFIG)
                : withSpring(MIN_HEIGHT, SPRING_CONFIG);
          }}
        >
          <View
            style={[
              p[1],
              {
                borderRadius: 999,
                backgroundColor: "rgba(0, 0, 0, 0.5)",
                overflow: "hidden",
              },
            ]}
          >
            <ChevronUp
              size={32}
              color="white"
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
            backgroundColor: "rgba(0, 0, 0, 0.5)",
            overflow: "visible",
            borderTopLeftRadius: 16,
            borderTopRightRadius: 16,
            minWidth: "100%",
          },
          style,
        ]}
      >
        <View style={[layout.flex.row, layout.flex.justifyCenter, h[2]]}>
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
                    backgroundColor: "#eeeeee66",
                    borderRadius: 999,

                    transform: [{ translateY: 5 }],
                  },
                ]}
              />
            </View>
          </GestureDetector>
        </View>

        {children}
      </AnimatedView>
    </>
  );
}

Resizable.displayName = "ResizableChatSheet";
