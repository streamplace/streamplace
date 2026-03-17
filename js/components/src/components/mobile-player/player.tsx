import { useEffect, useState } from "react";
import { flex, h, layout, w, zIndex } from "../../lib/theme/atoms";
import {
  PlayerStatus,
  PlayerStatusTracker,
  usePlayerStore,
} from "../../player-store";
import {
  useMuted,
  useSetMuted,
  useStreamplaceStore,
} from "../../streamplace-store";
import { Text, View } from "../ui";
import { Fullscreen } from "./fullscreen";
import { PlayerProps } from "./props";
import ReportModal from "./ui/report-modal";

const OFFLINE_THRESHOLD = 10000;

export * as PlayerUI from "./ui";

export function Player(
  props: Partial<PlayerProps> & { children?: React.ReactNode },
) {
  const setIngest = usePlayerStore((x) => x.setIngestConnectionState);

  const clearControlsTimeout = usePlayerStore((x) => x.clearControlsTimeout);

  const setReportingURL = usePlayerStore((x) => x.setReportingURL);
  const setEmbedded = usePlayerStore((x) => x.setEmbedded);
  const setMode = usePlayerStore((x) => x.setMode);

  const reportModalOpen = usePlayerStore((x) => x.reportModalOpen);
  const setReportModalOpen = usePlayerStore((x) => x.setReportModalOpen);
  const reportSubject = usePlayerStore((x) => x.reportSubject);

  const setMuted = useSetMuted();
  const muted = useMuted();

  // if we set muted, set it and restore after
  useEffect(() => {
    let wasMuted: null | boolean = null;
    setTimeout(() => {
      if (props.muted != undefined) {
        wasMuted = muted;
        setMuted(props.muted);
      }
    }, 200);
    return () => {
      wasMuted !== null && setMuted(wasMuted);
    };
  }, [props.muted]);

  useEffect(() => {
    setReportingURL(props.reportingURL ?? null);
  }, [props.reportingURL]);

  useEffect(() => {
    setEmbedded(props.embedded ?? false);
  }, [props.embedded]);

  useEffect(() => {
    setMode(props.mode ?? "live");
  }, [props.mode]);

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
        <ReportModal
          open={reportModalOpen}
          onOpenChange={setReportModalOpen}
          subject={reportSubject!}
        />
        <Fullscreen
          src={props.src}
          objectFit={props.objectFit}
          pictureInPictureEnabled={props.pictureInPictureEnabled}
        >
          {props.children}
        </Fullscreen>
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
