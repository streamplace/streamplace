// Fetches a page of place.stream.media.getVideoList and exposes
// cursor-based infinite scroll. Port of js/app/hooks/useVideoList.ts.
// Pass a `repo` (DID or handle) to scope the list to a single
// uploader; omit it for the global newest-first feed.
import { useCallback, useEffect, useRef, useState } from "react";
import { PlaceStreamMediaGetVideo } from "streamplace";
import { useStore } from "../lib/store";

export type VideoView = PlaceStreamMediaGetVideo.VideoView;

const PAGE_SIZE = 24;

export function useVideoList(repo?: string) {
  const anonPDSAgent = useStore((state) => state.anonPDSAgent);
  const streamplaceUrl = useStore((state) => state.url);
  const [videos, setVideos] = useState<VideoView[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const inFlight = useRef(false);

  // Lazily resolve the agent. Falls back to constructing one from
  // the streamplace URL if BlueskyProvider hasn't yet run
  // loadOAuthClient (which creates anonPDSAgent).
  const agentRef = useRef(anonPDSAgent);
  if (anonPDSAgent) agentRef.current = anonPDSAgent;

  const getAgent = useCallback(async () => {
    if (agentRef.current) return agentRef.current;
    const { StreamplaceAgent } = await import("streamplace");
    const agent = new StreamplaceAgent(streamplaceUrl);
    agentRef.current = agent;
    return agent;
  }, [streamplaceUrl]);

  const fetchPage = useCallback(
    async (nextCursor?: string, replace = false) => {
      if (inFlight.current) return;
      inFlight.current = true;
      if (replace) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      setError(null);
      try {
        const agent = await getAgent();
        const res = await agent.place.stream.media.getVideoList({
          ...(repo ? { repo } : {}),
          limit: PAGE_SIZE,
          ...(nextCursor ? { cursor: nextCursor } : {}),
        });
        const page = (res.data.videos ?? []) as VideoView[];
        setVideos((prev) => (replace ? page : [...prev, ...page]));
        setCursor(res.data.cursor);
        setHasMore(Boolean(res.data.cursor) && page.length > 0);
      } catch (e: any) {
        console.error("error fetching video list", e);
        setError(e?.message ?? "failed to load videos");
        setHasMore(false);
      } finally {
        inFlight.current = false;
        setLoading(false);
        setRefreshing(false);
      }
    },
    [getAgent, repo],
  );

  // Initial load + reload when the agent or repo changes.
  useEffect(() => {
    setVideos([]);
    setCursor(undefined);
    setHasMore(true);
    fetchPage(undefined, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [getAgent, repo]);

  const loadMore = useCallback(() => {
    if (loading || refreshing || !hasMore || !cursor) return;
    fetchPage(cursor, false);
  }, [loading, refreshing, hasMore, cursor, fetchPage]);

  const refresh = useCallback(() => {
    fetchPage(undefined, true);
  }, [fetchPage]);

  return { videos, loading, refreshing, error, hasMore, loadMore, refresh };
}
