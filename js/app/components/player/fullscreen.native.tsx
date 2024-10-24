import Video from "./video.native";
import Controls from "./controls";
import { VideoView } from "expo-video";
import { useRef } from "react";
import { PlayerProps } from "./props";
import PlayerLoading from "./player-loading";

export default function Fullscreen(props: PlayerProps) {
  const ref = useRef<VideoView>(null);
  const setFullscreen = (on) => {
    if (!ref.current) {
      return;
    }
    if (on) {
      ref.current.enterFullscreen();
    } else {
      ref.current.exitFullscreen();
    }
  };
  return (
    <>
      <PlayerLoading {...props}></PlayerLoading>
      <Controls {...props} setFullscreen={setFullscreen} />
      <Video {...props} videoRef={ref} />
    </>
  );
}
