import {
  PlayerProtocol,
  PlayerStatus,
  usePlayerStore,
  useStreamplaceStore,
} from "@streamplace/components";
import { useVideoPlayer, VideoPlayerEvents, VideoView } from "expo-video";
import { useEffect } from "react";
import { MediaStream, RTCView } from "react-native-webrtc";
import { View } from "tamagui";
import { srcToUrl } from "./shared";
import useWebRTC from "./use-webrtc";

export default function VideoNative() {
  const protocol = usePlayerStore((x) => x.protocol);
  if (protocol === PlayerProtocol.WEBRTC) {
    return <NativeWHEP />;
  } else {
    return <NativeVideo />;
  }
}

export function NativeVideo() {
  const protocol = usePlayerStore((x) => x.protocol);

  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const src = usePlayerStore((x) => x.src);
  const { url } = srcToUrl({ src: src, selectedRendition }, protocol);
  const setStatus = usePlayerStore((x) => x.setStatus);
  const muted = usePlayerStore((x) => x.muted);
  const volume = usePlayerStore((x) => x.volume);
  const setFullscreen = usePlayerStore((x) => x.setFullscreen);
  const fullscreen = usePlayerStore((x) => x.fullscreen);
  const playerEvent = usePlayerStore((x) => x.playerEvent);
  const spurl = useStreamplaceStore((x) => x.url);

  useEffect(() => {
    return () => {
      setStatus(PlayerStatus.START);
    };
  }, [setStatus]);

  const player = useVideoPlayer(url, (player) => {
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
      const now = new Date();
      return player.addListener(evType, (...args) => {
        playerEvent(spurl, now.toISOString(), evType, { args: args });
      });
    });

    subs.push(
      player.addListener("playingChange", (newIsPlaying) => {
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
  }, [player, playerEvent, setStatus]);

  return (
    <VideoView
      style={{ flex: 1, backgroundColor: "#111" }}
      //ref={props.nativeVideoRef}
      player={player}
      allowsFullscreen
      nativeControls={fullscreen}
      onFullscreenEnter={() => {
        setFullscreen(true);
      }}
      onFullscreenExit={() => {
        setFullscreen(false);
      }}
      allowsPictureInPicture
    />
  );
}

export function NativeWHEP() {
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const src = usePlayerStore((x) => x.src);
  const { url } = srcToUrl(
    { src: src, selectedRendition },
    PlayerProtocol.WEBRTC,
  );
  const [stream, stuck] = useWebRTC(url);

  const setStatus = usePlayerStore((x) => x.setStatus);
  const muted = usePlayerStore((x) => x.muted);
  const volume = usePlayerStore((x) => x.volume);

  console.log("native whep rendered");

  useEffect(() => {
    if (stuck) {
      setStatus(PlayerStatus.STALLED);
    } else {
      setStatus(PlayerStatus.PLAYING);
    }
  }, [stuck, setStatus]);

  const mediaStream = stream as unknown as MediaStream;

  useEffect(() => {
    if (!mediaStream) {
      setStatus(PlayerStatus.WAITING);
      return;
    }
    setStatus(PlayerStatus.PLAYING);
  }, [mediaStream, setStatus]);

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
