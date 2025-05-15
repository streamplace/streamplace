import { useCallback, useEffect, useRef } from "react";
import { TamaguiElement, View } from "tamagui";
import Controls from "./controls";
import PlayerLoading from "./player-loading";
import { PlayerProps } from "./props";
import Video from "./video";
import VideoRetry from "./video-retry";

export default function Fullscreen(props: PlayerProps) {
  const divRef = useRef<TamaguiElement>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const videoCallback = useCallback((node: HTMLVideoElement | null) => {
    videoRef.current = node;
    if (typeof props.videoRef === "function") {
      props.videoRef(node);
    } else if (props.videoRef) {
      props.videoRef.current = node;
    }
  }, []);

  const setFullscreen = (on: boolean) => {
    if (!divRef.current) {
      return;
    }
    (async () => {
      if (on && !document.fullscreenElement) {
        try {
          const div = divRef.current as HTMLDivElement;
          if (typeof div.requestFullscreen === "function") {
            await div.requestFullscreen();
          } else if (videoRef.current) {
            if (
              typeof (videoRef.current as any).webkitEnterFullscreen ===
              "function"
            ) {
              await (videoRef.current as any).webkitEnterFullscreen();
            } else if (
              typeof videoRef.current.requestFullscreen === "function"
            ) {
              await videoRef.current.requestFullscreen();
            }
          }
          props.setFullscreen(true);
        } catch (e) {
          console.error("fullscreen failed", e.message);
        }
      }
      if (!on) {
        if (document.fullscreenElement) {
          try {
            await document.exitFullscreen();
          } catch (e) {
            console.error("fullscreen exit failed", e.message);
          }
        }
        props.setFullscreen(false);
      }
    })();
  };

  useEffect(() => {
    const listener = () => {
      console.log("fullscreenchange", document.fullscreenElement);
      props.setFullscreen(!!document.fullscreenElement);
    };
    document.body.addEventListener("fullscreenchange", listener);
    document.body.addEventListener("webkitfullscreenchange", listener);
    return () => {
      document.body.removeEventListener("fullscreenchange", listener);
      document.body.removeEventListener("webkitfullscreenchange", listener);
    };
  }, []);

  return (
    <View flex={1} ref={divRef}>
      <PlayerLoading {...props}></PlayerLoading>
      <Controls {...props} setFullscreen={setFullscreen} videoRef={videoRef} />
      <VideoRetry {...props}>
        <Video {...props} videoRef={videoCallback} />
      </VideoRetry>
    </View>
  );
}
