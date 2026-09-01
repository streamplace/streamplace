import { useVideoPlayer, VideoView } from "expo-video";
import { Play } from "lucide-react-native";
import { useEffect, useState } from "react";
import { Pressable } from "react-native";
import { View } from "../../ui";

// Native clip preview: expo-video VideoView (the same library the main native
// player uses), with native controls hidden. The range-looping clamp lives
// here because the player store doesn't know about clip trims.
export function ClipPreview({
  uri,
  trimStart,
  trimEnd,
  seekTo,
  onTimeUpdate,
}: {
  uri: string;
  /** Trim window in ms — expo-video reports currentTime in seconds. */
  trimStart: number;
  trimEnd: number;
  /** External seek request (e.g. a timeline tap), in ms. */
  seekTo?: number;
  onTimeUpdate?: (ms: number) => void;
}) {
  const [playing, setPlaying] = useState(false);
  const player = useVideoPlayer({ uri }, (player) => {
    // Loop the whole file; the timeUpdate clamp below keeps playback inside
    // [trimStart, trimEnd] for trimmed selections.
    player.loop = true;
    player.muted = false;
    player.play();
  });

  // Loop playback inside the selected range. The store's trims are in ms while
  // expo-video's currentTime is in seconds — convert at the boundary.
  useEffect(() => {
    const sub = player.addListener("timeUpdate", ({ currentTime }) => {
      if (currentTime * 1000 > trimEnd || currentTime * 1000 < trimStart) {
        player.currentTime = trimStart / 1000;
      }
      onTimeUpdate?.(currentTime * 1000);
    });
    return () => sub.remove();
  }, [player, trimStart, trimEnd, onTimeUpdate]);

  // Jump to the requested position (trim change or timeline tap).
  useEffect(() => {
    player.currentTime = (seekTo ?? trimStart) / 1000;
  }, [player, seekTo, trimStart, uri]);

  useEffect(() => {
    const sub = player.addListener("playingChange", ({ isPlaying }) =>
      setPlaying(isPlaying),
    );
    return () => sub.remove();
  }, [player]);

  const togglePlay = () => {
    if (player.playing) player.pause();
    else player.play();
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
      <VideoView
        player={player}
        nativeControls={false}
        style={{ width: "100%", height: "100%" }}
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
