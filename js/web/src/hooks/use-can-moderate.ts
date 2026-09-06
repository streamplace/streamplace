import {
  moderationPermissionsFor,
  permissionRecordsFromListRecords,
} from "@/lib/moderation";
import { useSession } from "@/lib/session";
import type { LivestreamStore } from "@streamplace/core";
import { useEffect, useState } from "react";
import { useStore } from "zustand";

export interface UseCanModerateResult extends ReturnType<
  typeof moderationPermissionsFor
> {
  isLoading: boolean;
  error: string | null;
}

/**
 * Moderation capabilities of the logged-in viewer for the stream this store
 * is connected to. The streamer's delegation records are fetched once per
 * (agent, streamer) pair into the store; websocket permissionView pushes
 * keep the store current while the page is open.
 */
export function useCanModerate(store: LivestreamStore): UseCanModerateResult {
  const { pdsAgent, did } = useSession();
  const streamerDid = useStore(store, (s) => s.livestream?.author.did) ?? null;
  const records = useStore(store, (s) => s.moderationPermissions);
  const setModerationPermissions = useStore(
    store,
    (s) => s.setModerationPermissions,
  );
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    // Owners hold every permission by definition; don't spend a listRecords
    // call proving it.
    if (!pdsAgent?.did || !streamerDid || pdsAgent.did === streamerDid) {
      setModerationPermissions([]);
      setError(null);
      setIsLoading(false);
      return;
    }

    let cancelled = false;
    setIsLoading(true);
    (async () => {
      try {
        const result = await pdsAgent.com.atproto.repo.listRecords({
          repo: streamerDid,
          collection: "place.stream.moderation.permission",
          limit: 100,
        });
        if (cancelled) return;
        setModerationPermissions(
          permissionRecordsFromListRecords(result.data.records ?? []),
        );
        setError(null);
      } catch (e) {
        if (cancelled) return;
        setError(
          e instanceof Error
            ? e.message
            : "Failed to load moderation permissions",
        );
        setModerationPermissions([]);
      } finally {
        if (!cancelled) setIsLoading(false);
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [pdsAgent, streamerDid, setModerationPermissions]);

  return {
    ...moderationPermissionsFor(records, did, streamerDid),
    isLoading,
    error,
  };
}
