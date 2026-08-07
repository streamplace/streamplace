import { useEffect, useMemo, useState } from "react";
import { LayoutChangeEvent } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, {
  runOnJS,
  useAnimatedStyle,
  useSharedValue,
} from "react-native-reanimated";
import { useClipStore } from "../../../clip-store";
import {
  dragZone,
  MIN_CLIP_MS,
  moveWindow,
  msToPx,
  pxToMs,
  resizeWindow,
  type DragZone,
} from "../../../clip-store/trim-math";
import { Text, useTheme, View } from "../../ui";

const HANDLE_WIDTH = 12;
const TRACK_HEIGHT = 44;

function formatMs(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000));
  const m = Math.floor(total / 60);
  const s = total % 60;
  return `${m}:${String(s).padStart(2, "0")}`;
}

export function TrimTimeline({
  onSeek,
  playheadMs = 0,
}: {
  /** Called when the track is tapped, with the tapped position in ms. */
  onSeek?: (ms: number) => void;
  /** Current preview position in ms (drives the playhead marker). */
  playheadMs?: number;
}) {
  const th = useTheme();
  const durationMs = useClipStore((s) => s.durationMs);
  const trimStart = useClipStore((s) => s.trimStart);
  const trimEnd = useClipStore((s) => s.trimEnd);
  const setTrim = useClipStore((s) => s.setTrim);

  const [trackWidth, setTrackWidth] = useState(0);

  // Shared values drive the window visuals during a drag so the gesture never
  // re-renders React per frame; the store only sees the committed trim on
  // gesture end.
  const trackWidthSV = useSharedValue(0);
  const startPx = useSharedValue(0);
  const endPx = useSharedValue(0);
  const startMs = useSharedValue(0);
  const endMs = useSharedValue(0);
  const playheadPx = useSharedValue(0);
  const zone = useSharedValue<DragZone>("none");
  const originStartMs = useSharedValue(0);
  const originEndMs = useSharedValue(0);

  // Keep the animated window in sync with committed store values (initial
  // render, and after a gesture commits via setTrim).
  useEffect(() => {
    if (trackWidth <= 0 || durationMs <= 0) return;
    startPx.value = msToPx(trimStart, durationMs, trackWidth);
    endPx.value = msToPx(trimEnd, durationMs, trackWidth);
    startMs.value = trimStart;
    endMs.value = trimEnd;
  }, [
    trimStart,
    trimEnd,
    durationMs,
    trackWidth,
    startPx,
    endPx,
    startMs,
    endMs,
  ]);

  useEffect(() => {
    if (trackWidth <= 0 || durationMs <= 0) return;
    playheadPx.value = msToPx(playheadMs, durationMs, trackWidth);
  }, [playheadMs, trackWidth, durationMs, playheadPx]);

  const onLayout = (e: LayoutChangeEvent) => {
    const w = e.nativeEvent.layout.width;
    setTrackWidth(w);
    trackWidthSV.value = w;
  };

  const pan = useMemo(
    () =>
      Gesture.Pan()
        // Capture which zone the gesture grabbed at touch-down; the decision
        // lives in a shared value so the update worklet can read it.
        .onBegin((e) => {
          const w = trackWidthSV.value;
          if (w <= 0 || durationMs <= 0) {
            zone.value = "none";
            return;
          }
          zone.value = dragZone(e.x, startPx.value, endPx.value, HANDLE_WIDTH);
          originStartMs.value = startMs.value;
          originEndMs.value = endMs.value;
        })
        .onUpdate((e) => {
          const w = trackWidthSV.value;
          if (w <= 0 || durationMs <= 0) return;
          const z = zone.value;
          if (z === "left") {
            const res = resizeWindow(
              "left",
              startMs.value,
              endMs.value,
              pxToMs(e.x, durationMs, w),
              durationMs,
              MIN_CLIP_MS,
            );
            startMs.value = res.start;
            endMs.value = res.end;
          } else if (z === "right") {
            const res = resizeWindow(
              "right",
              startMs.value,
              endMs.value,
              pxToMs(e.x, durationMs, w),
              durationMs,
              MIN_CLIP_MS,
            );
            startMs.value = res.start;
            endMs.value = res.end;
          } else if (z === "body") {
            const delta = pxToMs(e.translationX, durationMs, w);
            const res = moveWindow(
              originStartMs.value,
              originEndMs.value,
              delta,
              durationMs,
              MIN_CLIP_MS,
            );
            startMs.value = res.start;
            endMs.value = res.end;
          }
          // zone "none": dragging the empty track is a no-op by design.
          startPx.value = msToPx(startMs.value, durationMs, w);
          endPx.value = msToPx(endMs.value, durationMs, w);
        })
        .onEnd(() => {
          runOnJS(setTrim)(startMs.value, endMs.value);
        }),
    [
      durationMs,
      trackWidthSV,
      startPx,
      endPx,
      startMs,
      endMs,
      zone,
      originStartMs,
      originEndMs,
      setTrim,
    ],
  );

  const tap = useMemo(
    () =>
      Gesture.Tap().onEnd((e) => {
        if (!onSeek) return;
        const w = trackWidthSV.value;
        if (w <= 0 || durationMs <= 0) return;
        const ms = Math.min(
          Math.max(0, pxToMs(e.x, durationMs, w)),
          durationMs,
        );
        runOnJS(onSeek)(ms);
      }),
    [onSeek, durationMs, trackWidthSV],
  );

  const gesture = useMemo(() => Gesture.Race(pan, tap), [pan, tap]);

  const windowStyle = useAnimatedStyle(() => ({
    left: startPx.value,
    width: Math.max(0, endPx.value - startPx.value),
  }));

  const leftHandleStyle = useAnimatedStyle(() => ({
    left: startPx.value - HANDLE_WIDTH / 2,
  }));

  const rightHandleStyle = useAnimatedStyle(() => ({
    left: endPx.value - HANDLE_WIDTH / 2,
  }));

  const playheadStyle = useAnimatedStyle(() => ({
    left: playheadPx.value - 1,
  }));

  return (
    <View style={{ gap: 6 }}>
      <View style={{ flexDirection: "row", justifyContent: "space-between" }}>
        <Text size="xs" style={{ color: th.theme.colors.mutedForeground }}>
          {formatMs(trimStart)}
        </Text>
        <Text size="xs" style={{ color: th.theme.colors.mutedForeground }}>
          {formatMs(trimEnd - trimStart)}
        </Text>
        <Text size="xs" style={{ color: th.theme.colors.mutedForeground }}>
          {formatMs(trimEnd)}
        </Text>
      </View>
      <GestureDetector gesture={gesture}>
        <View
          onLayout={onLayout}
          style={{ height: TRACK_HEIGHT, justifyContent: "center" }}
        >
          {/* Track */}
          <View
            style={{
              height: 6,
              borderRadius: 3,
              backgroundColor: th.theme.colors.border,
              overflow: "hidden",
            }}
          />
          {/* Selection window */}
          <Animated.View
            style={[
              {
                position: "absolute",
                top: TRACK_HEIGHT / 2 - 9,
                height: 18,
                borderRadius: 4,
                backgroundColor: th.theme.colors.primary,
                opacity: 0.85,
              },
              windowStyle,
            ]}
          />
          {/* Edge handles */}
          <Animated.View
            style={[
              {
                position: "absolute",
                top: TRACK_HEIGHT / 2 - 12,
                width: HANDLE_WIDTH,
                height: 24,
                borderRadius: 4,
                backgroundColor: "#fff",
                borderWidth: 1,
                borderColor: th.theme.colors.primary,
              },
              leftHandleStyle,
            ]}
          />
          <Animated.View
            style={[
              {
                position: "absolute",
                top: TRACK_HEIGHT / 2 - 12,
                width: HANDLE_WIDTH,
                height: 24,
                borderRadius: 4,
                backgroundColor: "#fff",
                borderWidth: 1,
                borderColor: th.theme.colors.primary,
              },
              rightHandleStyle,
            ]}
          />
          {/* Playhead */}
          <Animated.View
            style={[
              {
                position: "absolute",
                top: 0,
                bottom: 0,
                width: 2,
                backgroundColor: "#fff",
              },
              playheadStyle,
            ]}
          />
        </View>
      </GestureDetector>
    </View>
  );
}
