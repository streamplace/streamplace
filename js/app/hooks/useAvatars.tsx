import { ProfileViewDetailed } from "@atproto/api/dist/client/types/app/bsky/actor/defs";
import { useEffect, useRef } from "react";
import { useStore } from "store";
import { useCachedProfiles } from "store/hooks";

// Hack: Easy way to cache and get avatars
export default function useAvatars(dids: string[]) {
  const getProfiles = useStore((state) => state.getProfiles);
  const profiles: Record<string, ProfileViewDetailed> = useCachedProfiles();

  // Ask for each DID once. Callers pass a fresh array every render, and a DID
  // the Bluesky app view doesn't know (an account on another PDS with no
  // Bluesky profile) never lands in the cache, so keying on the array, or
  // re-asking for whatever is missing, refetches in a loop. A lookup that
  // failed outright is forgotten, so a later render asks again.
  const requested = useRef(new Set<string>());
  const missing = dids
    .filter((did) => !(did in profiles) && !requested.current.has(did))
    .join(",");

  useEffect(() => {
    if (!missing) return;
    const toFetch = missing.split(",");
    toFetch.forEach((did) => requested.current.add(did));
    getProfiles(toFetch).then((ok) => {
      if (!ok) toFetch.forEach((did) => requested.current.delete(did));
    });
  }, [missing]);

  return profiles;
}
