import { useEffect, useState } from "react";
import { place } from "streamplace";
import { flex, h, layout, w, zIndex } from "../../lib/theme/atoms";
import { useLivestreamStoreOptional } from "../../livestream-store/use-store";
import {
  PlayerStatus,
  PlayerStatusTracker,
  usePlayerStore,
} from "../../player-store";
import {
  useDID,
  useMuted,
  useSetMuted,
  useStreamplaceStore,
} from "../../streamplace-store";
import { usePDSAgent } from "../../streamplace-store/xrpc";
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
  // A stream whose segments carry B-frames can't play over WebRTC (the
  // encoder can't always be talked out of them), so hold such a stream on
  // HLS. The segment record says; the viewer's low-latency preference is
  // kept for streams that can use it.
  const streamHasBFrames = useLivestreamStoreOptional(
    (x) => x.segment?.video?.[0]?.bframes === true,
  );
  const streamForcesHLS = usePlayerStore((x) => x.streamForcesHLS);
  const setStreamForcesHLS = usePlayerStore((x) => x.setStreamForcesHLS);
  const protocol = usePlayerStore((x) => x.protocol);
  useEffect(() => {
    if (streamHasBFrames && (!streamForcesHLS || protocol !== "hls")) {
      setStreamForcesHLS(true);
    } else if (!streamHasBFrames && streamForcesHLS) {
      setStreamForcesHLS(false);
    }
  }, [streamHasBFrames, streamForcesHLS, protocol, setStreamForcesHLS]);

  // Pre-live over HLS: the streamer previewing their own stream needs a
  // playback token (the HLS requests carry no session). It only opens the
  // caller's own stream, so ask when this is that viewer, and again before
  // it expires. Lives here rather than in the app's wrapper so every place
  // that mounts a player (the live dashboard included) gets it.
  const myDid = useDID();
  const streamerDid = useLivestreamStoreOptional((x) => x.profile?.did);
  const agent = usePDSAgent();
  const setLiveToken = usePlayerStore((x) => x.setLiveToken);
  const isMine =
    !!myDid &&
    (props.src === myDid || (!!streamerDid && streamerDid === myDid));
  useEffect(() => {
    if (!isMine || !agent) {
      setLiveToken(undefined);
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const fetchToken = async () => {
      try {
        const res = await agent.client.call(place.stream.playback.getLiveToken);
        if (cancelled) return;
        setLiveToken(res.token);
        const msLeft = new Date(res.expiresAt).getTime() - Date.now();
        timer = setTimeout(fetchToken, Math.max(60_000, msLeft - 5 * 60_000));
      } catch (e) {
        if (cancelled) return;
        console.warn("could not fetch a live playback token", e);
        timer = setTimeout(fetchToken, 60_000);
      }
    };
    void fetchToken();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      setLiveToken(undefined);
    };
  }, [isMine, agent, setLiveToken]);

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

  useEffect(() => {
    return () => {
      clearControlsTimeout();
    };
  }, []);

  if (typeof props.src !== "string") {
    return (
      <View>
        <Text>No source provided 🤷</Text>
      </View>
    );
  }

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
