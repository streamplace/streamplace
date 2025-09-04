import Hls from "hls.js";
import { forwardRef, useCallback, useEffect, useRef, useState } from "react";
import {
  IngestMediaSource,
  PlayerProtocol,
  PlayerStatus,
  usePlayerStore,
  useStreamplaceStore,
} from "../..";
import { borderRadius, colors, mt } from "../../lib/theme/atoms";
import { Text, View } from "../ui/index";
import { Loader } from "../ui/loader";
import { srcToUrl } from "./shared";
import useWebRTC, { useWebRTCIngest } from "./use-webrtc";
import {
  logWebRTCDiagnostics,
  useWebRTCDiagnostics,
} from "./webrtc-diagnostics";
import { checkWebRTCSupport } from "./webrtc-primitives";

function assignVideoRef(
  ref:
    | React.MutableRefObject<HTMLVideoElement | null>
    | ((instance: HTMLVideoElement | null) => void)
    | null
    | undefined,
  instance: HTMLVideoElement | null,
) {
  if (!ref) return;
  if (typeof ref === "function") ref(instance);
  else ref.current = instance;
}

type VideoProps = {
  url: string;
  videoRef?: React.RefObject<HTMLVideoElement>;
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
};

function useVideoDimensions(videoRef: React.RefObject<HTMLVideoElement>) {
  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });

  useEffect(() => {
    if (!videoRef.current) return;

    function updateSize() {
      setDimensions({
        width: videoRef.current?.videoWidth || 0,
        height: videoRef.current?.videoHeight || 0,
      });
    }

    updateSize();

    const observer = new window.ResizeObserver(updateSize);
    observer.observe(videoRef.current);

    videoRef.current.addEventListener("loadedmetadata", updateSize);
    videoRef.current.addEventListener("resize", updateSize);

    return () => {
      observer.disconnect();
      videoRef.current?.removeEventListener("loadedmetadata", updateSize);
      videoRef.current?.removeEventListener("resize", updateSize);
    };
  }, [videoRef, videoRef.current]);

  return dimensions;
}

export default function WebVideo(props?: {
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
}) {
  const inProto = usePlayerStore((x) => x.protocol);
  const isIngesting = usePlayerStore((x) => x.ingestConnectionState !== null);
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const src = usePlayerStore((x) => x.src);
  const setPlayerWidth = usePlayerStore((x) => x.setPlayerWidth);
  const setPlayerHeight = usePlayerStore((x) => x.setPlayerHeight);
  const { url, protocol } = srcToUrl({ src: src, selectedRendition }, inProto);

  const videoRef = useRef<HTMLVideoElement | null>(null);
  const dimensions = useVideoDimensions(videoRef);

  useEffect(() => {
    if (videoRef.current) {
      setPlayerWidth(dimensions.width);
      setPlayerHeight(dimensions.height);
    }
  }, [dimensions, setPlayerWidth, setPlayerHeight]);

  const playerProps = {
    url,
    videoRef,
    objectFit: props?.objectFit,
    pictureInPictureEnabled: props?.pictureInPictureEnabled,
  };

  return (
    <>
      {isIngesting ? (
        <WebcamIngestPlayer {...playerProps} />
      ) : protocol === PlayerProtocol.PROGRESSIVE_MP4 ? (
        <ProgressiveMP4Player {...playerProps} />
      ) : protocol === PlayerProtocol.PROGRESSIVE_WEBM ? (
        <ProgressiveWebMPlayer {...playerProps} />
      ) : protocol === PlayerProtocol.HLS ? (
        <HLSPlayer {...playerProps} />
      ) : protocol === PlayerProtocol.WEBRTC ? (
        <WebRTCPlayer {...playerProps} />
      ) : (
        (() => {
          throw new Error(`unknown playback protocol ${inProto}`);
        })()
      )}
    </>
  );
}

const updateEvents = {
  playing: true,
  waiting: true,
  stalled: true,
  pause: true,
  suspend: true,
  mute: true,
};

const VideoElement = forwardRef<
  HTMLVideoElement,
  VideoProps & { videoRef?: React.RefObject<HTMLVideoElement> }
