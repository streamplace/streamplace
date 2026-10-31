import { Badge, LiveBadge } from "@streamplace/components";
import { useLiveStatus } from "hooks/useLiveStatus";

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
  const { isLive, mode } = useLiveStatus();
  if (mode === "vod") return null;

  if (isLive) {
    return <LiveBadge />;
  }

  // Broadcaster still establishing signal (no fresh frames yet) — "connecting",
  // amber to agree with the connection HUD, never a bare "OFFLINE".
  if (broadcasting) {
    return <Badge variant="warning">CONNECTING</Badge>;
  }

  // Viewer side: the stream simply isn't live.
  return <Badge variant="neutral">OFFLINE</Badge>;
}
