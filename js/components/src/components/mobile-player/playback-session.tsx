import { useEffect } from "react";
import { place } from "streamplace";
import { useLivestreamStoreOptional } from "../../livestream-store/use-store";
import { usePlayerStore } from "../../player-store";
import { useDID } from "../../streamplace-store";
import { usePDSAgent } from "../../streamplace-store/xrpc";

// Pre-live over HLS: the streamer previewing their own stream needs their
// own playback session (an anonymous one opens published streams only).
// It only opens the caller's own stream, so ask when this is that viewer,
// and again before it expires. Fullscreen calls this next to the video so
// every place that mounts a player (the live dashboard included) gets it,
// and so the session lasts as long as the video whose URL carries it.
export function usePlaybackSession(src: string) {
  const myDid = useDID();
  const streamerDid = useLivestreamStoreOptional((x) => x.profile?.did);
  const agent = usePDSAgent();
  const setPlaybackSession = usePlayerStore((x) => x.setPlaybackSession);
  const isMine =
    !!myDid && (src === myDid || (!!streamerDid && streamerDid === myDid));
  useEffect(() => {
    if (!isMine || !agent) {
      setPlaybackSession(undefined);
      return;
    }
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | null = null;
    const fetchToken = async () => {
      try {
        const res = await agent.client.call(
          place.stream.playback.getPlaybackSession,
        );
        if (cancelled) return;
        setPlaybackSession(res.sid);
        const msLeft = new Date(res.expiresAt).getTime() - Date.now();
        timer = setTimeout(fetchToken, Math.max(60_000, msLeft - 5 * 60_000));
      } catch (e) {
        if (cancelled) return;
        console.warn("could not fetch a playback session", e);
        timer = setTimeout(fetchToken, 60_000);
      }
    };
    void fetchToken();
    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      setPlaybackSession(undefined);
    };
  }, [isMine, agent, setPlaybackSession]);
}