>((props, ref) => {
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
  const setVideoRef = usePlayerStore((x) => x.setVideoRef);

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
  const setAutoplayFailed = usePlayerStore((x) => x.setAutoplayFailed);

  const localVideoRef = props.videoRef ?? useRef<HTMLVideoElement | null>(null);

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

  const canPlayThrough = (e) => {
    console.log("canPlayThrough called", {
      firstAttempt,
      videoRef: !!localVideoRef.current,
    });
    setStatus(PlayerStatus.PLAYING);
    event("canplaythrough")(e);
    if (firstAttempt && localVideoRef.current) {
      setFirstAttempt(false);
      console.log("Attempting to play video");
      localVideoRef.current.play().catch((err) => {
        console.log("error playing video", err.name);
        if (err.name === "NotAllowedError") {
          if (localVideoRef.current) {
            console.log("Setting muted and retrying");
            setMuted(true);
            localVideoRef.current.muted = true;
            localVideoRef.current
              .play()
              .then(() => {
                console.log("Muted play succeeded");
                setMuteWasForced(true);
              })
              .catch((err) => {
                console.error("Muted play also failed", err);
                setAutoplayFailed(true);
              });
          }
        } else {
          // For other errors (not NotAllowedError), also show play button
          setAutoplayFailed(true);
        }
      });
    }
  };

  const handlePlaying = (e) => {
    setAutoplayFailed(false);
    event("playing")(e);
  };

  useEffect(() => {
    return () => {
      setStatus(PlayerStatus.START);
    };
  }, []);

  useEffect(() => {
    if (localVideoRef.current) {
      localVideoRef.current.volume = volume;
      console.log("Setting volume to", volume);
    }
  }, [volume]);

  useEffect(() => {
    console.log(localVideoRef.current?.width, localVideoRef.current?.height);
    setVideoRef(localVideoRef);
  }, [setVideoRef, localVideoRef]);

  const handleVideoRef = (videoElement: HTMLVideoElement | null) => {
    if (typeof ref === "function") {
      ref(videoElement);
    } else if (ref) {
      (ref as React.MutableRefObject<HTMLVideoElement | null>).current =
        videoElement;
    }
    (localVideoRef as any).current = videoElement;
  };

  const eventLogger = (evType) => (e) => {
    console.log("📺 Video event:", evType);
    const now = new Date();
    if (updateEvents[evType]) {
      x.setStatus(evType);
    }
    console.log("Sending", evType, "status to", url);
    playerEvent(url, now.toISOString(), evType, {});
  };

  return (
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
      onCanPlay={eventLogger}
      onCanPlayThroughCapture={eventLogger}
      onCanPlayThrough={canPlayThrough}
      onEmptied={event("emptied")}
      onEncrypted={event("encrypted")}
      onEnded={event("ended")}
      onError={event("error")}
      onLoadedData={event("loadeddata")}
      onLoadedMetadata={event("loadedmetadata")}
      onLoadStart={event("loadstart")}
      onPause={event("pause")}
      onPlay={event("play")}
      onPlaying={handlePlaying}
      onRateChange={event("ratechange")}
      onSeeked={event("seeked")}
      onSeeking={event("seeking")}
      onStalled={event("stalled")}
      onSuspend={event("suspend")}
      onVolumeChange={event("volumechange")}
      onWaiting={event("waiting")}
      style={{
        objectFit: props.objectFit || "contain",
        backgroundColor: "transparent",
        width: "100%",
        height: "100%",
        maxWidth: "100%",
        maxHeight: "100%",
        transform: ingest ? "scaleX(-1)" : undefined,
      }}
      disablePictureInPicture={props.pictureInPictureEnabled === false}
    />
  );
});

export function ProgressiveMP4Player(props: VideoProps) {
  return <VideoElement {...props} />;
}

export function ProgressiveWebMPlayer(props: VideoProps) {
  return <VideoElement {...props} />;
}

