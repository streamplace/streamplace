import { useEffect, useRef } from "react";
import { View as RNView } from "react-native";
import {
  DanmuOverlay,
  getFirstPlayerID,
  useDanmuEnabled,
  useDanmuLaneCount,
  useDanmuMaxMessages,
  useDanmuOpacity,
  useDanmuSpeed,
  usePlayerStore,
} from "../..";
import { View } from "../../components/ui";
import { AudioOnlyOverlay } from "./ui/audio-only-overlay";
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

  const danmuEnabled = useDanmuEnabled();
  const danmuOpacity = useDanmuOpacity();
  const danmuSpeed = useDanmuSpeed();
  const danmuLaneCount = useDanmuLaneCount();
  const danmuMaxMessages = useDanmuMaxMessages();

  const divRef = useRef<RNView>(null);
  const videoRef = useRef<HTMLVideoElement | null>(null);
  // iPhone Safari has no Element.requestFullscreen, only the video
  // element's own webkitEnterFullscreen; the store holds that element.
  const storeVideoRef = usePlayerStore((x) => x.videoRef, playerId);
  const videoElement = (): HTMLVideoElement | null => {
    if (videoRef.current) return videoRef.current;
    if (storeVideoRef && typeof storeVideoRef !== "function") {
      return storeVideoRef.current ?? null;
    }
    return null;
  };

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
          } else if (videoElement()) {
            const v = videoElement() as any;
            if (typeof v.webkitEnterFullscreen === "function") {
              await v.webkitEnterFullscreen();
            } else if (typeof v.requestFullscreen === "function") {
              await v.requestFullscreen();
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
      <AudioOnlyOverlay />
      <DanmuOverlay
        enabled={danmuEnabled}
        opacity={danmuOpacity}
        speed={danmuSpeed}
        laneCount={danmuLaneCount}
        maxMessages={danmuMaxMessages}
      />
      {props.children}
    </View>
  );
}
