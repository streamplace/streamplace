import { useVideoPlayer, VideoPlayerEvents, VideoView } from "expo-video";
import React, { useEffect } from "react";
import { RTCView } from "react-native-webrtc";
import { View } from "tamagui";
import { PlayerProps, PlayerStatus, PROTOCOL_WEBRTC } from "./props";
import { srcToUrl } from "./shared";
import useWebRTC from "./use-webrtc";
import { MediaStream } from "react-native-webrtc";
import { usePlayerProtocol } from "features/player/playerSlice";
import { useAppSelector } from "store/hooks";

// export function Player() {
//   return <View f={1}></View>;
// }

export default function NativeVideo(
  props: PlayerProps & { nativeVideoRef: React.RefObject<VideoView> },
) {
  const protocol = useAppSelector(usePlayerProtocol());
  if (protocol === PROTOCOL_WEBRTC) {
    return <NativeWHEP {...props} />;
  }
  const { url } = srcToUrl(props, protocol);
  useEffect(() => {
    return () => {
      props.setStatus(PlayerStatus.START);
    };
  }, []);
  const player = useVideoPlayer(url, (player) => {
    player.loop = true;
    player.muted = props.muted;
    player.play();
  });

  useEffect(() => {
    player.muted = props.muted;
  }, [props.muted, player]);

  useEffect(() => {
    player.volume = props.volume;
  }, [props.volume, player]);

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
      const now = new Date();
      return player.addListener(evType, (...args) => {
        props.playerEvent(now.toISOString(), evType, { args: args });
      });
    });

    subs.push(
      player.addListener("playingChange", (newIsPlaying) => {
        if (newIsPlaying) {
          props.setStatus(PlayerStatus.PLAYING);
        } else {
          props.setStatus(PlayerStatus.WAITING);
        }
      }),
    );

    return () => {
      for (const sub of subs) {
        sub.remove();
      }
    };
  }, [player]);

  return (
    <VideoView
      style={{ flex: 1, backgroundColor: "#111" }}
      ref={props.nativeVideoRef}
      player={player}
      allowsFullscreen
      nativeControls={props.fullscreen}
      onFullscreenEnter={() => {
        props.setFullscreen(true);
      }}
      onFullscreenExit={() => {
        props.setFullscreen(false);
      }}
      allowsPictureInPicture
    />
  );
}

export function NativeWHEP(props: PlayerProps) {
  const { url } = srcToUrl(props, PROTOCOL_WEBRTC);
  const [stream, stuck] = useWebRTC(url);
  useEffect(() => {
    if (stuck) {
      props.setStatus(PlayerStatus.STALLED);
    } else {
      props.setStatus(PlayerStatus.PLAYING);
    }
  }, [stuck]);
  const mediaStream = stream as unknown as MediaStream;
  useEffect(() => {
    if (!mediaStream) {
      props.setStatus(PlayerStatus.WAITING);
      return;
    }
    props.setStatus(PlayerStatus.PLAYING);
  }, [mediaStream]);
  useEffect(() => {
    if (!mediaStream) {
      return;
    }
    mediaStream.getTracks().forEach((track) => {
      if (track.kind === "audio") {
        track._setVolume(props.muted ? 0 : props.volume);
      }
    });
  }, [mediaStream, props.muted, props.volume]);
  if (!mediaStream) {
    return <View></View>;
  }
  return (
    <RTCView
      mirror={false}
      objectFit={"contain"}
      streamURL={mediaStream.toURL()}
      style={{
        backgroundColor: "#111",
        flex: 1,
      }}
    />
  );
}
