import { useMemo } from "react";
import { StreamplaceAgent } from "streamplace";
import { useStreamplaceStore } from ".";

export function usePDSAgent(): StreamplaceAgent | null {
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);

  return useMemo(() => {
    if (!oauthSession) {
      return null;
    }

    return new StreamplaceAgent(oauthSession);
  }, [oauthSession]);
}
