import { useVideoPlayer, VideoPlayerEvents, VideoView } from "expo-video";
import { ArrowRight } from "lucide-react-native";
import { useCallback, useEffect, useRef, useState } from "react";
import { LayoutChangeEvent, Linking } from "react-native";
import {
  MediaStream,
  RTCView,
  RTCView as RTCViewIngest,
} from "react-native-webrtc";
import {
  Button,
  IngestMediaSource,
  PlayerStatus as IngestPlayerStatus,
  PlayerProtocol,
  PlayerStatus,
  Text,
  useEffectiveVolume,
  usePlayerStore as useIngestPlayerStore,
  useMuted,
  usePlayerStore,
  useStreamplaceStore,
  View,
} from "../..";
import {
  borderRadius,
  colors,
  fontWeight,
  gap,
  h,
  layout,
  m,
  p,
} from "../../lib/theme/atoms";
import { srcToUrl } from "./shared";
import useWebRTC, { useWebRTCIngest } from "./use-webrtc";
import { mediaDevices, WebRTCMediaStream } from "./webrtc-primitives.native";

// Add NativeIngestPlayer to the switch below!
export default function VideoNative(props?: {
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
}) {
  const protocol = usePlayerStore((x) => x.protocol);
  const ingest = usePlayerStore((x) => x.ingestConnectionState) != null;

  return (
    <View>
      {ingest ? (
        <NativeIngestPlayer objectFit={props?.objectFit} />
      ) : protocol === PlayerProtocol.WEBRTC ? (
        <NativeWHEP
          objectFit={props?.objectFit}
          pictureInPictureEnabled={props?.pictureInPictureEnabled}
        />
      ) : (
        <NativeVideo
          objectFit={props?.objectFit}
          pictureInPictureEnabled={props?.pictureInPictureEnabled}
        />
      )}
    </View>
  );
}

export function NativeVideo(props?: {
  objectFit?: "contain" | "cover" | "fill" | "none" | "scale-down";
  pictureInPictureEnabled?: boolean;
}) {
  const videoRef = useRef<VideoView | null>(null);
  const protocol = usePlayerStore((x) => x.protocol);

  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const src = usePlayerStore((x) => x.src);
  const { url } = srcToUrl({ src: src, selectedRendition }, protocol);
  const setStatus = usePlayerStore((x) => x.setStatus);
  const muted = useMuted();
  const volume = useEffectiveVolume();
  const setFullscreen = usePlayerStore((x) => x.setFullscreen);
  const fullscreen = usePlayerStore((x) => x.fullscreen);
  const playerEvent = usePlayerStore((x) => x.playerEvent);
  const spurl = useStreamplaceStore((x) => x.url);

  const setPlayerWidth = usePlayerStore((x) => x.setPlayerWidth);
  const setPlayerHeight = usePlayerStore((x) => x.setPlayerHeight);

  // State for live dimensions
  const [dimensions, setDimensions] = useState<{
    width: number;
    height: number;
  }>({ width: 0, height: 0 });

  const handleLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setDimensions({ width, height });
    setPlayerWidth(width);
    setPlayerHeight(height);
  }, []);

  useEffect(() => {
    return () => {
      setStatus(PlayerStatus.START);
    };
  }, [setStatus]);

  const player = useVideoPlayer(url, (player) => {
    player.addListener("playingChange", (newIsPlaying) => {
      console.log("playingChange", newIsPlaying);
      if (newIsPlaying) {
        setStatus(PlayerStatus.PLAYING);
      } else {
        setStatus(PlayerStatus.WAITING);
      }
    });
    player.loop = true;
    player.muted = muted;
    player.play();
  });

  useEffect(() => {
    player.muted = muted;
  }, [muted, player]);

  useEffect(() => {
    player.volume = volume;
  }, [volume, player]);

  useEffect(() => {
    const subs = (
      [
        "playToEnd",
        "playbackRateChange",
        "playingChange",
        "sourceChange",
        "statusChange",
        "volumeChange",
      ] as (keyof VideoPlayerEvents)[]
    ).map((evType) => {
      return player.addListener(evType, (...args) => {
        const now = new Date();
        console.log("video native event", evType);
        playerEvent(spurl, now.toISOString(), evType, { args: args });
      });
    });

    subs.push(
      player.addListener("playingChange", (newIsPlaying) => {
        console.log("playingChange", newIsPlaying);
        if (newIsPlaying) {
          setStatus(PlayerStatus.PLAYING);
        } else {
          setStatus(PlayerStatus.WAITING);
        }
      }),
    );

    return () => {
      for (const sub of subs) {
        sub.remove();
      }
    };
  }, [player, playerEvent, setStatus, spurl]);

  return (
    <>
      <VideoView
        ref={videoRef}
        player={player}
        allowsFullscreen
        nativeControls={fullscreen}
        onFullscreenEnter={() => {
          setFullscreen(true);
        }}
        onFullscreenExit={() => {
          setFullscreen(false);
        }}
        allowsPictureInPicture={props?.pictureInPictureEnabled !== false}
        onLayout={handleLayout}
      />
    </>
  );
}

