import { useLivestreamStore, useSegmentTiming } from "@streamplace/components";
import { deriveLivestreamStatus } from "@streamplace/core";

export function useLiveDashboardStatus() {
  const websocketConnected = useLivestreamStore(
    (state) => state.websocketConnected,
  );
  const segment = useLivestreamStore((state) => state.segment);
  const livestream = useLivestreamStore((state) => state.livestream);
  const profile = useLivestreamStore((state) => state.profile);
  const hasReceivedSegment = useLivestreamStore(
    (state) => state.hasReceivedSegment,
  );
  const { now, connectionQuality } = useSegmentTiming();

  return {
    ...deriveLivestreamStatus(
      { websocketConnected, segment, livestream, profile, hasReceivedSegment },
      now,
    ),
    connectionQuality,
  };
}

export type LiveDashboardStatus = ReturnType<typeof useLiveDashboardStatus>;
