import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect, useMemo } from "react";
import { useProfileCache } from "../context/profile-cache";

export function useAvatars(
  dids: string[],
): Record<string, ProfileViewDetailed> {
  const { profiles, requestProfiles } = useProfileCache();

  useEffect(() => {
    requestProfiles(dids);
  }, [dids, requestProfiles]);

  return useMemo(
    () =>
      Object.fromEntries(
        dids.filter((d) => d in profiles).map((d) => [d, profiles[d]]),
      ),
    [dids, profiles],
  );
}