export function NativeWHEP(props?: {
  objectFit?: "contain" | "cover";
  pictureInPictureEnabled?: boolean;
}) {
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const src = usePlayerStore((x) => x.src);
  const { url } = srcToUrl(
    { src: src, selectedRendition },
    PlayerProtocol.WEBRTC,
  );
  const [stream, stuck] = useWebRTC(url);
  const status = usePlayerStore((x) => x.status);

  const setPlayerWidth = usePlayerStore((x) => x.setPlayerWidth);
  const setPlayerHeight = usePlayerStore((x) => x.setPlayerHeight);

  // PiP support: wire up videoRef (no direct ref for RTCView)
  const setVideoRef = usePlayerStore((x) => x.setVideoRef);

  // State for live dimensions
  const [dimensions, setDimensions] = useState<{
    width: number;
    height: number;
  }>({ width: 0, height: 0 });

  const handleLayout = useCallback((event: LayoutChangeEvent) => {
    const { width, height } = event.nativeEvent.layout;
    setDimensions({ width, height });
    setPlayerWidth(width);
    setPlayerHeight(height);
  }, []);

  const setStatus = usePlayerStore((x) => x.setStatus);
  const muted = useMuted();
  const volume = useEffectiveVolume();

  useEffect(() => {
    if (stuck && status === PlayerStatus.PLAYING) {
      console.log("setting status to stalled", status);
      setStatus(PlayerStatus.STALLED);
    }
    if (!stuck && status === PlayerStatus.STALLED) {
      console.log("setting status to playing", status);
      setStatus(PlayerStatus.PLAYING);
    }
  }, [stuck, status]);

  const mediaStream = stream as unknown as MediaStream;

  // useEffect(() => {
  //   if (!mediaStream) {
  //     setStatus(PlayerStatus.WAITING);
  //     return;
  //   }
  //   setStatus(PlayerStatus.PLAYING);
  // }, [mediaStream, setStatus]);

  useEffect(() => {
    if (!mediaStream) {
      return;
    }
    mediaStream.getTracks().forEach((track) => {
      if (track.kind === "audio") {
        track._setVolume(muted ? 0 : volume);
      }
    });
  }, [mediaStream, muted, volume]);

  // Keep the playerStore videoRef in sync for PiP (if possible)
  useEffect(() => {
    if (typeof setVideoRef === "function") {
      setVideoRef(null); // No direct ref for RTCView, but keep API consistent
    }
  }, [setVideoRef]);

  if (!mediaStream) {
    return <View></View>;
  }

  return (
    <>
      <RTCView
        mirror={false}
        objectFit={props?.objectFit || "contain"}
        streamURL={mediaStream.toURL()}
        onLayout={handleLayout}
        pictureInPictureEnabled={props?.pictureInPictureEnabled !== false}
        autoStartPictureInPicture={true}
        pictureInPicturePreferredSize={{
          width: 160,
          height: 90,
        }}
        style={{
          minWidth: "100%",
          minHeight: "100%",
          flex: 1,
        }}
      />
    </>
  );
}

