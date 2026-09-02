import type { LivestreamStore } from "@streamplace/core";
import { useEffect, useState } from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import {
  deriveLiveness,
  latestActivityAgeSeconds,
  type Liveness,
} from "./liveness-state";

export type { Liveness } from "./liveness-state";

export function useLivenessState(store: LivestreamStore): Liveness {
  const state = useStore(
    store,
    useShallow((s) => ({
      segment: s.segment,
      hasReceivedSegment: s.hasReceivedSegment,
      livestream: s.livestream,
      profile: s.profile,
      websocketConnected: s.websocketConnected,
    })),
  );

  const segmentStartTime = state.segment?.startTime;
  const segmentDurationNanoseconds = state.segment?.duration;
  const lastSeenAt = state.livestream?.record.lastSeenAt;
  const [nowMs, setNowMs] = useState(() => Date.now());

  useEffect(() => {
    setNowMs(Date.now());
    if (!segmentStartTime && !lastSeenAt) return;
    const id = window.setInterval(() => {
      setNowMs(Date.now());
    }, 1000);
    return () => window.clearInterval(id);
  }, [segmentStartTime, segmentDurationNanoseconds, lastSeenAt]);

  const secondsSinceActivity = latestActivityAgeSeconds(
    {
      segmentStartTime,
      segmentDurationNanoseconds,
      lastSeenAt,
    },
    Math.max(nowMs, Date.now()),
  );

  return deriveLiveness({
    endedAt: state.livestream?.record.endedAt,
    hasInitialResponse:
      state.websocketConnected ||
      state.profile !== null ||
      state.hasReceivedSegment ||
      state.livestream !== null,
    hasReceivedSegment: state.hasReceivedSegment,
    hasLivestream: state.livestream !== null,
    secondsSinceActivity,
  });
}
