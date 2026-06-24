import type { LivestreamStore } from "@streamplace/core";
import { useEffect, useState } from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";

// "stale" fires when local playback hasn't received a segment in a
// while — typically a network blip. Short by design so the user sees
// a "reconnecting" overlay quickly when the stream is hiccuping.
const STALE_AFTER_SECONDS = 10;

// "offline" is the authoritative "the stream is done" signal, derived
// from the server's lastSeenAt on the livestream record (updated by
// the director every ~30s whenever a segment is ingested). 5 minutes
// gives transient stalls a chance to recover without falsely flipping
// to the offline page. As a fallback, if the livestream record's
// lastSeenAt isn't available, we use the local segment age — covers
// the case where the WebSocket hasn't delivered the record yet but
// segments stopped flowing long ago.
const OFFLINE_AFTER_SECONDS = 300;

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

  // The lastSeenAt-based offline check needs a re-render driver so it
  // progresses over time when no segments are arriving. Run a 1s tick
  // while a livestream is being tracked; clean up when it goes away.
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!state.livestream?.record.lastSeenAt) return;
    setNow(Date.now());
    const id = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(id);
  }, [state.livestream?.record.lastSeenAt]);

  // Hard offline: the streamer explicitly ended the stream.
  if (state.livestream?.record.endedAt) {
    return "offline";
  }

  // Authoritative offline: server hasn't seen a segment in 5m.
  const lastSeenAt = state.livestream?.record.lastSeenAt;
  if (lastSeenAt) {
    const secondsSinceLastSeen = (now - new Date(lastSeenAt).getTime()) / 1000;
    if (secondsSinceLastSeen >= OFFLINE_AFTER_SECONDS) {
      return "offline";
    }
  }

  if (!state.hasReceivedSegment && !state.livestream) {
    return "never-live";
  }

  // Stale is purely about local playback freshness — the server
  // thinks the stream is fine but this client hasn't seen a segment
  // in a while. Reset to live as soon as a new segment arrives.
  if (secondsSinceSegment >= STALE_AFTER_SECONDS) return "stale";
  return "live";
}
