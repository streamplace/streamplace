import { useCallback, useRef, useState } from "react";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { usePlayerStore } from "../../../player-store";
import { Slider } from "../../ui";
import { Text, View } from "../../ui/index";

function formatTime(seconds: number): string {
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => String(n).padStart(2, "0");
  if (h > 0) return `${h}:${pad(m)}:${pad(sec)}`;
  return `${m}:${pad(sec)}`;
}

export function SeekBar() {
  const mode = usePlayerStore((x) => x.mode);
  const playTime = usePlayerStore((x) => x.playTime);
  const duration = usePlayerStore((x) => x.duration);
  const bufferedEnd = usePlayerStore((x) => x.bufferedEnd);
  const seekTo = usePlayerStore((x) => x.seekTo);

  const seekingRef = useRef(false);
  const [seekValue, setSeekValue] = useState(0);

  const thumbHovered = useSharedValue(0);
  const thumbAnimStyle = useAnimatedStyle(() => ({
    boxShadow: `0 2px 10px rgba(0, 0, 0, ${withSpring(thumbHovered.value === 1 ? 1 : 0.18)})`,
    transform: [
      { scaleY: withSpring(thumbHovered.value === 1 ? 1 : 0.18) },
      {
        scaleX: withSpring(thumbHovered.value === 1 ? 1 : 0.35),
      },
    ],
  }));

  const handleValueChange = useCallback((vals: number[]) => {
    seekingRef.current = true;
    setSeekValue(vals[0]);
  }, []);

  const handlePointerUp = useCallback(() => {
    if (seekingRef.current) {
      seekTo(seekValue);
      seekingRef.current = false;
    }
  }, [seekValue, seekTo]);

  if (mode !== "vod" || duration <= 0) return null;

  const displayTime = seekingRef.current ? seekValue : playTime;

  return (
    <View
      style={{
        width: "100%",
        paddingHorizontal: 16,
        paddingVertical: 8,
        gap: 4,
        flexDirection: "column",
      }}
    >
      <View
        style={{
          flex: 1,
          height: 90,
          paddingBottom: 24,
        }}
      >
        <Slider.Root
          style={{
            position: "relative",
            display: "flex",
            alignItems: "center",
            flex: 1,
            height: 20,
          }}
          value={displayTime}
          min={0}
          max={duration}
          onValueChange={handleValueChange}
          asChild
        >
          <Slider.Track
            onPointerUp={handlePointerUp}
            style={{
              flexGrow: 1,
              height: 30,
              position: "relative",
              flex: 1,
            }}
          >
            <View
              style={{
                position: "absolute",
                height: 32,
                width: "100%",
                zIndex: 1,
              }}
              // @ts-ignore — web-only pointer events
              onPointerEnter={() => {
                thumbHovered.value = 1;
              }}
              // @ts-ignore — web-only pointer events
              onPointerLeave={() => {
                thumbHovered.value = 0;
              }}
            />
            {/* full track background */}
            <View
              style={{
                position: "absolute",
                backgroundColor: "rgba(255, 255, 255, 0.15)",
                borderRadius: 999,
                height: 3,
                width: "100%",
                transform: [{ translateY: 14 }],
              }}
            />
            {/* buffered range */}
            <View
              style={{
                position: "absolute",
                backgroundColor: "rgba(255, 255, 255, 0.35)",
                borderRadius: 999,
                height: 3,
                left: 0,
                width: `${Math.min((bufferedEnd / duration) * 100, 100)}%`,
                transform: [{ translateY: 14 }],
              }}
            />
            <Slider.Range
              style={{
                position: "absolute",
                height: 32,
              }}
            >
              <View
                style={{
                  backgroundColor: "rgba(255, 255, 255, 0.75)",
                  borderRadius: 999,
                  maxHeight: 3,
                  flex: 1,
                  transform: [{ translateY: 14 }],
                }}
              />
            </Slider.Range>
            <Slider.Thumb
              onPointerUp={handlePointerUp}
              style={{
                position: "absolute",
                width: 16,
                height: 16,
                transform: [{ translateX: -10 }, { translateY: 7.5 }],
              }}
            >
              <Animated.View
                style={[
                  {
                    width: 16,
                    height: 16,
                    borderRadius: 8,
                    backgroundColor: "white",
                  },
                  thumbAnimStyle,
                ]}
              />
            </Slider.Thumb>
          </Slider.Track>
        </Slider.Root>
      </View>
      <View
        style={{
          flexDirection: "row",
          justifyContent: "space-between",
        }}
      >
        <Text size="xs" style={{ color: "#ccc" }}>
          {formatTime(displayTime)}
        </Text>
        <Text size="xs" style={{ color: "#ccc" }}>
          {formatTime(duration)}
        </Text>
      </View>
    </View>
  );
}