export function NativeIngestPlayer(props?: {
  objectFit?: "contain" | "cover";
}) {
  const ingestStarting = useIngestPlayerStore((x) => x.ingestStarting);
  const ingestMediaSource = useIngestPlayerStore((x) => x.ingestMediaSource);
  const ingestAutoStart = useIngestPlayerStore((x) => x.ingestAutoStart);
  const setStatus = useIngestPlayerStore((x) => x.setStatus);
  const setVideoRef = usePlayerStore((x) => x.setVideoRef);

  const [error, setError] = useState<Error | null>(null);

  const ingestCamera = useIngestPlayerStore((x) => x.ingestCamera);

  useEffect(() => {
    setStatus(IngestPlayerStatus.PLAYING);
  }, [setStatus]);

  useEffect(() => {
    if (typeof setVideoRef === "function") {
      setVideoRef(null);
    }
  }, [setVideoRef]);

  const url = useStreamplaceStore((x) => x.url);
  const [lms, setLocalMediaStream] = useState<WebRTCMediaStream | null>(null);
  const [, setRemoteMediaStream] = useWebRTCIngest({
    endpoint: `${url}/api/ingest/webrtc`,
  });

  // Use lms directly as localMediaStream
  const localMediaStream = lms;

  useEffect(() => {
    if (ingestMediaSource === IngestMediaSource.DISPLAY) {
      mediaDevices
        .getDisplayMedia()
        .then((stream: WebRTCMediaStream) => {
          console.log("display media", stream);
          setLocalMediaStream(stream);
        })
        .catch((e: any) => {
          console.log("error getting display media", e);
          console.error("error getting display media", e);
        });
    } else {
      mediaDevices
        .getUserMedia({
          audio: {
            // deviceId: "audio-1",
            // echoCancellation: true,
            // autoGainControl: true,
            // noiseSuppression: true,
            // latency: false,
            // channelCount: false,
          },
          video: {
            facingMode: ingestCamera,
            width: { min: 200, ideal: 1080, max: 2160 },
            height: { min: 200, ideal: 1920, max: 3840 },
          },
        })
        .then((stream: WebRTCMediaStream) => {
          setLocalMediaStream(stream);

          let errs: string[] = [];
          if (stream.getAudioTracks().length === 0) {
            console.warn("No audio tracks found in user media stream");
            errs.push("microphone");
          }
          if (stream.getVideoTracks().length === 0) {
            console.warn("No video tracks found in user media stream");
            errs.push("camera");
          }
          if (errs.length > 0) {
            setError(
              new Error(
                `We could not access your ${errs.join(" and ")}. To stream, you need to give us permission to access these.`,
              ),
            );
          } else {
            setError(null);
          }
        })
        .catch((e: any) => {
          console.error("error getting user media", e);
          setError(
            new Error(
              "We could not access your camera or microphone. To stream, you need to give us permission to access these.",
            ),
          );
        });
    }
  }, [ingestMediaSource, ingestCamera]);

  useEffect(() => {
    if (!ingestStarting && !ingestAutoStart) {
      setRemoteMediaStream(null);
      return;
    }
    if (!localMediaStream) {
      return;
    }
    console.log("setting remote media stream", localMediaStream);
    // @ts-expect-error: WebRTCMediaStream may not have all MediaStream properties, but is compatible for our use
    setRemoteMediaStream(localMediaStream);
  }, [localMediaStream, ingestStarting, ingestAutoStart, setRemoteMediaStream]);

  if (!localMediaStream) {
    return null;
  }

  if (error) {
    return (
      <View
        backgroundColor={colors.destructive[900]}
        style={[p[4], m[4], gap.all[2], { borderRadius: borderRadius.md }]}
      >
        <View>
          <Text style={[fontWeight.semibold]} size="2xl">
            Error encountered!
          </Text>
        </View>
        <Text>{error.message}</Text>
        {error.message.includes(
          "To stream, you need to give us permission to access these.",
        ) && (
          <Button
            onPress={Linking.openSettings}
            style={[h[10]]}
            variant="secondary"
          >
            <View style={[layout.flex.row, gap.all[1]]}>
              <Text>Open Settings</Text> <ArrowRight color="white" size="18" />
            </View>
          </Button>
        )}
      </View>
    );
  }

  return (
    <RTCViewIngest
      mirror={ingestCamera !== "environment"}
      objectFit={props?.objectFit || "contain"}
      streamURL={localMediaStream.toURL()}
      zOrder={0}
      style={{
        minWidth: "100%",
        minHeight: "100%",
        flex: 1,
      }}
    />
  );
}
