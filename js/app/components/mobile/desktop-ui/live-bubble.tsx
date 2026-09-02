import {
  Badge,
  LiveBadge,
  useLivestream,
  usePlayerStore,
  useSegment,
} from "@streamplace/components";
import { useMemo } from "react";

export function LiveBubble({
  /**
   * True when the current user is the one broadcasting (ingest active). A
   * broadcaster who just hit "Go live" but has no fresh frames yet is
   * *connecting*, not "offline" — showing OFFLINE contradicts the connection
   * HUD and the LIVE title badge.
   */
  broadcasting = false,
}: {
  broadcasting?: boolean;
}) {
  // are we actually live? (is the most recent segment <= 10 seconds old?)
  const seg = useSegment();

  const livestream = useLivestream();

  const segDate = useMemo(() => {
    return seg?.startTime ? new Date(seg.startTime) : undefined;
  }, [seg?.startTime]);

  const isLive = useMemo(() => {
    return segDate && Date.now() - segDate.getTime() <= 10 * 1000;
  }, [segDate]);

  const mode = usePlayerStore((x) => x.mode);
  if (mode === "vod") return null;

  // On air: segments are flowing AND the livestream record exists — the
  // design-system LIVE badge (live-red fill, pulsing dot).
  if (isLive && livestream) {
    return <LiveBadge />;
  }

  // Broadcaster still establishing signal (no fresh frames yet) — "connecting",
  // amber to agree with the connection HUD, never a bare "OFFLINE".
  if (broadcasting) {
    return <Badge variant="warning">CONNECTING</Badge>;
  }

  // Viewer side: the stream simply isn't live.
  return <Badge variant="neutral">{isLive ? "NOT LIVE" : "OFFLINE"}</Badge>;
}
