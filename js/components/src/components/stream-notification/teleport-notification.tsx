import { useEffect, useState } from "react";
import { useWindowDimensions, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import { Button, Text, useTheme, zero } from "../../";

export function TeleportNotification({
  targetHandle,
  countdown,
  canCancel,
  startTime,
  onDismiss,
}: {
  targetHandle: string;
  countdown: number;
  canCancel: boolean;
  startTime?: number;
  onDismiss: (reason?: "user" | "auto") => void;
}) {
  const { zero: z } = useTheme();
  const w = useWindowDimensions().width;

  const [start, setStart] = useState(Date.now());
  const [now, setNow] = useState(Date.now());
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    const interval = setInterval(() => {
      setNow(Date.now());
    }, 100);
    return () => clearInterval(interval);
  }, []);

  const timeLeft = Math.max(0, countdown - Math.floor((now - start) / 1000));

  useEffect(() => {
    if (dismissed) {
      return;
    }
    if (timeLeft <= 0) {
      setDismissed(true);
      onDismiss("auto");
    }
  }, [dismissed, onDismiss, timeLeft]);

  // if we're past 5 seconds from start, stripes should already be hidden
  const elapsedTime = startTime ? (Date.now() - startTime) / 1000 : 0;
  const [showStripes, setShowStripes] = useState(elapsedTime < 5);

  const stripeX = useSharedValue(0);
  const stripeOpacity = useSharedValue(1);
  const progressWidth = useSharedValue(100);

  useEffect(() => {
    // if stripes are already hidden, fade out asap and return
    if (!showStripes) {
      stripeOpacity.value = withTiming(0, { duration: 0 });
      return;
    }
    // warning stripes animation
    stripeX.value = withRepeat(
      withTiming(30 * 2, {
        duration: 1000,
        easing: Easing.linear,
      }),
      3,
      false,
    );

    // hide stripes after 500ms
    const stripesTimer = setTimeout(() => {
      // woosh the stripes off to the right before hiding
      stripeX.value = withTiming(30 * 80, {
        duration: 1500,
        easing: Easing.cubic,
      });
      // after animation, set stripes as hidden
      setTimeout(() => {
        setShowStripes(false);
      }, 350);
    }, 1500);

    return () => clearTimeout(stripesTimer);
  }, []);

  useEffect(() => {
    if (showStripes) return;

    // animate progress bar
    const percentage = (timeLeft / countdown) * 100;
    progressWidth.value = withTiming(percentage, {
      duration: 1000,
      easing: Easing.linear,
    });
  }, [timeLeft, countdown, showStripes]);

  const stripesStyle = useAnimatedStyle(() => ({
    opacity: stripeOpacity.value,
    transform: [{ translateX: stripeX.value }],
  }));

  const progressStyle = useAnimatedStyle(() => ({
    width: `${progressWidth.value}%`,
  }));

  return (
    <View style={[{ overflow: "hidden" }, zero.r.lg, zero.bg.neutral[900]]}>
      <View
        style={[
          zero.layout.flex.row,
          zero.layout.flex.alignCenter,
          zero.layout.flex.spaceBetween,
          zero.px[3],
          w > 650 ? zero.py[4] : zero.py[2],
        ]}
      >
        <Text size={w > 650 ? "xl" : "base"}>
          Teleporting to @{targetHandle}
        </Text>
        <View
          style={[
            zero.layout.flex.row,
            zero.layout.flex.alignCenter,
            zero.gap.all[3],
          ]}
        >
          <Text color="muted">{timeLeft}s</Text>
          {canCancel && (
            <Button
              onPress={() => onDismiss("user")}
              width="min"
              variant="danger"
            >
              Cancel
            </Button>
          )}
        </View>
      </View>
      <View
        style={{
          height: 4,
          width: "100%",
          borderRadius: 2,
          overflow: "hidden",
          backgroundColor: "#0f0f1e", // token-ok: teleport effect color
        }}
      >
        <Animated.View
          style={[
            { height: "100%", borderRadius: 2, backgroundColor: "#16f4d0" }, // token-ok: teleport effect color
            progressStyle,
          ]}
        />
      </View>
      <Animated.View
        style={[
          {
            position: "absolute",
            flexDirection: "row",
            height: 180,
            width: "200%",
            //clickthrough
            pointerEvents: "none",
          },
          stripesStyle,
        ]}
      >
        {[...Array(80)].map((_, i) => (
          <View
            key={i}
            style={{
              width: 30,
              height: "100%",
              backgroundColor: i % 2 === 0 ? "#FFA500" : "#000000", // token-ok: teleport effect color
              transform: [{ skewX: "-45deg" }, { translateX: -30 * 8 }],
            }}
          />
        ))}
      </Animated.View>
    </View>
  );
}
