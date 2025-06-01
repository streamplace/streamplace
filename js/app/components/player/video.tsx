import streamKey from "components/live-dashboard/stream-key";
import { selectStoredKey } from "features/bluesky/blueskySlice";
import { usePlayer, usePlayerProtocol } from "features/player/playerSlice";
import Hls from "hls.js";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import {
  ForwardedRef,
  forwardRef,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { View } from "tamagui";
import { quietReceiver } from "./av-sync";
import {
  IngestMediaSource,
  PlayerProps,
  PlayerStatus,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
  PROTOCOL_PROGRESSIVE_WEBM,
  PROTOCOL_WEBRTC,
} from "./props";
import { srcToUrl } from "./shared";
import useWebRTC, { useWebRTCIngest } from "./use-webrtc";

type VideoProps = PlayerProps & { url: string };

export default function WebVideo(props: PlayerProps) {
  const inProto = useAppSelector(usePlayerProtocol());
  const { url, protocol } = srcToUrl(props, inProto);
  if (props.ingest) {
    return <WebcamIngestPlayer url={url} {...props} />;
  }
  if (protocol === PROTOCOL_PROGRESSIVE_MP4) {
    return <ProgressiveMP4Player url={url} {...props} />;
  } else if (protocol === PROTOCOL_PROGRESSIVE_WEBM) {
    return <ProgressiveWebMPlayer url={url} {...props} />;
  } else if (protocol === PROTOCOL_HLS) {
    return <HLSPlayer url={url} {...props} />;
  } else if (protocol === PROTOCOL_WEBRTC) {
    return <WebRTCPlayer url={url} {...props} />;
  } else {
    throw new Error(`unknown playback protocol ${inProto}`);
  }
}

const POLL_INTERVAL = 5000;
const updateEvents = {
  playing: true,
  waiting: true,
  stalled: true,
  pause: true,
  suspend: true,
  mute: true,
};

const VideoElement = forwardRef(
  (props: VideoProps, ref: ForwardedRef<HTMLVideoElement | null>) => {
    const event = (evType) => (e) => {
      console.log(evType);
      const now = new Date();
      if (updateEvents[evType]) {
        props.setStatus(evType);
      }
      props.playerEvent(now.toISOString(), evType, {});
    };
    const [firstAttempt, setFirstAttempt] = useState(true);

    const localVideoRef = useRef<HTMLVideoElement | null>(null);

    // attempts to autoplay the video. if that fails, it attempts
    // to play the video muted; some browsers will only let you
    // autoplay if you're muted
    const canPlayThrough = (e) => {
      event("canplaythrough")(e);
      if (firstAttempt && localVideoRef.current) {
        setFirstAttempt(false);
        localVideoRef.current.play().catch((err) => {
          if (err.name === "NotAllowedError") {
            if (localVideoRef.current) {
              props.setMuted(true);
              localVideoRef.current.muted = true;
              localVideoRef.current
                .play()
                .then(() => {
                  console.log("muted video");
                  props.setMuteWasForced(true);
                })
                .catch((err) => {
                  console.error("error playing video", err);
                });
            }
          }
        });
      }
    };

    useEffect(() => {
      return () => {
        props.setStatus(PlayerStatus.START);
      };
    }, []);

    useEffect(() => {
      if (localVideoRef.current) {
        localVideoRef.current.volume = props.volume;
      }
    }, [props.volume]);

    // Use a callback ref to handle when the video element is mounted
    const handleVideoRef = (videoElement: HTMLVideoElement | null) => {
      if (videoElement && typeof ref === "function") {
        ref(videoElement);
      } else if (videoElement && ref && "current" in ref) {
        ref.current = videoElement;
      }

      // Additional initialization can be done here when the video element is first mounted
      if (videoElement) {
        localVideoRef.current = videoElement;
      }
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
          playsInline={true}
          ref={handleVideoRef}
          controls={false}
          src={props.ingest ? undefined : props.url}
          muted={props.muted}
          crossOrigin="anonymous"
          onMouseMove={props.userInteraction}
          onClick={props.userInteraction}
          onAbort={event("abort")}
          onCanPlay={event("canplay")}
          onCanPlayThrough={canPlayThrough}
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
            transform: props.ingest ? "scaleX(-1)" : undefined,
          }}
        />
      </View>
    );
  },
);

export function ProgressiveMP4Player(props: VideoProps) {
  return <VideoElement {...props} />;
}

export function ProgressiveWebMPlayer(props: VideoProps) {
  return <VideoElement {...props} />;
}

