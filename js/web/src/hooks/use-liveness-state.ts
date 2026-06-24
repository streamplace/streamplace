import type { LivestreamStore } from "@streamplace/core";
import { useEffect, useState } from "react";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";

// "stale" fires when local playback hasn't received a segment in a
// while — typically a network blip. Short by design so the user sees
// a "reconnecting" overlay quickly when the stream is hiccuping.
const STALE_AFTER_SECONDS = 10;

// "offline" is the "the stream is done" signal. Driven by local
// segment age: if no segment has arrived in this window, we treat
// the stream as offline.
//
// We initially tried to drive this off livestream.record.lastSeenAt
// (the server's view of "last segment submitted"), but the WebSocket
// only delivers the livestream record on connect — the server's
// periodic 30s updates to lastSeenAt aren't pushed to clients, so
// the client-side lastSeenAt is frozen at connect time and goes
// stale while the streamer is still active. Local segment age has
// the same 0–30s lag as lastSeenAt would (segments arrive via the
// same WebSocket) without the staleness.
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

  // Hard offline: the streamer explicitly ended the stream.
  if (state.livestream?.record.endedAt) {
    return "offline";
  }

  if (!state.hasReceivedSegment && !state.livestream) {
    return "never-live";
  }

  // Local segment age doubles as a server signal — if no segment has
  // arrived in OFFLINE_AFTER_SECONDS, the server isn't pushing any.
  if (secondsSinceSegment >= OFFLINE_AFTER_SECONDS) return "offline";
  if (secondsSinceSegment >= STALE_AFTER_SECONDS) return "stale";
  return "live";
}
