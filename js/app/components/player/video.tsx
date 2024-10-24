import React, {
  ForwardedRef,
  forwardRef,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useTransition,
} from "react";
import { Button, Text, View, XStack } from "tamagui";
import WHEPClient from "./webrtc";
import Hls from "hls.js";
import { Circle, CheckCircle } from "@tamagui/lucide-icons";
import useAquareumNode from "hooks/useAquareumNode";
import Controls from "./controls";
import {
  PlayerProps,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
  PROTOCOL_PROGRESSIVE_WEBM,
} from "./props";
import { srcToUrl } from "./shared";

type VideoProps = PlayerProps & { url: string };

export default function WebVideo(props: PlayerProps) {
  const { url, protocol } = srcToUrl(props);
  if (protocol === PROTOCOL_PROGRESSIVE_MP4) {
    return <ProgressiveMP4Player url={url} {...props} />;
  } else if (protocol === PROTOCOL_PROGRESSIVE_WEBM) {
    return <ProgressiveWebMPlayer url={url} {...props} />;
  } else if (protocol === PROTOCOL_HLS) {
    return <HLSPlayer url={url} {...props} />;
  } else {
    throw new Error(`unknown playback protocol ${props.protocol}`);
  }
}

const POLL_INTERVAL = 5000;
const updateEvents = {
  playing: true,
  waiting: true,
  stalled: true,
};

const VideoElement = forwardRef(
  (props: VideoProps, ref: ForwardedRef<HTMLVideoElement>) => {
    const event = (evType) => (e) => {
      const now = new Date();
      if (updateEvents[evType]) {
        props.setStatus(evType);
      }
      props.playerEvent(now.toISOString(), evType, {});
    };

    return (
      <View
        backgroundColor="#111"
        alignItems="stretch"
        f={1}
        onPointerMove={props.userInteraction}
      >
        <video
          autoPlay={true}
          ref={ref}
          loop={true}
          controls={false}
          src={props.url}
          muted={props.muted}
          crossOrigin="anonymous"
          onMouseMove={props.userInteraction}
          onClick={props.userInteraction}
          onAbort={event("abort")}
          onCanPlay={event("canplay")}
          onCanPlayThrough={event("canplaythrough")}
          // onDurationChange={event("durationchange")}
          onEmptied={event("emptied")}
          onEncrypted={event("encrypted")}
          onEnded={event("ended")}
          onError={event("error")}
          onLoadedData={event("loadeddata")}
          onLoadedMetadata={event("loadedmetadata")}
          onLoadStart={event("loadstart")}
          onPause={event("pause")}
          onPlay={event("play")}
          onPlaying={event("playing")}
          // onProgress={event("progress")}
          // onTimeUpdate={event("timeupdate")}
          onRateChange={event("ratechange")}
          onSeeked={event("seeked")}
          onSeeking={event("seeking")}
          onStalled={event("stalled")}
          onSuspend={event("suspend")}
          onVolumeChange={event("volumechange")}
          onWaiting={event("waiting")}
          style={{
            objectFit: "contain",
            backgroundColor: "transparent",
            width: "100%",
            height: "100%",
          }}
        />
      </View>
    );
  },
);

export function ProgressiveMP4Player(props: VideoProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  return <VideoElement {...props} ref={videoRef} />;
}

export function ProgressiveWebMPlayer(props: VideoProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  return <VideoElement {...props} ref={videoRef} />;
}

export function HLSPlayer(props: VideoProps) {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  useEffect(() => {
    if (!videoRef.current) {
      return;
    }
    if (Hls.isSupported()) {
      var hls = new Hls();
      hls.loadSource(props.url);
      try {
        hls.attachMedia(videoRef.current);
      } catch (e) {
        console.error("error on attachMedia");
        hls.stopLoad();
        return;
      }
      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        if (!videoRef.current) {
          return;
        }
        videoRef.current.play();
      });
      return () => {
        hls.stopLoad();
      };
    } else if (videoRef.current.canPlayType("application/vnd.apple.mpegurl")) {
      videoRef.current.src = props.url;
      videoRef.current.addEventListener("canplay", () => {
        if (!videoRef.current) {
          return;
        }
        videoRef.current.play();
      });
    }
  }, [videoRef.current]);
  return <VideoElement {...props} ref={videoRef} />;
}

// export function WebRTCPlayer(props: { src: string }) {
//   const videoRef = useRef<HTMLVideoElement | null>(null);
//   const { url } = useAquareumNode();
//   useEffect(() => {
//     if (!videoRef.current) {
//       return;
//     }
//     const client = new WHEPClient(
//       `${url}/api/webrtc/${props.src}`,
//       videoRef.current,
//     );
//     return () => {
//       client.close();
//     };
//   }, [videoRef.current]);
//   return <VideoElement ref={videoRef} />;
// }
