import { useMemo } from "react";
import { StreamplaceAgent } from "streamplace";
import { useStreamplaceStore } from ".";

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
