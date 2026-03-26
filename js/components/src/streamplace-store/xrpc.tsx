import { useMemo } from "react";
import { StreamplaceAgent } from "streamplace";
import { useStreamplaceStore, useUrl } from "./streamplace-store";

export function usePDSAgent(): StreamplaceAgent | null {
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);
  // oauthsession is
  // - undefined when loading
  // - null when logged out, and
  // - SessionManager when logged in
  return useMemo(() => {
    if (!oauthSession) {
      if (oauthSession === undefined) return null;
      // TODO: change once we allow unauthed requests + profile indexing
      // it's bluesky's AppView b/c otherwise we'd have goosewithpipe.jpg
      // showing up everywhere
      return new StreamplaceAgent("https://public.api.bsky.app"); // nodeUrl);
    }

    return new StreamplaceAgent(oauthSession);
  }, [oauthSession]);
}

// can be unauthed, but will always use the current node URL
export function usePossiblyUnauthedPDSAgent(): StreamplaceAgent | null {
  const nodeUrl = useUrl();
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);
  // oauthsession is
  // - undefined when loading
  // - null when logged out, and
  // - SessionManager when logged in
  return useMemo(() => {
    if (!oauthSession) {
      return new StreamplaceAgent(nodeUrl);
    }

    return new StreamplaceAgent(oauthSession);
  }, [oauthSession]);
}

// always returns an unauthenticated agent pointed at the public bluesky API
// probably should not be used in most places, but in case we have a bug it may be useful
export function useUnauthenticatedBlueskyAppViewAgent(): StreamplaceAgent {
  return useMemo(() => {
    return new StreamplaceAgent("https://public.api.bsky.app");
  }, []);
}
