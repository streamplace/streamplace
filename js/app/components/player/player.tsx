import React, { useEffect, useMemo, useState } from "react";
import { Button, Text, View, XStack } from "tamagui";
import Controls from "./controls";
import Video from "./video";
import Fullscreen from "./fullscreen";
import {
  PlayerEvent,
  PlayerProps,
  PlayerStatus,
  PlayerStatusTracker,
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
  const [status, setStatus] = usePlayerStatus(playerEvent);
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
    status: status,
    setStatus: setStatus,
  };
  return (
    <View f={1} justifyContent="center" position="relative">
      <Fullscreen {...childProps}></Fullscreen>
    </View>
  );
}

const POLL_INTERVAL = 5000;
export function usePlayerStatus(
  playerEvent: (
    time: string,
    eventType: string,
    meta: { [key: string]: any },
  ) => Promise<void>,
): [PlayerStatus, (PlayerStatus) => void] {
  const [whatDoing, setWhatDoing] = useState<PlayerStatus>(PlayerStatus.START);
  const [whatDid, setWhatDid] = useState<PlayerStatusTracker>({});
  const [doingSince, setDoingSince] = useState(Date.now());
  const [lastUpdated, setLastUpdated] = useState(0);
  const updateWhatDid = (now: Date): PlayerStatusTracker => {
    const prev = whatDid[whatDoing] ?? 0;
    const duration = now.getTime() - doingSince;
    const ret = {
      ...whatDid,
      [whatDoing]: prev + duration,
    };
    return ret;
  };
  const updateStatus = (status: PlayerStatus) => {
    const now = new Date();
    if (status !== whatDoing) {
      setWhatDid(updateWhatDid(now));
      setWhatDoing(status);
      setDoingSince(now.getTime());
    }
  };

  useEffect(() => {
    if (lastUpdated === 0) {
      return;
    }
    const now = new Date();
    const fullWhatDid = updateWhatDid(now);
    setWhatDid({} as PlayerStatusTracker);
    setDoingSince(now.getTime());
    playerEvent(now.toISOString(), "aq-played", {
      whatHappened: fullWhatDid,
    });
  }, [lastUpdated]);

  useEffect(() => {
    const interval = setInterval((_) => {
      setLastUpdated(Date.now());
    }, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, []);
  return [whatDoing, updateStatus];
}
