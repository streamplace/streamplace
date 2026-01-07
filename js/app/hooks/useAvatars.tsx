import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect, useMemo } from "react";
import { useStore } from "store";
import { useCachedProfiles } from "store/hooks";

// Hack: Easy way to cache and get avatars
export default function useAvatars(dids: string[]) {
  const getProfiles = useStore((state) => state.getProfiles);
  const profiles: Record<string, ProfileViewDetailed> = useCachedProfiles();

  const missingDids = useMemo(
    () => dids.filter((did) => !(did in profiles)),
    [dids, profiles],
  );

  useEffect(() => {
    if (missingDids.length > 0) {
      console.log("Fetching profiles for DIDs:", missingDids);
      getProfiles(missingDids).then((e) => console.log("ok", e));
    }
  }, [missingDids]);

  return profiles;
}
