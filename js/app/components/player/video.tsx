import {
  IngestMediaSource,
  PlayerProtocol,
  PlayerStatus,
  usePlayerStore,
  useStreamplaceStore,
} from "@streamplace/components";
import streamKey from "components/live-dashboard/stream-key";
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
import { Text, View } from "tamagui";
import { srcToUrl } from "./shared";
import useWebRTC, { useWebRTCIngest } from "./use-webrtc";
import {
  logWebRTCDiagnostics,
  useWebRTCDiagnostics,
} from "./webrtc-diagnostics";
import { checkWebRTCSupport } from "./webrtc-primitives";

type VideoProps = { url: string };

export default function WebVideo() {
  const inProto = usePlayerStore((x) => x.protocol);
  const isIngesting = usePlayerStore((x) => x.ingestConnectionState !== null);
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const src = usePlayerStore((x) => x.src);
  const { url, protocol } = srcToUrl({ src: src, selectedRendition }, inProto);
  if (isIngesting) {
    return <WebcamIngestPlayer url={url} />;
  }
  if (protocol === PlayerProtocol.PROGRESSIVE_MP4) {
    return <ProgressiveMP4Player url={url} />;
  } else if (protocol === PlayerProtocol.PROGRESSIVE_WEBM) {
    return <ProgressiveWebMPlayer url={url} />;
  } else if (protocol === PlayerProtocol.HLS) {
    return <HLSPlayer url={url} />;
  } else if (protocol === PlayerProtocol.WEBRTC) {
    return <WebRTCPlayer url={url} />;
  } else {
    throw new Error(`unknown playback protocol ${inProto}`);
  }
}

const updateEvents = {
  playing: true,
  waiting: true,
  stalled: true,
  pause: true,
  suspend: true,
  mute: true,
};

