import React, { useEffect } from "react";
import { useVideoPlayer, VideoPlayerEvents, VideoView } from "expo-video";
import useAquareumNode from "hooks/useAquareumNode";
import {
  PlayerProps,
  PlayerStatus,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
  PROTOCOL_PROGRESSIVE_WEBM,
} from "./props";
import { srcToUrl } from "./shared";

// export function Player() {
//   return <View f={1}></View>;
// }

export default function NativeVideo(
  props: PlayerProps & { videoRef: React.RefObject<VideoView> },
) {
  const { url } = srcToUrl(props);
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
      player.addListener("playingChange", (newIsPlaying, oldIsPlaying) => {
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
      ref={props.videoRef}
      player={player}
      allowsFullscreen
      allowsPictureInPicture
      nativeControls={props.fullscreen}
      onFullscreenEnter={() => {
        props.setFullscreen(true);
      }}
      onFullscreenExit={() => {
        props.setFullscreen(false);
      }}
    />
  );
}