export function HLSPlayer(props: VideoProps) {
  const localRef = useRef<HTMLVideoElement | null>(null);
  const refCallback = useCallback((node: HTMLVideoElement | null) => {
    localRef.current = node;
    if (typeof props.videoRef === "function") {
      props.videoRef(node);
    } else if (props.videoRef) {
      (
        props.videoRef as React.MutableRefObject<HTMLVideoElement | null>
      ).current = node;
    }
  }, []);
  useEffect(() => {
    if (!localRef.current) {
      return;
    }
    if (Hls.isSupported()) {
      // workaround for not having quite the right number of audio frames :(
      var hls = new Hls({ maxAudioFramesDrift: 20 });
      hls.loadSource(props.url);
      try {
        hls.attachMedia(localRef.current);
      } catch (e) {
        console.error("error on attachMedia");
        hls.stopLoad();
        return;
      }
      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        if (!localRef.current) {
          return;
        }
        localRef.current.play();
      });
      return () => {
        hls.stopLoad();
      };
    } else if (localRef.current.canPlayType("application/vnd.apple.mpegurl")) {
      localRef.current.src = props.url;
      localRef.current.addEventListener("canplay", () => {
        if (!localRef.current) {
          return;
        }
        localRef.current.play();
      });
    }
  }, [localRef.current]);
  return <VideoElement {...props} ref={refCallback} />;
}

export function WebRTCPlayer(props: VideoProps) {
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );

  const handleRef = useCallback((node: HTMLVideoElement | null) => {
    if (node) {
      setVideoElement(node);
    }
    if (typeof props.videoRef === "function") {
      props.videoRef(node);
    } else if (props.videoRef) {
      props.videoRef.current = node;
    }
  }, []);

  const [mediaStream, stuck] = useWebRTC(props.url);

  useEffect(() => {
    if (stuck && props.status === PlayerStatus.PLAYING) {
      props.setStatus(PlayerStatus.STALLED);
    }
    if (!stuck) {
      props.setStatus(PlayerStatus.PLAYING);
    }
  }, [stuck, props.status]);

  useEffect(() => {
    if (!mediaStream) {
      return;
    }
    const evt = (evType) => (e) => {
      console.log("webrtc event", evType);
      props.playerEvent(new Date().toISOString(), evType, {});
    };
    const active = evt("active");
    const inactive = evt("inactive");
    const ended = evt("ended");
    const mute = evt("mute");
    const unmute = evt("playing"); // playing has resumed yay

    mediaStream.addEventListener("active", active);
    mediaStream.addEventListener("inactive", inactive);
    mediaStream.addEventListener("ended", ended);
    for (const track of mediaStream.getTracks()) {
      track.addEventListener("ended", ended);
      track.addEventListener("mute", mute);
      track.addEventListener("unmute", unmute);
    }
    return () => {
      for (const track of mediaStream.getTracks()) {
        track.removeEventListener("ended", ended);
        track.removeEventListener("mute", mute);
        track.removeEventListener("unmute", unmute);
      }
      mediaStream.removeEventListener("active", active);
      mediaStream.removeEventListener("inactive", inactive);
      mediaStream.removeEventListener("ended", ended);
    };
  }, [mediaStream]);

  useEffect(() => {
    if (!props.avSyncTest) {
      return;
    }
    if (!mediaStream) {
      return;
    }
    quietReceiver(mediaStream, props.playerEvent);
  }, [props.avSyncTest, mediaStream]);

  useEffect(() => {
    if (!videoElement) {
      return;
    }
    videoElement.srcObject = mediaStream;
  }, [videoElement, mediaStream]);

  return <VideoElement {...props} ref={handleRef} />;
}

export function WebcamIngestPlayer(props: VideoProps) {
  const dispatch = useAppDispatch();
  const player = useAppSelector(usePlayer());
  const storedKey = useAppSelector(selectStoredKey);
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const handleRef = useCallback((node: HTMLVideoElement | null) => {
    if (node) {
      setVideoElement(node);
    }
  }, []);

  const { url } = useStreamplaceNode();
  const [localMediaStream, setLocalMediaStream] = useState<MediaStream | null>(
    null,
  );
  const [remoteMediaStream, setRemoteMediaStream] = useWebRTCIngest({
    endpoint: `${url}/api/ingest/webrtc`,
    streamKey: props.ingestStreamKey,
  });

  useEffect(() => {
    if (props.ingestMediaSource === IngestMediaSource.DISPLAY) {
      navigator.mediaDevices
        .getDisplayMedia({
          audio: true,
          video: true,
        })
        .then((stream) => {
          setLocalMediaStream(stream);
        })
        .catch((e) => {
          console.error("error getting display media", e);
        });
    } else {
      navigator.mediaDevices
        .getUserMedia({
          audio: true,
          video: {
            width: { min: 200, ideal: 1920, max: 3840 },
            height: { min: 200, ideal: 1080, max: 2160 },
          },
        })
        .then((stream) => {
          setLocalMediaStream(stream);
        })
        .catch((e) => {
          console.error("error getting user media", e);
        });
    }
  }, [props.ingestMediaSource]);

  useEffect(() => {
    if (!player.ingestStarting && !props.ingestAutoStart) {
      setRemoteMediaStream(null);
      return;
    }
    if (!localMediaStream) {
      return;
    }
    if (!streamKey) {
      return;
    }
    setRemoteMediaStream(localMediaStream);
  }, [
    localMediaStream,
    player.ingestStarting,
    streamKey,
    props.ingestAutoStart,
  ]);

  useEffect(() => {
    if (!videoElement) {
      return;
    }
    if (!localMediaStream) {
      return;
    }
    videoElement.srcObject = localMediaStream;
  }, [videoElement, localMediaStream]);

  return <VideoElement {...props} ref={handleRef} />;
}
