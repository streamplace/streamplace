import { useLivestreamStoreOptional } from "../livestream-store";
import { usePlayerStore } from "../player-store";
import { useVideoStoreOptional } from "../video-store";

// Surfaces the human-readable title for whatever the player is currently
// showing, transparently switching between the live stream record and the
// VOD record based on the player's mode. This lets player UI stay generic:
// `const title = useTitle()` works the same in live and vod contexts.
//
// Reads from the *optional* store hooks so it's safe to call even when only
// one of the providers (LivestreamProvider / VideoProvider) is mounted.
// Returns undefined when no title is available yet (e.g. metadata still
// loading) so callers can supply their own fallback.
export function useTitle(): string | undefined {
  const mode = usePlayerStore((x) => x.mode);
  const liveTitle = useLivestreamStoreOptional(
    (x) => x.livestream?.record.title,
  );
  const vodTitle = useVideoStoreOptional((x) => x.video?.record.title);
  return mode === "vod" ? vodTitle : liveTitle;
}
