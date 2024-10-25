import { VideoView } from "expo-video";
import { useRef } from "react";
import Controls from "./controls";
import PlayerLoading from "./player-loading";
import { PlayerProps } from "./props";
import Video from "./video.native";
import VideoRetry from "./video-retry";

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
      <VideoRetry {...props}>
        <Video {...props} videoRef={ref} />
      </VideoRetry>
    </>
  );
}
