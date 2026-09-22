import { useEffect, useRef } from "react";
import { ChatMessageViewHydrated, place, VideoViewHydrated } from "streamplace";
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

// Fetches the chat of the livestream(s) the video was recorded from, every
// page of it, and puts it in the store as one oldest-first list; null when
// the video is not a recording of a livestream (the node says so with an
// empty livestreams list). Refetches with the uri or the agent.
export function useChatReplayFetch(aturi: string) {
  const store = getVideoStoreFromContext();
  const agent = usePossiblyUnauthedPDSAgent();

  useEffect(() => {
    if (!agent || !aturi) {
      return;
    }
    const abort = new AbortController();
    let cancelled = false;
    const { setReplay } = store.getState();
    setReplay(null);
    (async () => {
      const messages: ChatMessageViewHydrated[] = [];
      let cursor: string | undefined;
      let startedAt: string | undefined;
      let livestreams: string[] = [];
      // A long stream's chat is a few thousand messages; 40 pages of 1000
      // is well past that and keeps a runaway cursor from looping forever.
      for (let page = 0; page < 40; page++) {
        const res = await agent.client.call(
          place.stream.chat.getReplay,
          { video: aturi as any, limit: 1000, cursor } as any,
          { signal: abort.signal },
        );
        if (cancelled) return;
        livestreams = res.livestreams;
        startedAt = res.startedAt ?? startedAt;
        messages.push(...(res.messages as ChatMessageViewHydrated[]));
        if (!res.cursor) break;
        cursor = res.cursor;
      }
      if (cancelled) return;
      if (livestreams.length === 0 || !startedAt) {
        setReplay(null);
        return;
      }
      setReplay({
        messages,
        livestreams,
        startedAt: new Date(startedAt).getTime(),
      });
    })().catch((e) => {
      if (cancelled || abort.signal.aborted) return;
      console.error("error fetching chat replay", e);
    });

    return () => {
      cancelled = true;
      abort.abort();
    };
  }, [agent, aturi, store]);
}
