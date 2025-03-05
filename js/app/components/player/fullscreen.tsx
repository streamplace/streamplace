import { useEffect, useRef } from "react";
import { TamaguiElement, View } from "tamagui";
import Controls from "./controls";
import PlayerLoading from "./player-loading";
import { PlayerProps } from "./props";
import Video from "./video";
import VideoRetry from "./video-retry";

declare global {
  interface HTMLVideoElement {
    webkitEnterFullscreen?: () => Promise<void>;
  }
}

/**
 * Returns true if the current device is an iOS device.
 *
 * source: https://stackoverflow.com/a/62094756/2311366
 * license: CC BY-SA 4.0
 */
const isIOS = () => {
  const iosQuirkPresent = () => {
    const audio = new Audio();
    audio.volume = 0.5;
    return audio.volume === 1; // volume cannot be changed from "1" on iOS 12 and below
  };

  const isIOS = /iPad|iPhone|iPod/.test(navigator.userAgent);
  const isAppleDevice = navigator.userAgent.includes("Macintosh");
  const isTouchScreen = navigator.maxTouchPoints >= 1; // true for iOS 13 (and hopefully beyond)
  return isIOS || (isAppleDevice && (isTouchScreen || iosQuirkPresent()));
};

export default function Fullscreen(props: PlayerProps) {
  const divRef = useRef<HTMLElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null!);

  const setFullscreen = (on: boolean) => {
    (async () => {
      if (on) {
        if (isIOS()) {
          // in iOS, we need to request fullscreen on the video element
          await videoRef.current.webkitEnterFullscreen?.();
        } else {
          // try to fullscreen the div first, then fallback to the video element
          if (divRef.current) {
            try {
              await divRef.current.requestFullscreen();
            } catch {
              await videoRef.current.requestFullscreen();
            }
          } else {
            await videoRef.current.requestFullscreen();
          }
        }
      } else {
        if (props.fullscreen) {
          try {
            await document.exitFullscreen();
          } catch (error) {
            console.error("fullscreen exit failed", error.message);
          }
        }
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
      <Controls {...props} setFullscreen={setFullscreen} />
      <VideoRetry {...props}>
        <Video {...props} videoRef={videoRef} />
      </VideoRetry>
    </View>
  );
}
