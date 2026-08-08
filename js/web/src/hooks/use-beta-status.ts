// Reports the logged-in account's access status for a named beta feature
// (e.g. "vod") via place.stream.beta.getStatus. Ported from
// js/components/src/hooks/useBeta.ts. The DID is passed explicitly so the
// query works against the unauthenticated node agent.
import { useCallback, useEffect, useState } from "react";
import { place } from "streamplace";
import { useStore } from "../lib/store";
import { useOAuthSession, usePDSAgent } from "../lib/store/hooks";

export type BetaStatus = "granted" | "requested" | "none";

export function useBetaStatus(feature: string) {
  const authedAgent = usePDSAgent();
  const anonAgent = useStore((s) => s.anonPDSAgent);
  const oauthSession = useOAuthSession();
  const did = oauthSession?.did ?? null;
  const agent = authedAgent ?? anonAgent;

  const [status, setStatus] = useState<BetaStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refetch = useCallback(async () => {
    if (!agent || !did) {
      setStatus(null);
      setLoading(false);
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const res = await agent.client.call(place.stream.beta.getStatus, {
        feature,
        did: did as any,
      });
      setStatus(res.status as BetaStatus);
    } catch (e: any) {
      console.error("error fetching beta status", e);
      setError(e?.message ?? "failed to load beta status");
    } finally {
      setLoading(false);
    }
  }, [agent, did, feature]);

  useEffect(() => {
    refetch();
  }, [refetch]);

  // Optimistically reflect a just-published request without waiting for the
  // firehose to index it and getStatus to catch up.
  const markRequested = useCallback(() => setStatus("requested"), []);

  return { status, loading, error, refetch, markRequested, did };
}