export function HLSPlayer(props: VideoProps) {
  const localRef = useRef<HTMLVideoElement | null>(null);

  useEffect(() => {
    if (!localRef.current) {
      return;
    }
    if (Hls.isSupported()) {
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

  return <VideoElement {...props} ref={localRef} />;
}

export function WebRTCPlayer(
  props: VideoProps & { videoRef?: React.RefObject<HTMLVideoElement> },
) {
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
      <View backgroundColor="#111">
        <View>
          <View>
            <Text>WebRTC Not Supported</Text>
          </View>
          <Text>{webrtcError}</Text>
          {diagnostics.errors.length > 0 && (
            <View>
              <Text>Technical Details:</Text>
              {diagnostics.errors.map((error, index) => (
                <Text key={index}>• {error}</Text>
              ))}
            </View>
          )}
          <Text>
            • To use WebRTC, you may need to disable any blocking extensions or
            update your browser.
          </Text>
          <Text style={[mt[2]]}>Switching to HLS...</Text>
        </View>
      </View>
    );
  }
  return <WebRTCPlayerInner url={props.url} videoRef={props.videoRef} />;
}

const connectionStatusMessages: Record<string, string> = {
  initializing: "Starting up...",
  connecting: "Connecting...",
  "connection-failed": "Connecting...",
  connected: "Connected",
  reconnecting: "Reconnecting...",
  checking: "Checking connection...",
};

export function WebRTCPlayerInner({
  videoRef,
  url,
  width,
  height,
}: {
  videoRef?: React.RefObject<HTMLVideoElement>;
  url: string;
  width?: string | number;
  height?: string | number;
}) {
  const [connectionStatus, setConnectionStatus] =
    useState<string>("initializing");

  const status = usePlayerStore((x) => x.status);
  const setStatus = usePlayerStore((x) => x.setStatus);

  const playerEvent = usePlayerStore((x) => x.playerEvent);
  const spurl = useStreamplaceStore((x) => x.url);

  const [mediaStream, stuck] = useWebRTC(url);

  useEffect(() => {
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
    if (!stuck && status === PlayerStatus.STALLED) {
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
    const unmute = evt("playing");

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
    if (!videoRef || !videoRef.current) {
      return;
    }
    videoRef.current.srcObject = mediaStream;
  }, [mediaStream]);

  if (!mediaStream) {
    const isError = connectionStatus === "connection-failed";
    return (
      <View
        backgroundColor="#111"
        style={{
          minWidth: "100%",
          minHeight: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <View
          style={{
            borderRadius: borderRadius.md,
            padding: 24,
            alignItems: "center",
            gap: 16,
          }}
        >
          {!isError && <Loader size="large" />}
          <Text size="lg" weight="semibold">
            {connectionStatusMessages[connectionStatus] || "Connecting..."}
          </Text>
        </View>
      </View>
    );
  }
  return <VideoElement url={url} ref={videoRef} />;
}

export function WebcamIngestPlayer(props: VideoProps) {
  const ingestStarting = usePlayerStore((x) => x.ingestStarting);
  const ingestMediaSource = usePlayerStore((x) => x.ingestMediaSource);
  const ingestAutoStart = usePlayerStore((x) => x.ingestAutoStart);

  const [error, setError] = useState<Error | null>(null);

  let streamKey = null;

  const [videoElement, setVideoElement] = useState<HTMLVideoElement | null>(
    null,
  );
  const handleRef = useCallback((node: HTMLVideoElement | null) => {
    if (node) {
      setVideoElement(node);
    }
  }, []);

  const url = useStreamplaceStore((x) => x.url);
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
            width: { min: 200, ideal: 1080, max: 2160 },
            height: { min: 200, ideal: 1920, max: 3840 },
          },
        })
        .then((stream) => {
          setLocalMediaStream(stream);
        })
        .catch((e) => {
          console.error("error getting user media", e.name);
          if (e.name == "NotAllowedError") {
            setError(
              new Error(
                "Unable to access video! Please allow it in your browser settings.",
              ),
            );
          }
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
    setRemoteMediaStream(localMediaStream);
  }, [localMediaStream, ingestStarting, ingestAutoStart]);

  useEffect(() => {
    if (!videoElement) {
      return;
    }
    if (!localMediaStream) {
      return;
    }
    videoElement.srcObject = localMediaStream;
  }, [videoElement, localMediaStream]);

  if (error) {
    return (
      <View
        backgroundColor="#111"
        style={{
          minWidth: "100%",
          minHeight: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <View
          backgroundColor={colors.destructive[900]}
          style={{
            borderRadius: borderRadius.md,
            padding: 24,
            alignItems: "center",
            gap: 16,
            maxWidth: 400,
          }}
        >
          <Text size="xl" weight="extrabold" color="default">
            {error.message}
          </Text>
        </View>
      </View>
    );
  }

  return <VideoElement {...props} ref={handleRef} />;
}