const VideoElement = forwardRef(
  (props: VideoProps, refCallback: ForwardedRef<HTMLVideoElement | null>) => {
    const x = usePlayerStore((x) => x);
    const url = useStreamplaceStore((x) => x.url);
    const playerEvent = usePlayerStore((x) => x.playerEvent);
    const setMuted = usePlayerStore((x) => x.setMuted);
    const setMuteWasForced = usePlayerStore((x) => x.setMuteWasForced);
    const muted = usePlayerStore((x) => x.muted);
    const ingest = usePlayerStore((x) => x.ingestConnectionState !== null);
    const volume = usePlayerStore((x) => x.volume);
    const setStatus = usePlayerStore((x) => x.setStatus);
    const setUserInteraction = usePlayerStore((x) => x.setUserInteraction);

    const event = (evType) => (e) => {
      console.log(evType);
      const now = new Date();
      if (updateEvents[evType]) {
        x.setStatus(evType);
      }
      console.log("Sending", evType, "status to", url);
      playerEvent(url, now.toISOString(), evType, {});
    };
    const [firstAttempt, setFirstAttempt] = useState(true);

    const localVideoRef = useRef<HTMLVideoElement | null>(null);

    // setPipAction comes from Zustand store
    useEffect(() => {
      if (typeof x.setPipAction === "function") {
        const fn = () => {
          if (localVideoRef.current) {
            try {
              localVideoRef.current.requestPictureInPicture?.();
            } catch (err) {
              console.error("Error requesting Picture-in-Picture:", err);
            }
          } else {
            console.log("No video ref available for PiP");
          }
        };
        x.setPipAction(fn);
      }
      // Cleanup on unmount
      return () => {
        if (typeof x.setPipAction === "function") {
          x.setPipAction(undefined);
        }
      };
    }, []);

    // Memoized callback ref for video element
    const handleVideoRef = useCallback(
      (videoElement: HTMLVideoElement | null) => {
        localVideoRef.current = videoElement;
        if (typeof refCallback === "function") {
          refCallback(videoElement);
        } else if (refCallback && "current" in refCallback) {
          refCallback.current = videoElement;
        }
      },
      [refCallback],
    );

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
              setMuted(true);
              localVideoRef.current.muted = true;
              localVideoRef.current
                .play()
                .then(() => {
                  console.warn("Browser forced video to start muted");
                  setMuteWasForced(true);
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
        setStatus(PlayerStatus.START);
      };
    }, [setStatus]);

    useEffect(() => {
      if (localVideoRef.current) {
        localVideoRef.current.volume = volume;
        console.log("Setting volume to", volume);
      }
    }, [volume]);

    return (
      <View
        backgroundColor="#111"
        alignItems="stretch"
        f={1}
        onPointerMove={setUserInteraction}
      >
        <video
          autoPlay={true}
          playsInline={true}
          ref={handleVideoRef}
          controls={false}
          src={ingest ? undefined : props.url}
          muted={muted}
          crossOrigin="anonymous"
          onMouseMove={setUserInteraction}
          onClick={setUserInteraction}
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
            transform: ingest ? "scaleX(-1)" : undefined,
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

  const videoRef = usePlayerStore((x) => x.videoRef);
  const setVideoRef = usePlayerStore((x) => x.setVideoRef);

  const refCallback = useCallback((node: HTMLVideoElement | null) => {
    localRef.current = node;
    if (typeof videoRef === "function") {
      videoRef(node);
    } else if (videoRef) {
      localRef.current = node;
      setVideoRef(localRef);
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
  }, [props.url]);
  return <VideoElement {...props} ref={refCallback} />;
}

export function WebRTCPlayer(props: VideoProps) {
  const [webrtcError, setWebrtcError] = useState<string | null>(null);
  const setStatus = usePlayerStore((x) => x.setStatus);
  const setProtocol = usePlayerStore((x) => x.setProtocol);
  const diagnostics = useWebRTCDiagnostics();
  // Check WebRTC compatibility on component mount
  useEffect(() => {
    try {
      checkWebRTCSupport();
      console.log("WebRTC Player - Browser compatibility check passed");
      logWebRTCDiagnostics();
    } catch (error) {
      console.error("WebRTC Player - Compatibility error:", error.message);
      setWebrtcError(error.message);
      setStatus(PlayerStatus.START);
      return;
    }
  }, []);

  // Monitor diagnostics for errors
  useEffect(() => {
    if (!diagnostics.browserSupport && diagnostics.errors.length > 0) {
      setWebrtcError(diagnostics.errors.join(", "));
    }
  }, [diagnostics]);

  if (!diagnostics.done) return <></>;

  if (webrtcError) {
    setProtocol(PlayerProtocol.HLS);
    return (
      <View
        backgroundColor="#111"
        alignItems="center"
        justifyContent="center"
        f={1}
        padding="$4"
      >
        <View
          backgroundColor="$red10"
          padding="$3"
          borderRadius="$4"
          maxWidth={400}
        >
          <View marginBottom="$2">
            <Text fontSize="$8" fontWeight="bold" color="white">
              WebRTC Not Supported
            </Text>
          </View>
          <Text fontSize="$4" color="white" lineHeight="$1" marginBottom="$3">
            {webrtcError}
          </Text>
          {diagnostics.errors.length > 0 && (
            <View>
              <Text
                fontSize="$4"
                fontWeight="bold"
                color="white"
                marginBottom="$2"
              >
                Technical Details:
              </Text>
              {diagnostics.errors.map((error, index) => (
                <Text key={index} fontSize="$3" color="white" marginBottom="$1">
                  • {error}
                </Text>
              ))}
            </View>
          )}
          <Text fontSize="$3">
            • To use WebRTC, you may need to disable any blocking extensions or
            update your browser.
          </Text>
          <Text mt="$2">Switching to HLS...</Text>
        </View>
      </View>
    );
  }
  return <WebRTCPlayerInner url={props.url} />;
}

export function WebRTCPlayerInner({ url }: { url: string }) {
  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const [connectionStatus, setConnectionStatus] =
    useState<string>("initializing");

  const localVideoRef = useRef<HTMLVideoElement | null>(null);

  const videoRef = usePlayerStore((x) => x.videoRef);
  const setVideoRef = usePlayerStore((x) => x.setVideoRef);

  const status = usePlayerStore((x) => x.status);
  const setStatus = usePlayerStore((x) => x.setStatus);

  const playerEvent = usePlayerStore((x) => x.playerEvent);
  const spurl = useStreamplaceStore((x) => x.url);

  const handleRef = useCallback((node: HTMLVideoElement | null) => {
    if (node) {
      setVideoElement(node);
    }
    if (typeof videoRef === "function") {
      videoRef(node);
    } else if (setVideoRef) {
      setVideoRef(localVideoRef);
    }
  }, []);

  const [mediaStream, stuck] = useWebRTC(url);

  // Debug logging for WebRTC connection state
  useEffect(() => {
    // Update connection status based on state
    if (stuck) {
      setConnectionStatus("connection-failed");
    } else if (mediaStream) {
      setConnectionStatus("connected");
    } else {
      setConnectionStatus("connecting");
    }
  }, [url, mediaStream, stuck, status]);

  useEffect(() => {
    if (stuck && status === PlayerStatus.PLAYING) {
      setStatus(PlayerStatus.STALLED);
    }
    if (!stuck && mediaStream) {
      setStatus(PlayerStatus.PLAYING);
    }
  }, [stuck, status, mediaStream]);

  useEffect(() => {
    if (!mediaStream) {
      return;
    }
    const evt = (evType) => (e) => {
      console.log("webrtc event", evType);
      playerEvent(spurl, new Date().toISOString(), evType, {});
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

  // Test not working right now
  // useEffect(() => {
  //   if (!props.avSyncTest) {
  //     return;
  //   }
  //   if (!mediaStream) {
  //     return;
  //   }
  //   quietReceiver(mediaStream, playerEvent);
  // }, [mediaStream, props.avSyncTest, playerEvent]);

  useEffect(() => {
    if (!videoElement) {
      return;
    }
    videoElement.srcObject = mediaStream;
  }, [videoElement, mediaStream]);

  // Show loading/connection status when no media stream is available
  if (!mediaStream) {
    return (
      <View
        backgroundColor="#111"
        alignItems="center"
        justifyContent="center"
        f={1}
        padding="$4"
      >
        <View
          backgroundColor="$blue10"
          padding="$3"
          borderRadius="$4"
          maxWidth={400}
        >
          <View marginBottom="$2">
            <Text fontSize="$6" fontWeight="bold" color="white">
              Connecting...
            </Text>
          </View>
          <Text fontSize="$4" color="white" lineHeight="$1">
            Establishing WebRTC connection ({connectionStatus})
          </Text>
        </View>
      </View>
    );
  }
  return <VideoElement url={url} ref={handleRef} />;
}

export function WebcamIngestPlayer(props: VideoProps) {
  const ingestStarting = usePlayerStore((x) => x.ingestStarting);
  const ingestMediaSource = usePlayerStore((x) => x.ingestMediaSource);
  const ingestAutoStart = usePlayerStore((x) => x.ingestAutoStart);

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
  // we assign a stream key in the webrtcingest hook
  const [remoteMediaStream, setRemoteMediaStream] = useWebRTCIngest({
    endpoint: `${url}/api/ingest/webrtc`,
  });

  useEffect(() => {
    if (ingestMediaSource === IngestMediaSource.DISPLAY) {
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
  }, [ingestMediaSource]);

  useEffect(() => {
    if (!ingestStarting && !ingestAutoStart) {
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
  }, [localMediaStream, ingestStarting, streamKey, ingestAutoStart]);

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
