import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect, useMemo, useRef, useState } from "react";
import { usePDSAgent } from "../streamplace-store/xrpc";

export function useAvatars(
  dids: string[],
): Record<string, ProfileViewDetailed> {
  let agent = usePDSAgent();
  const [profiles, setProfiles] = useState<Record<string, ProfileViewDetailed>>(
    {},
  );
  const inFlight = useRef<Set<string>>(new Set());

  const missingDids = useMemo(
    () =>
      dids.filter((did) => !(did in profiles) && !inFlight.current.has(did)),
    [dids, profiles],
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
        setProfiles((prev) => ({ ...prev, ...newProfiles }));
      } catch (e) {
        console.error("Failed to fetch profiles", e);
      } finally {
        toFetch.forEach((did) => inFlight.current.delete(did));
      }
    };

    fetchProfiles();
  }, [missingDids, agent]);

  return profiles;
}
