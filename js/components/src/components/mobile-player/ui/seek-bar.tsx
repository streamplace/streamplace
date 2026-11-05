import { KeyboardEvent, useState } from "react";
import { LayoutChangeEvent, Platform, View } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import {
  borderRadius,
  colors,
  spacing,
  textAlphas,
  touchTargets,
} from "../../../lib/theme/tokens";
import { usePlayerStore } from "../../../player-store";
import { useTheme } from "../../ui";

const TRACK_HEIGHT = spacing[1];
const THUMB_SIZE = spacing[3];

// Preview a drag locally, then commit its final position to the native player.
export function SeekBar({
  onScrubbingChange,
}: {
  onScrubbingChange?: (active: boolean) => void;
}) {
  const mode = usePlayerStore((x) => x.mode);
  const playTime = usePlayerStore((x) => x.playTime);
  const duration = usePlayerStore((x) => x.duration);
  const bufferedEnd = usePlayerStore((x) => x.bufferedEnd);
  const seekTo = usePlayerStore((x) => x.seekTo);
  const { theme } = useTheme();

  const [trackWidth, setTrackWidth] = useState(0);
  const [scrubbing, setScrubbing] = useState(false);
  const [scrubTime, setScrubTime] = useState(0);

  if (mode !== "vod" || duration <= 0) {
    return null;
  }

  const timeAt = (x: number) => {
    if (trackWidth <= 0) return 0;
    const clamped = Math.max(0, Math.min(trackWidth, x));
    return (clamped / trackWidth) * duration;
  };

  const pan = Gesture.Pan()
    .runOnJS(true)
    .onBegin((e) => {
      setScrubbing(true);
      onScrubbingChange?.(true);
      setScrubTime(timeAt(e.x));
    })
    .onUpdate((e) => {
      setScrubTime(timeAt(e.x));
    })
    .onEnd((e) => {
      seekTo(timeAt(e.x));
    })
    .onFinalize(() => {
      setScrubbing(false);
      onScrubbingChange?.(false);
    });

  const tap = Gesture.Tap()
    .runOnJS(true)
    .onEnd((e) => {
      seekTo(timeAt(e.x));
      onScrubbingChange?.(false);
    });

  const gesture = Gesture.Race(pan, tap);

  const current = scrubbing ? scrubTime : playTime;
  const progressPct = Math.max(0, Math.min(1, current / duration)) * 100;
  const bufferedPct = Math.max(0, Math.min(1, bufferedEnd / duration)) * 100;

  return (
    <View style={{ paddingHorizontal: spacing[4] }}>
      <GestureDetector gesture={gesture}>
        <View
          accessible
          accessibilityRole="adjustable"
          accessibilityLabel="Playback position"
          accessibilityValue={{ min: 0, max: duration, now: current }}
          {...(Platform.OS === "web" && {
            tabIndex: 0,
            "aria-valuemin": 0,
            "aria-valuemax": duration,
            "aria-valuenow": current,
            onKeyDown: (event: KeyboardEvent) => {
              let time: number;
              switch (event.key) {
                case "ArrowLeft":
                case "ArrowDown":
                  time = playTime - 10;
                  break;
                case "ArrowRight":
                case "ArrowUp":
                  time = playTime + 10;
                  break;
                case "Home":
                  time = 0;
                  break;
                case "End":
                  time = duration;
                  break;
                default:
                  return;
              }
              event.preventDefault();
              seekTo(Math.max(0, Math.min(duration, time)));
              onScrubbingChange?.(false);
            },
          })}
          accessibilityActions={[{ name: "increment" }, { name: "decrement" }]}
          onAccessibilityAction={({ nativeEvent: { actionName } }) => {
            if (actionName !== "increment" && actionName !== "decrement")
              return;
            seekTo(
              Math.max(
                0,
                Math.min(
                  duration,
                  playTime + (actionName === "increment" ? 10 : -10),
                ),
              ),
            );
            onScrubbingChange?.(false);
          }}
          onLayout={(e: LayoutChangeEvent) =>
            setTrackWidth(e.nativeEvent.layout.width)
          }
          style={{
            height: touchTargets.minimum,
            justifyContent: "flex-end",
            paddingBottom: spacing[2],
          }}
        >
          <View
            style={{
              height: TRACK_HEIGHT,
              borderRadius: borderRadius.full,
              backgroundColor: textAlphas.dark[4],
            }}
          >
            <View
              style={{
                position: "absolute",
                top: 0,
                bottom: 0,
                left: 0,
                width: `${bufferedPct}%`,
                borderRadius: borderRadius.full,
                backgroundColor: textAlphas.dark[3],
              }}
            />
            <View
              style={{
                position: "absolute",
                top: 0,
                bottom: 0,
                left: 0,
                width: `${progressPct}%`,
                borderRadius: borderRadius.full,
                backgroundColor: theme.colors.primary,
              }}
            />
          </View>
          <View
            style={{
              position: "absolute",
              bottom: spacing[2] - (THUMB_SIZE - TRACK_HEIGHT) / 2,
              left: `${progressPct}%`,
              marginLeft: -THUMB_SIZE / 2,
              width: THUMB_SIZE,
              height: THUMB_SIZE,
              borderRadius: borderRadius.full,
              backgroundColor: colors.white,
            }}
          />
        </View>
      </GestureDetector>
    </View>
  );
}
