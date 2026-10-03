// Cache and batch-fetch profile data for a list of DIDs.
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect } from "react";
import { useStore } from "../lib/store";
import { useCachedProfiles } from "../lib/store/hooks";

export default function useAvatars(
  dids: string[],
): Record<string, ProfileViewDetailed> {
  const getProfiles = useStore((state) => state.getProfiles);
  const profiles = useCachedProfiles();

  // Callers commonly create a new DID array on each playback render. Key
  // requests by their contents, not by array/cache identity; Bluesky may
  // legitimately omit Streamplace-only actors from a successful response.
  const missingKey = JSON.stringify(
    [...new Set(dids.filter((did) => !(did in profiles)))].sort(),
  );
  useEffect(() => {
    const missingDids: string[] = JSON.parse(missingKey);
    if (missingDids.length > 0) {
      getProfiles(missingDids);
    }
  }, [missingKey, getProfiles]);

  return profiles;
}
