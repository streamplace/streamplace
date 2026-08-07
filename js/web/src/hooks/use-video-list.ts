// Fetches a page of place.stream.media.getVideoList and exposes
// cursor-based infinite scroll via @tanstack/react-query.
// Pass a `repo` (DID or handle) to scope the list to a single
// uploader; omit it for the global newest-first feed.
import { useInfiniteQuery } from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { place } from "streamplace";
import { useStore } from "../lib/store";

export type VideoView = place.stream.media.getVideo.VideoView;

const PAGE_SIZE = 24;

interface VideoPage {
  videos: VideoView[];
  cursor: string | undefined;
}

export function useVideoList(repo?: string) {
  const anonPDSAgent = useStore((state) => state.anonPDSAgent);
  const streamplaceUrl = useStore((state) => state.url);

  const getAgent = useCallback(async () => {
    if (anonPDSAgent) return anonPDSAgent;
    const { StreamplaceAgent } = await import("streamplace");
    return new StreamplaceAgent(streamplaceUrl);
  }, [anonPDSAgent, streamplaceUrl]);

  const query = useInfiniteQuery({
    queryKey: ["videoList", repo ?? null],
    queryFn: async ({ pageParam }): Promise<VideoPage> => {
      const agent = await getAgent();
      const res = await agent.client.call(place.stream.media.getVideoList, {
        ...(repo ? { repo } : {}),
        limit: PAGE_SIZE,
        ...(pageParam ? { cursor: pageParam } : {}),
      });
      return {
        videos: (res.data.videos ?? []) as VideoView[],
        cursor: res.data.cursor as string | undefined,
      };
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (lastPage) => {
      if (!lastPage.cursor || lastPage.videos.length === 0) {
        return undefined;
      }
      return lastPage.cursor;
    },
  });

  // Flatten pages into a single array.
  const videos = useMemo(
    () => query.data?.pages.flatMap((p) => p.videos) ?? [],
    [query.data],
  );

  return {
    videos,
    loading: query.isLoading,
    refreshing: query.isRefetching && !query.isFetchingNextPage,
    error: query.error?.message ?? null,
    hasMore: query.hasNextPage,
    loadMore: query.fetchNextPage,
    refresh: query.refetch,
  };
}
