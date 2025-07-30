import { useEffect, useState } from "react";
import { flex, h, layout, w, zIndex } from "../../lib/theme/atoms";
import { useSegment } from "../../livestream-store";
import {
  PlayerStatus,
  PlayerStatusTracker,
  usePlayerStore,
} from "../../player-store";
import { useStreamplaceStore } from "../../streamplace-store";
import { Text, View } from "../ui";
import { Fullscreen } from "./fullscreen";
import { PlayerProps } from "./props";

const OFFLINE_THRESHOLD = 10000;

export * as PlayerUI from "./ui";

export function Player(
  props: Partial<PlayerProps> & { children?: React.ReactNode },
) {
  const playing = usePlayerStore((x) => x.status === PlayerStatus.PLAYING);

  const setOffline = usePlayerStore((x) => x.setOffline);
  const setIngest = usePlayerStore((x) => x.setIngestConnectionState);

  const clearControlsTimeout = usePlayerStore((x) => x.clearControlsTimeout);

  // Will call back every few seconds to send health updates
  usePlayerStatus();

  useEffect(() => {
    setIngest(props.ingest ? "new" : null);
  }, []);

  if (typeof props.src !== "string") {
    return (
      <View>
        <Text>No source provided 🤷</Text>
      </View>
    );
  }

  useEffect(() => {
    return () => {
      clearControlsTimeout();
    };
  }, []);

  const segment = useSegment();
  const [lastCheck, setLastCheck] = useState(0);

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

  return (
    <>
      <View
        style={[
          zIndex[0],
          w.percent[100],
          h.percent[100],
          flex.shrink[1],
          layout.flex.center,
        ]}
      >
        <Fullscreen src={props.src}>{props.children}</Fullscreen>
      </View>
    </>
  );
}

const POLL_INTERVAL = 5000;
export function usePlayerStatus(): [PlayerStatus] {
  const playerStatus = usePlayerStore((x) => x.status);
  const url = useStreamplaceStore((x) => x.url);
  const playerEvent = usePlayerStore((x) => x.playerEvent);
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
  // callback to update the status
  useEffect(() => {
    const now = new Date();
    if (playerStatus !== whatDoing) {
      setWhatDid(updateWhatDid(now));
      setWhatDoing(playerStatus);
      setDoingSince(now.getTime());
    }
  }, [playerStatus]);

  useEffect(() => {
    if (lastUpdated === 0) {
      return;
    }
    const now = new Date();
    const fullWhatDid = updateWhatDid(now);
    setWhatDid({} as PlayerStatusTracker);
    setDoingSince(now.getTime());
    playerEvent(url, now.toISOString(), "aq-played", {
      whatHappened: fullWhatDid,
    });
  }, [lastUpdated]);

  useEffect(() => {
    const interval = setInterval((_) => {
      setLastUpdated(Date.now());
    }, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, []);
  return [whatDoing];
}
