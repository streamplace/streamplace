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
  const { now, connectionQuality, timeBetweenSegments, segmentDeltas } =
    useSegmentTiming();

  return {
    ...deriveLivestreamStatus(
      { websocketConnected, segment, livestream, profile, hasReceivedSegment },
      now,
    ),
    connectionQuality,
    timeBetweenSegments,
    hasTimingReadings: segmentDeltas.length > 0,
  };
}

export type LiveDashboardStatus = ReturnType<typeof useLiveDashboardStatus>;
