import { StreamplaceAgent } from "streamplace";
import { useStreamplaceStore } from "../streamplace-store";

export function usePDSAgent(): StreamplaceAgent {
  const oauthSession = useStreamplaceStore((state) => state.oauthSession);

  if (!oauthSession) {
    throw new Error("No OAuth session found");
  }

  return new StreamplaceAgent(oauthSession);
}
