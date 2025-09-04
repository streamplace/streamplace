import { useEffect, useRef } from "react";
import { View as RNView } from "react-native";
import { getFirstPlayerID, usePlayerStore } from "../..";
import { View } from "../../components/ui";
import Video from "./video";
import VideoRetry from "./video-retry";

export function Fullscreen(props: {
  src: string;
  children?: React.ReactNode;
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
}) {
  const playerId = getFirstPlayerID();
  const protocol = usePlayerStore((x) => x.protocol, playerId);
  const fullscreen = usePlayerStore((x) => x.fullscreen, playerId);
  const setFullscreen = usePlayerStore((x) => x.setFullscreen, playerId);
  const setSrc = usePlayerStore((x) => x.setSrc);
  const setAutoplayFailed = usePlayerStore((x) => x.setAutoplayFailed);

  const divRef = useRef<RNView>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    setSrc(props.src);
    setAutoplayFailed(false);
  }, [props.src]);

  useEffect(() => {
    if (!divRef.current) {
      return;
    }
    (async () => {
      if (fullscreen && !document.fullscreenElement) {
        try {
          const div = divRef.current as unknown as HTMLDivElement;
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
          setFullscreen(true);
        } catch (e) {
          console.error("fullscreen failed", e.message);
        }
      }
      if (!fullscreen) {
        if (document.fullscreenElement) {
          try {
            await document.exitFullscreen();
          } catch (e) {
            console.error("fullscreen exit failed", e.message);
          }
        }
        setFullscreen(false);
      }
    })();
  }, [fullscreen, protocol]);

  useEffect(() => {
    const listener = () => {
      console.log("fullscreenchange", document.fullscreenElement);
      setFullscreen(!!document.fullscreenElement);
    };
    document.body.addEventListener("fullscreenchange", listener);
    document.body.addEventListener("webkitfullscreenchange", listener);
    return () => {
      document.body.removeEventListener("fullscreenchange", listener);
      document.body.removeEventListener("webkitfullscreenchange", listener);
    };
  }, []);

  return (
    <View
      ref={divRef}
      style={{ width: "100%", height: "100%", overflow: "hidden" }}
    >
      <VideoRetry>
        <Video
          objectFit={props.objectFit}
          pictureInPictureEnabled={props.pictureInPictureEnabled}
        />
      </VideoRetry>
      {props.children}
    </View>
  );
}
