import { useEffect, useRef } from "react";
import { place } from "streamplace";
import { useLivestreamStoreOptional } from "../../livestream-store/use-store";
import { PlayerStatus, usePlayerStore } from "../../player-store";
import { useDID } from "../../streamplace-store";
import { usePDSAgent } from "../../streamplace-store/xrpc";

// Pre-live over HLS: the streamer previewing their own stream needs their
// own playback session (an anonymous one opens published streams only).
// It only opens the caller's own stream, so ask when this is that viewer,
// and again before it expires. Fullscreen calls this next to the video so
// every place that mounts a player (the live dashboard included) gets it,
// and so the session lasts as long as the video whose URL carries it.
// Owner sessions are bound to the client IP, so a playback error re-mints
// (at most every REMINT_MS) in case the IP changed.
const REMINT_MS = 15_000;

export function usePlaybackSession(src: string) {
  const myDid = useDID();
  const streamerDid = useLivestreamStoreOptional((x) => x.profile?.did);
  const agent = usePDSAgent();
  const setPlaybackSession = usePlayerStore((x) => x.setPlaybackSession);
  const failed = usePlayerStore(
    (x) => !!x.playbackSession && x.status === PlayerStatus.ERROR,
  );
  const remint = useRef<(() => void) | undefined>(undefined);
  const isMine =
    !!myDid && (src === myDid || (!!streamerDid && streamerDid === myDid));
  useEffect(() => {
    if (!isMine || !agent) {
      setPlaybackSession(undefined);
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    let lastMint = 0;
    const schedule = (ms: number) => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(fetchToken, ms);
    };
    const fetchToken = async () => {
      lastMint = Date.now();
      try {
        const res = await agent.client.call(
          place.stream.playback.getPlaybackSession,
        );
        if (cancelled) return;
        setPlaybackSession(res.sid);
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
    void fetchToken();
    return () => {
      cancelled = true;
      remint.current = undefined;
      if (timer) clearTimeout(timer);
      setPlaybackSession(undefined);
    };
  }, [isMine, agent, setPlaybackSession]);
  useEffect(() => {
    if (failed) remint.current?.();
  }, [failed]);
}
