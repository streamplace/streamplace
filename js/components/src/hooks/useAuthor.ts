import { AppBskyActorDefs } from "@atproto/api";
import { useLivestreamStoreOptional } from "../livestream-store";
import { usePlayerStore } from "../player-store";
import { useVideoStoreOptional } from "../video-store";

// Surfaces the profile of whoever created what the player is currently
// showing — the streamer in live mode, the uploader in vod mode — switching
// on the player's mode. Like useTitle, this keeps player UI generic:
// `const author = useAuthor()` works the same in live and vod contexts.
//
// Reads from the *optional* store hooks so it's safe to call even when only
// one of the providers (LivestreamProvider / VideoProvider) is mounted.
// Returns null until the relevant metadata has loaded.
export function useAuthor(): AppBskyActorDefs.ProfileViewBasic | null {
  const mode = usePlayerStore((x) => x.mode);
  const liveProfile = useLivestreamStoreOptional((x) => x.profile);
  const vodAuthor = useVideoStoreOptional((x) => x.video?.author ?? null);
  return mode === "vod" ? vodAuthor : liveProfile;
}
