import { useEffect, useRef } from "react";
import { place, VideoViewHydrated } from "streamplace";
import { usePossiblyUnauthedPDSAgent } from "../streamplace-store";
import { getVideoStoreFromContext } from "../video-store";

// Fetches the place.stream.video metadata for the given AT-URI and pushes it
// into the surrounding VideoProvider's store. Refetches whenever the uri or
// the active agent changes. This is the VOD analogue of the livestream
// websocket consumer: instead of a long-lived socket, VOD metadata is a
// one-shot read that we hydrate into the store.
export function useVideoFetch(aturi: string) {
  const store = getVideoStoreFromContext();
  const agent = usePossiblyUnauthedPDSAgent();
  const abortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    if (!agent || !aturi) {
      return;
    }

    // cancel any in-flight request from a previous uri/agent
    abortRef.current?.abort();
    const abort = new AbortController();
    abortRef.current = abort;

    const { setAturi, setVideo, setLoading, setError } = store.getState();
    setAturi(aturi);
    setLoading(true);
    setError(null);

    agent.client
      .call(
        place.stream.media.getVideo,
        { uri: aturi as any },
        { signal: abort.signal },
      )
      .then((res) => {
        if (abort.signal.aborted) return;
        setVideo(res as unknown as VideoViewHydrated);
        setLoading(false);
      })
      .catch((e) => {
        if (abort.signal.aborted) return;
        console.error("error fetching video", e);
        setError(e?.message ?? "failed to load video");
        setLoading(false);
      });

    return () => abort.abort();
  }, [agent, aturi, store]);
}
