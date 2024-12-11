import Hls from "hls.js";
import {
  ForwardedRef,
  forwardRef,
  RefObject,
  useEffect,
  useState,
  useCallback,
} from "react";
import { View } from "tamagui";
import {
  PlayerProps,
  PlayerStatus,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
  PROTOCOL_PROGRESSIVE_WEBM,
  PROTOCOL_WEBRTC,
} from "./props";
import { srcToUrl } from "./shared";
import useWebRTC from "./use-webrtc";

type VideoProps = PlayerProps & { url: string };

export default function WebVideo(
  props: PlayerProps & { videoRef: RefObject<HTMLVideoElement> },
) {
  const { url, protocol } = srcToUrl(props);
  useEffect(() => {
    if (props.playTime == 0) {
      return;
    }
    if (props.videoRef.current) {
      props.videoRef.current.play();
    }
  }, [props.playTime]);
  if (protocol === PROTOCOL_PROGRESSIVE_MP4) {
    return <ProgressiveMP4Player url={url} {...props} />;
  } else if (protocol === PROTOCOL_PROGRESSIVE_WEBM) {
    return <ProgressiveWebMPlayer url={url} {...props} />;
  } else if (protocol === PROTOCOL_HLS) {
    return <HLSPlayer url={url} {...props} />;
  } else if (protocol === PROTOCOL_WEBRTC) {
    return <WebRTCPlayer url={url} {...props} />;
  } else {
    throw new Error(`unknown playback protocol ${props.protocol}`);
  }
}

const POLL_INTERVAL = 5000;
const updateEvents = {
  playing: true,
  waiting: true,
  stalled: true,
  pause: true,
  suspend: true,
};

const VideoElement = forwardRef(
  (props: VideoProps, ref: ForwardedRef<HTMLVideoElement>) => {
    const event = (evType) => (e) => {
      console.log(evType);
      const now = new Date();
      if (updateEvents[evType]) {
        props.setStatus(evType);
      }
      props.playerEvent(now.toISOString(), evType, {});
    };

    useEffect(() => {
      console.log("video mounted");
      return () => {
        props.setStatus(PlayerStatus.START);
      };
    }, []);

    return (
      <View
        backgroundColor="#111"
        alignItems="stretch"
        f={1}
        onPointerMove={props.userInteraction}
      >
        <video
          autoPlay={true}
          playsInline={true}
          ref={ref}
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

export function ProgressiveMP4Player(
  props: VideoProps & { videoRef: RefObject<HTMLVideoElement> },
) {
  return <VideoElement {...props} ref={props.videoRef} />;
}

export function ProgressiveWebMPlayer(
  props: VideoProps & { videoRef: RefObject<HTMLVideoElement> },
) {
  return <VideoElement {...props} ref={props.videoRef} />;
}

export function HLSPlayer(
  props: VideoProps & { videoRef: RefObject<HTMLVideoElement> },
) {
  const videoRef = props.videoRef;
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

export function WebRTCPlayer(
  props: VideoProps & { videoRef: RefObject<HTMLVideoElement> },
) {
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );

  const handleRef = useCallback((node: HTMLVideoElement | null) => {
    if (node) {
      setVideoElement(node);
    }
  }, []);

  const [mediaStream] = useWebRTC(props.url);

  useEffect(() => {
    if (!videoElement) {
      return;
    }
    videoElement.srcObject = mediaStream;
  }, [videoElement, mediaStream]);

  return <VideoElement {...props} ref={handleRef} />;
}
