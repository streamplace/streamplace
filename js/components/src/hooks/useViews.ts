import { useLivestreamStoreOptional } from "../livestream-store";
import { usePlayerStore } from "../player-store";
import { useVideoStoreOptional } from "../video-store";

// Surfaces the view count for whatever the player is currently showing,
// switching on the player's mode: the live concurrent viewer count in live
// mode, or the VOD's total accumulated view count (from VideoView.viewCounts)
// in vod mode. Returns null when the count isn't known yet.
export function useViews(): number | null {
  const mode = usePlayerStore((x) => x.mode);
  const liveViewers = useLivestreamStoreOptional((x) => x.viewers);
  const vodViews = useVideoStoreOptional(
    (x) => x.video?.viewCounts?.count ?? null,
  );
  return mode === "vod" ? vodViews : liveViewers;
}
