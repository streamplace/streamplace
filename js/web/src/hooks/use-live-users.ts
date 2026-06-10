// Fetches live streams via React Query with automatic 30s polling.
// Only active while a component that calls this hook is mounted,
// so navigating away from the home screen stops the requests.
import { useQuery } from "@tanstack/react-query";
import { useCallback } from "react";
import { PlaceStreamLivestream } from "streamplace";
import { useStore } from "../lib/store";

export type LivestreamView = PlaceStreamLivestream.LivestreamView;

const REFETCH_INTERVAL = 30_000;

export function useLiveUsers() {
  const anonPDSAgent = useStore((state) => state.anonPDSAgent);
  const streamplaceUrl = useStore((state) => state.url);

  const getAgent = useCallback(async () => {
    if (anonPDSAgent) return anonPDSAgent;
    const { StreamplaceAgent } = await import("streamplace");
    return new StreamplaceAgent(streamplaceUrl);
  }, [anonPDSAgent, streamplaceUrl]);

  return useQuery({
    queryKey: ["liveUsers"],
    queryFn: async (): Promise<LivestreamView[]> => {
      const agent = await getAgent();
      const result = await agent.place.stream.live.getLiveUsers();
      return (result.data.streams ?? []) as LivestreamView[];
    },
    refetchInterval: REFETCH_INTERVAL,
    refetchIntervalInBackground: false,
  });
}
