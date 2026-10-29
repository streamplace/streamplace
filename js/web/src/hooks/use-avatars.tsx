// Cache and batch-fetch profile data for a list of DIDs.
import { AppBskyActorDefs } from "@atproto/api";
import { useEffect, useMemo } from "react";
import { useStore } from "../lib/store";
import { useCachedProfiles } from "../lib/store/hooks";

export default function useAvatars(
  dids: string[],
): Record<string, AppBskyActorDefs.ProfileViewDetailed> {
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
