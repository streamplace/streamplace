import { useCallback, useEffect, useState } from "react";
import { PlaceStreamBetaRequest } from "streamplace";
import { useDID } from "../streamplace-store/streamplace-store";
import {
  usePDSAgent,
  usePossiblyUnauthedPDSAgent,
} from "../streamplace-store/xrpc";

export type BetaStatus = "granted" | "requested" | "none";

// Reports the logged-in account's access status for a named beta feature
// (e.g. "vod") via place.stream.beta.getStatus. The DID is passed explicitly
// so the query works against the unauthenticated node agent. Returns
// status=null while loading or when logged out.
export function useBetaStatus(feature: string) {
  const agent = usePossiblyUnauthedPDSAgent();
  const did = useDID();
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
      const res = await agent.place.stream.beta.getStatus({ feature, did });
      setStatus(res.data.status as BetaStatus);
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

// Publishes a place.stream.beta.request record in the caller's own repo,
// asking for access to the named feature. The requesting account is the
// record's authority, so no subject field is needed.
export function useRequestBetaAccess(feature: string) {
  const agent = usePDSAgent();
  const did = useDID();
  return useCallback(async () => {
    if (!agent || !did) {
      throw new Error("not logged in");
    }
    const record: PlaceStreamBetaRequest.Record = {
      $type: "place.stream.beta.request",
      feature,
      createdAt: new Date().toISOString(),
    };
    return await agent.com.atproto.repo.createRecord({
      repo: did,
      collection: "place.stream.beta.request",
      record,
    });
  }, [agent, did, feature]);
}
