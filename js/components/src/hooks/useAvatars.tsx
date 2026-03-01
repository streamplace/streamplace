import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect, useMemo } from "react";
import { useProfileCache } from "../context/profile-cache";
import { usePDSAgent } from "../streamplace-store/xrpc";

export function useAvatars(
  dids: string[],
): Record<string, ProfileViewDetailed> {
  const agent = usePDSAgent();
  const { profiles, inFlight, addProfiles } = useProfileCache();

  const missingDids = useMemo(
    () =>
      dids.filter((did) => !(did in profiles) && !inFlight.current.has(did)),
    [dids, profiles, inFlight],
  );

  useEffect(() => {
    if (missingDids.length === 0 || !agent) return;
    const toFetch = missingDids.slice(0, 25);
    toFetch.forEach((did) => inFlight.current.add(did));

    const fetchProfiles = async () => {
      try {
        const result = await agent.getProfiles({ actors: toFetch });
        const newProfiles: Record<string, ProfileViewDetailed> = {};
        result.data.profiles.forEach((p) => {
          newProfiles[p.did] = p;
        });
        addProfiles(newProfiles);
      } catch (e) {
        console.error("Failed to fetch profiles", e);
      } finally {
        toFetch.forEach((did) => inFlight.current.delete(did));
      }
    };

    fetchProfiles();
  }, [missingDids, agent]);

  return useMemo(
    () =>
      Object.fromEntries(
        dids.filter((d) => d in profiles).map((d) => [d, profiles[d]]),
      ),
    [dids, profiles],
  );
}
