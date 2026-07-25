import { useState } from "react";
import { LayoutChangeEvent, View } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import { usePlayerStore } from "../../../player-store";

const TRACK_HEIGHT = 3;
const THUMB_SIZE = 14;
const TOUCH_HEIGHT = 16;

// Native VOD scrub bar. The old implementation leaned on @rn-primitives/slider
// driven by web pointer events (onPointerUp/onPointerEnter), which never fire
// on Android — so it didn't scrub and its tall internal layout floated the bar
// toward the middle. This is a plain gesture-handler Pan/Tap over a measured
// track: drag updates a local scrub position, release commits via seekTo (which
// drives the native player through the store).
export function SeekBar() {
  const mode = usePlayerStore((x) => x.mode);
  const playTime = usePlayerStore((x) => x.playTime);
  const duration = usePlayerStore((x) => x.duration);
  const bufferedEnd = usePlayerStore((x) => x.bufferedEnd);
  const seekTo = usePlayerStore((x) => x.seekTo);

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
    });

  const tap = Gesture.Tap()
    .runOnJS(true)
    .onEnd((e) => {
      seekTo(timeAt(e.x));
    });

  const gesture = Gesture.Race(pan, tap);

  const current = scrubbing ? scrubTime : playTime;
  const progressPct = Math.max(0, Math.min(1, current / duration)) * 100;
  const bufferedPct = Math.max(0, Math.min(1, bufferedEnd / duration)) * 100;

  return (
    <View style={{ paddingHorizontal: 16, paddingTop: 2, marginBottom: -2 }}>
      <GestureDetector gesture={gesture}>
        <View
          onLayout={(e: LayoutChangeEvent) =>
            setTrackWidth(e.nativeEvent.layout.width)
          }
          style={{ height: TOUCH_HEIGHT, justifyContent: "center" }}
        >
          <View
            style={{
              height: TRACK_HEIGHT,
              borderRadius: 999,
              backgroundColor: "rgba(255,255,255,0.25)",
            }}
          >
            <View
              style={{
                position: "absolute",
                top: 0,
                bottom: 0,
                left: 0,
                width: `${bufferedPct}%`,
                borderRadius: 999,
                backgroundColor: "rgba(255,255,255,0.4)",
              }}
            />
            <View
              style={{
                position: "absolute",
                top: 0,
                bottom: 0,
                left: 0,
                width: `${progressPct}%`,
                borderRadius: 999,
                backgroundColor: "#fff",
              }}
            />
          </View>
          <View
            style={{
              position: "absolute",
              left: `${progressPct}%`,
              marginLeft: -THUMB_SIZE / 2,
              width: THUMB_SIZE,
              height: THUMB_SIZE,
              borderRadius: THUMB_SIZE / 2,
              backgroundColor: "#fff",
            }}
          />
        </View>
      </GestureDetector>
    </View>
  );
}
