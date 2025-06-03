import { useRenditions, useSegment } from "@streamplace/components";
import { usePlayerSelectedRendition } from "features/player/playerSlice";
import { selectUserMuted } from "features/streamplace/streamplaceSlice";
import usePlatform from "hooks/usePlatform";
import useStreamplaceNode from "hooks/useStreamplaceNode";
import { uuidv7 } from "hooks/uuid";
import { useEffect, useMemo, useState } from "react";
import { useAppSelector } from "store/hooks";
import { Text, View } from "tamagui";
import Fullscreen from "./fullscreen";
import {
  IngestMediaSource,
  PlayerEvent,
  PlayerProps,
  PlayerStatus,
  PlayerStatusTracker,
} from "./props";
import PlayerProvider from "./provider";

const HIDE_CONTROLS_AFTER = 2000;
const OFFLINE_THRESHOLD = 10000;

export function Player(props: Partial<PlayerProps>) {
  return (
    <PlayerProvider {...props}>
      <PlayerInner {...props} />
    </PlayerProvider>
  );
}

export function PlayerInner(props: Partial<PlayerProps>) {
  if (typeof props.src !== "string") {
    return (
      <View>
        <Text>No source provided 🤷</Text>
      </View>
    );
  }
  const userMuted = useAppSelector(selectUserMuted);
  const playerId = useMemo(() => props.playerId ?? uuidv7(), [props.playerId]);
  const [muted, setMuted] = useState(userMuted ?? false);
  const [volume, setVolume] = useState(1.0);

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
  const { url } = useStreamplaceNode();
  const info = usePlatform();
  const playerEvent = async (
    time: string,
    eventType: string,
    meta: { [key: string]: any },
  ) => {
    if (props.telemetry !== true) {
      return;
    }
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
  const [playTime, setPlayTime] = useState(0);
  const [fullscreen, setFullscreen] = useState(false);

  const [offline, setOffline] = useState(true);
  const playing = status === PlayerStatus.PLAYING;

  const segment = useSegment();
  const [lastCheck, setLastCheck] = useState(0);

  const renditions = useRenditions();
  const selectedRendition = useAppSelector(usePlayerSelectedRendition());

  useEffect(() => {
    if (playing) {
      setOffline(false);
      return;
    }
    if (!segment) {
      setOffline(false);
      return;
    }
    const startTime = Date.parse(segment.startTime);
    if (!startTime) {
      console.error("startTime is not a number", segment.startTime);
      return;
    }
    const timeSinceStart = Date.now() - startTime;
    if (timeSinceStart > OFFLINE_THRESHOLD) {
      setOffline(true);
      return;
    }
    const handle = setTimeout(() => {
      setLastCheck(Date.now());
    }, 1000);
    return () => clearTimeout(handle);
  }, [segment, playing, lastCheck]);

  const [muteWasForced, setMuteWasForced] = useState(false);

  const childProps: PlayerProps = {
    playerId: playerId,
    ingest: props.ingest,
    name: props.ingest ? "Go Live" : props.name || props.src,
    telemetry: props.telemetry ?? false,
    src: props.src,
    muted: muted,
    volume: volume,
    setMuted: setMuted,
    setVolume: setVolume,
    setFullscreen: setFullscreen,
    fullscreen: fullscreen,
    offline: offline,
    showControls: props.showControls ?? showControls,
    userInteraction: userInteraction,
    playerEvent: playerEvent,
    status: status,
    setStatus: setStatus,
    playTime: playTime,
    setPlayTime: setPlayTime,
    ingestMediaSource: props.ingestMediaSource ?? IngestMediaSource.USER,
    ingestAutoStart: props.ingestAutoStart ?? false,
    renditions: renditions ?? [],
    selectedRendition: selectedRendition ?? "source",
    muteWasForced: muteWasForced,
    setMuteWasForced: setMuteWasForced,
    embedded: props.embedded ?? false,
    videoRef: props.videoRef,
    ...props,
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
