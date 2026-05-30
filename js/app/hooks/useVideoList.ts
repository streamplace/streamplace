import { usePossiblyUnauthedPDSAgent } from "@streamplace/components";
import { useCallback, useEffect, useRef, useState } from "react";
import { PlaceStreamMediaGetVideo } from "streamplace";

export type VideoView = PlaceStreamMediaGetVideo.VideoView;

const PAGE_SIZE = 24;

// Fetches a page of place.stream.media.getVideoList and exposes cursor-based
// infinite scroll. Pass a `repo` (DID or handle) to scope the list to a single
// uploader; omit it for the global newest-first feed. The two listing screens
// (/video and /:did/video) share this hook — they differ only in whether a
// repo is supplied.
export function useVideoList(repo?: string) {
  const agent = usePossiblyUnauthedPDSAgent();
  const [videos, setVideos] = useState<VideoView[]>([]);
  const [cursor, setCursor] = useState<string | undefined>(undefined);
  const [hasMore, setHasMore] = useState(true);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Guards against overlapping fetches (e.g. onEndReached firing repeatedly).
  const inFlight = useRef(false);

  const fetchPage = useCallback(
    async (nextCursor?: string, replace = false) => {
      if (!agent) return;
      if (inFlight.current) return;
      inFlight.current = true;
      if (replace) {
        setRefreshing(true);
      } else {
        setLoading(true);
      }
      setError(null);
      try {
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
    [agent, repo],
  );

  // Initial load + reload when the agent or repo changes.
  useEffect(() => {
    setVideos([]);
    setCursor(undefined);
    setHasMore(true);
    fetchPage(undefined, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agent, repo]);

  const loadMore = useCallback(() => {
    if (loading || refreshing || !hasMore || !cursor) return;
    fetchPage(cursor, false);
  }, [loading, refreshing, hasMore, cursor, fetchPage]);

  const refresh = useCallback(() => {
    fetchPage(undefined, true);
  }, [fetchPage]);

  return { videos, loading, refreshing, error, hasMore, loadMore, refresh };
}
