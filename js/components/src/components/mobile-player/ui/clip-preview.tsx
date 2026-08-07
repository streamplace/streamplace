import { Play } from "lucide-react-native";
import { useEffect, useRef, useState } from "react";
import { Pressable } from "react-native";
import { View } from "../../ui";

// Web clip preview: a raw <video> element, matching the main player (video.tsx
// renders plain <video> on web — there's no expo-video-on-web precedent). The
// range-looping clamp lives here because the player store doesn't know about
// clip trims.
export function ClipPreview({
  uri,
  trimStart,
  trimEnd,
  seekTo,
  onTimeUpdate,
}: {
  uri: string;
  /** Trim window in ms — the video element reports currentTime in seconds. */
  trimStart: number;
  trimEnd: number;
  /** External seek request (e.g. a timeline tap), in ms. */
  seekTo?: number;
  onTimeUpdate?: (ms: number) => void;
}) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const [playing, setPlaying] = useState(false);

  // Loop playback inside the selected range. The store's trims are in ms while
  // <video>.currentTime is in seconds — convert at the boundary.
  const clampToRange = () => {
    const el = videoRef.current;
    if (!el) return;
    if (el.currentTime * 1000 > trimEnd || el.currentTime * 1000 < trimStart) {
      el.currentTime = trimStart / 1000;
    }
  };

  // Jump to the requested position (trim change or timeline tap).
  useEffect(() => {
    const el = videoRef.current;
    if (!el) return;
    el.currentTime = (seekTo ?? trimStart) / 1000;
  }, [seekTo, trimStart, uri]);

  // Autoplay when the editor opens. Browsers may block unmuted autoplay until
  // the user interacts — the play overlay invites the tap in that case.
  useEffect(() => {
    const el = videoRef.current;
    if (!el) return;
    el.play().catch(() => {});
  }, [uri]);

  const togglePlay = () => {
    const el = videoRef.current;
    if (!el) return;
    if (el.paused) el.play().catch(() => {});
    else el.pause();
  };

  return (
    <Pressable
      onPress={togglePlay}
      style={{
        flex: 1,
        backgroundColor: "#000",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <video
        ref={(node) => {
          videoRef.current = node;
        }}
        src={uri}
        crossOrigin="anonymous"
        playsInline
        // Loop the whole file; the timeupdate clamp below keeps playback
        // inside [trimStart, trimEnd] for trimmed selections.
        loop
        style={{ width: "100%", height: "100%", objectFit: "contain" }}
        onTimeUpdate={(e) => {
          const el = e.target as HTMLVideoElement;
          clampToRange();
          onTimeUpdate?.(el.currentTime * 1000);
        }}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
      />
      {!playing && (
        <View
          style={{
            position: "absolute",
            width: 56,
            height: 56,
            borderRadius: 28,
            backgroundColor: "rgba(0,0,0,0.6)",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Play size={24} color="#fff" fill="#fff" />
        </View>
      )}
    </Pressable>
  );
}
