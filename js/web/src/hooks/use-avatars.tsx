// Cache and batch-fetch profile data for a list of DIDs.
import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect, useMemo } from "react";
import { useStore } from "../lib/store";
import { useCachedProfiles } from "../lib/store/hooks";

export default function useAvatars(
  dids: string[],
): Record<string, ProfileViewDetailed> {
  const getProfiles = useStore((state) => state.getProfiles);
  const profiles = useCachedProfiles();

  const missingDids = useMemo(
    () => dids.filter((did) => !(did in profiles)),
    [dids, profiles],
  );

  useEffect(() => {
    if (missingDids.length > 0) {
      getProfiles(missingDids);
    }
  }, [missingDids, getProfiles]);

  return profiles;
}
