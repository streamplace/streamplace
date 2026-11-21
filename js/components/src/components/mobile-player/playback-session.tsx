import { useEffect, useRef } from "react";
import { place } from "streamplace";
import { useLivestreamStoreOptional } from "../../livestream-store/use-store";
import {
  PlayerProtocol,
  PlayerStatus,
  usePlayerStore,
} from "../../player-store";
import { useDID } from "../../streamplace-store";
import { usePDSAgent } from "../../streamplace-store/xrpc";
import { srcToUrl } from "./shared";

// Pre-live playback: the streamer previewing their own stream needs their
// own playback session (an anonymous one opens published streams only).
// It only opens the caller's own stream, so ask when this is that viewer,
// and again before it expires. Fullscreen calls this next to the video so
// every place that mounts a player (the live dashboard included) gets it,
// and so the session lasts as long as the video whose URL carries it.
// A playback error fetches the playlist once to learn why: 400/403 re-mints
// (at most every REMINT_MS), 409 (evicted) waits for "Play here".
const REMINT_MS = 15_000;

export function usePlaybackSession(src: string) {
  const myDid = useDID();
  const streamerDid = useLivestreamStoreOptional((x) => x.profile?.did);
  const agent = usePDSAgent();
  const setPlaybackSession = usePlayerStore((x) => x.setPlaybackSession);
  const elsewhere = usePlayerStore((x) => x.playbackStartedElsewhere);
  const setElsewhere = usePlayerStore((x) => x.setPlaybackStartedElsewhere);
  const setActions = usePlayerStore((x) => x.setPlaybackSessionActions);
  const failed = usePlayerStore(
    (x) => !!x.playbackSession && x.status === PlayerStatus.ERROR,
  );
  const selectedRendition = usePlayerStore((x) => x.selectedRendition);
  const { url } = srcToUrl({ src, selectedRendition }, PlayerProtocol.HLS);
  const urlRef = useRef(url);
  urlRef.current = url;
  const elsewhereRef = useRef(elsewhere);
  elsewhereRef.current = elsewhere;
  const remint = useRef<(() => void) | undefined>(undefined);
  const isMine =
    !!myDid && (src === myDid || (!!streamerDid && streamerDid === myDid));
  useEffect(() => {
    if (!isMine || !agent) {
      setPlaybackSession(undefined);
      if (!isMine) setElsewhere(false);
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let lastMint = 0;
    const schedule = (ms: number) => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        if (!elsewhereRef.current) void fetchToken();
      }, ms);
    };
    const fetchToken = async (reclaim = false) => {
      lastMint = Date.now();
      try {
        const res = await agent.client.call(
          place.stream.playback.getPlaybackSession,
        );
        // Evicted meanwhile: only "Play here" takes playback back.
        if (cancelled || (elsewhereRef.current && !reclaim)) return;
        setPlaybackSession(res.sid);
        if (reclaim) setElsewhere(false);
        const msLeft = new Date(res.expiresAt).getTime() - Date.now();
        schedule(Math.max(60_000, msLeft - 5 * 60_000));
      } catch (e) {
        if (cancelled) return;
        console.warn("could not fetch a playback session", e);
        schedule(60_000);
      }
    };
    remint.current = () =>
      schedule(Math.max(0, lastMint + REMINT_MS - Date.now()));
    setActions({
      playHere: () => void fetchToken(true),
      remint: () => remint.current?.(),
    });
    if (!elsewhereRef.current) void fetchToken();
    return () => {
      cancelled = true;
      remint.current = undefined;
      if (timer) clearTimeout(timer);
      setActions(undefined);
      setPlaybackSession(undefined);
    };
  }, [isMine, agent, setPlaybackSession, setElsewhere, setActions]);
  useEffect(() => {
    if (!failed) return;
    const probed = urlRef.current;
    fetch(probed).then(
      (res) => {
        if (urlRef.current !== probed) return;
        if (res.status === 409) setElsewhere(true);
        else if (res.status === 400 || res.status === 403) remint.current?.();
      },
      () => {},
    );
  }, [failed, setElsewhere]);
}
