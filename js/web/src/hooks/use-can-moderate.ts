import {
  listModerationPermissionRecords,
  moderationPermissionsFor,
  type ModerationPermissionRecord,
  type ModeratorRecord,
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

// Chat panel, stream info, and the pinned banner each mount this hook for
// the same stream; let them share one in-flight listRecords fetch per
// (agent, streamer) instead of stacking identical requests.
const inflight = new Map<string, Promise<ModeratorRecord[]>>();
const MAX_TIMEOUT = 2_147_483_647;

function moderationPermissionKey(record: ModerationPermissionRecord): string {
  return record.uri ?? JSON.stringify([record.moderator, record.createdAt]);
}

function mergePermissionChanges(
  permissions: ModeratorRecord[],
  changes: Map<string, ModerationPermissionRecord | null>,
): ModeratorRecord[] {
  const merged = new Map<string, ModeratorRecord>(
    permissions.map((record) => [moderationPermissionKey(record), record]),
  );
  for (const [key, record] of changes) {
    if (record) {
      merged.set(key, {
        ...record,
        rkey: record.uri?.split("/").pop() ?? "",
      });
    } else {
      merged.delete(key);
    }
  }
  return [...merged.values()];
}

/**
 * Moderation capabilities of the logged-in viewer for the stream this store
 * is connected to. The streamer's delegation records are fetched once per
 * (agent, streamer) pair into the store; websocket permissionView pushes
 * keep the store current while the page is open, including permission
 * revocations.
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
  const [now, setNow] = useState(() => Date.now());

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
    const key = `${pdsAgent.did}:${streamerDid}`;
    // Preserve websocket changes that arrive while the initial listRecords
    // snapshot is in flight, so a late response cannot undo a revocation.
    const changes = new Map<string, ModerationPermissionRecord | null>();
    let previousPermissions = store.getState().moderationPermissions;
    const unsubscribe = store.subscribe((state) => {
      const nextPermissions = state.moderationPermissions;
      if (nextPermissions === previousPermissions) return;

      const previousByKey = new Map(
        previousPermissions.map((record) => [
          moderationPermissionKey(record),
          record,
        ]),
      );
      const nextByKey = new Map(
        nextPermissions.map((record) => [
          moderationPermissionKey(record),
          record,
        ]),
      );
      for (const [permissionKey, record] of nextByKey) {
        if (previousByKey.get(permissionKey) !== record) {
          changes.set(permissionKey, record);
        }
      }
      for (const permissionKey of previousByKey.keys()) {
        if (!nextByKey.has(permissionKey)) changes.set(permissionKey, null);
      }
      previousPermissions = nextPermissions;
    });

    let request = inflight.get(key);
    if (!request) {
      request = listModerationPermissionRecords(pdsAgent, streamerDid).finally(
        () => inflight.delete(key),
      );
      inflight.set(key, request);
    }

    setIsLoading(true);
    request
      .then((permissions) => {
        if (cancelled) return;
        setModerationPermissions(mergePermissionChanges(permissions, changes));
        setError(null);
      })
      .catch((e) => {
        if (cancelled) return;
        setError(
          e instanceof Error
            ? e.message
            : "Failed to load moderation permissions",
        );
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, [pdsAgent, streamerDid, setModerationPermissions]);

  useEffect(() => {
    if (!did || did === streamerDid) return;

    let nextExpiration = Number.POSITIVE_INFINITY;
    for (const record of records) {
      if (record.moderator !== did || !record.expirationTime) continue;
      const expiration = Date.parse(record.expirationTime);
      if (expiration > now && expiration < nextExpiration) {
        nextExpiration = expiration;
      }
    }

    if (!Number.isFinite(nextExpiration)) return;

    const timeout = window.setTimeout(
      () => setNow(Date.now()),
      Math.min(nextExpiration - now, MAX_TIMEOUT),
    );
    return () => window.clearTimeout(timeout);
  }, [did, now, records, streamerDid]);

  return {
    ...moderationPermissionsFor(records, did, streamerDid),
    isLoading,
    error,
  };
}
