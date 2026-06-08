import type { LivestreamStore } from "@streamplace/core";
import { useEffect, useState } from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";

const STALE_AFTER_SECONDS = 10;
const OFFLINE_AFTER_SECONDS = 15;

export type Liveness = "live" | "stale" | "offline" | "never-live";

export function useLivenessState(store: LivestreamStore): Liveness {
  const state = useStore(
    store,
    useShallow((s) => ({
      segment: s.segment,
      hasReceivedSegment: s.hasReceivedSegment,
      livestream: s.livestream,
    })),
  );

  const [secondsSinceSegment, setSecondsSinceSegment] = useState(0);

  useEffect(() => {
    if (!state.segment) return;
    setSecondsSinceSegment(0);
    const id = window.setInterval(() => {
      setSecondsSinceSegment((s) => s + 1);
    }, 1000);
    return () => window.clearInterval(id);
  }, [state.segment]);

  if (!state.hasReceivedSegment && !state.livestream) {
    return "never-live";
  }
  if (secondsSinceSegment >= OFFLINE_AFTER_SECONDS) return "offline";
  if (secondsSinceSegment >= STALE_AFTER_SECONDS) return "stale";
  return "live";
}
