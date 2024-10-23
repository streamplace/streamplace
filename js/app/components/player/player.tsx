import React, { useEffect, useMemo, useState } from "react";
import { Button, Text, View, XStack } from "tamagui";
import Controls from "./controls";
import Video from "./video";
import Fullscreen from "./fullscreen";
import {
  PlayerEvent,
  PlayerProps,
  PROTOCOL_HLS,
  PROTOCOL_PROGRESSIVE_MP4,
} from "./props";
import usePlatform from "hooks/usePlatform";
import useAquareumNode from "hooks/useAquareumNode";
import { uuidv7 } from "hooks/uuid";

const HIDE_CONTROLS_AFTER = 2000;

export function Player(props: Partial<PlayerProps>) {
  if (typeof props.src !== "string") {
    return (
      <View>
        <Text>No source provided 🤷</Text>
      </View>
    );
  }
  const playerId = useMemo(() => props.playerId ?? uuidv7(), [props.playerId]);
  const [muted, setMuted] = useState(true);
  const [showControls, setShowControls] = useState(true);
  const [touchTime, setTouchTime] = useState(0);
  useEffect(() => {
    // Use setTimeout to update the message after 2000 milliseconds (2 seconds)
    const timeoutId = setTimeout(() => {
      setShowControls(false);
    }, HIDE_CONTROLS_AFTER);

    // Cleanup function to clear the timeout if the component unmounts
    return () => clearTimeout(timeoutId);
  }, [touchTime]);
  const userInteraction = () => {
    setTouchTime(Date.now());
    setShowControls(true);
  };
  const plat = usePlatform();
  let defProto = PROTOCOL_PROGRESSIVE_MP4;
  if (plat.isIOS) {
    defProto = PROTOCOL_HLS;
  } else if (plat.isSafari) {
    defProto = PROTOCOL_HLS;
  } else if (plat.isFirefox) {
    defProto = PROTOCOL_HLS;
  }
  const { url } = useAquareumNode();
  const info = usePlatform();
  const playerEvent = async (
    time: string,
    eventType: string,
    meta: { [key: string]: any },
  ) => {
    const data: PlayerEvent = {
      time: time,
      playerId: playerId,
      eventType: eventType,
      meta: {
        ...meta,
        ...info,
      },
    };
    try {
      await fetch(`${url}/api/player-event`, {
        method: "POST",
        body: JSON.stringify(data),
      });
    } catch (e) {
      console.error("error sending player telemetry", e);
    }
  };
  const [protocol, setProtocol] = useState(defProto);
  const [fullscreen, setFullscreen] = useState(false);
  const childProps: PlayerProps = {
    playerId: playerId,
    name: props.name || props.src,
    telemetry: props.telemetry ?? false,
    src: props.src,
    muted: muted,
    setMuted: setMuted,
    setFullscreen: setFullscreen,
    fullscreen: fullscreen,
    protocol: protocol,
    setProtocol: setProtocol,
    showControls: props.showControls ?? showControls,
    userInteraction: userInteraction,
    playerEvent: playerEvent,
  };
  return (
    <View f={1} justifyContent="center" position="relative">
      <Fullscreen {...childProps}></Fullscreen>
    </View>
  );
}
