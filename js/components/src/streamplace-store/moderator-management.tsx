import { useCallback, useEffect, useState } from "react";
import { PlaceStreamModerationPermission } from "streamplace";
import { usePDSAgent } from "./xrpc";

interface ModeratorRecord {
  uri: string;
  cid: string;
  value: PlaceStreamModerationPermission.Record;
  rkey: string;
}

interface ListModeratorsResult {
  moderators: ModeratorRecord[];
  isLoading: boolean;
  error: string | null;
  refresh: () => void;
}

/**
 * Hook to list all moderators for the current user's stream.
 * Fetches place.stream.moderation.permission records from the user's repo.
 */
export function useListModerators(): ListModeratorsResult {
  const agent = usePDSAgent();
  const [moderators, setModerators] = useState<ModeratorRecord[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  const refresh = useCallback(() => {
    setRefreshTrigger((prev) => prev + 1);
  }, []);

  useEffect(() => {
    if (!agent?.did) {
      setModerators([]);
      setError(null);
      return;
    }

    const fetchModerators = async () => {
      setIsLoading(true);
      setError(null);

      try {
        const result = await agent.place.stream.moderation.permission.list({
          repo: agent.did!,
        });

        const records = result.records.map((record: any) => {
          const rkey = record.uri.split("/").pop();
          return {
            uri: record.uri,
            cid: record.cid || "",
            value: record.value,
            rkey: rkey || "",
          };
        });

        setModerators(records);
      } catch (err) {
        setError(
          `Failed to fetch moderators: ${err instanceof Error ? err.message : "Unknown error"}`,
        );
        setModerators([]);
      } finally {
        setIsLoading(false);
      }
    };

    fetchModerators();
  }, [agent?.did, refreshTrigger]);

  return {
    moderators,
    isLoading,
    error,
    refresh,
  };
}

interface AddModeratorParams {
  moderatorDID: string;
  permissions: ("ban" | "hide" | "livestream.manage")[];
  expirationTime?: string; // ISO 8601 datetime string
}

/**
 * Hook to add a new moderator.
 * Creates a place.stream.moderation.permission record in the current user's repo.
 */
export function useAddModerator() {
  const agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const addModerator = async (params: AddModeratorParams) => {
    if (!agent?.did) {
      throw new Error("Not logged in");
    }

    if (params.permissions.length === 0) {
      throw new Error("At least one permission must be selected");
    }

    setIsLoading(true);
    try {
      // Resolve handle to DID if needed
      let moderatorDID = params.moderatorDID.trim();
      if (moderatorDID.startsWith("@")) {
        // It's a handle, resolve to DID
        const resolved = await agent.com.atproto.identity.resolveHandle({
          handle: moderatorDID.substring(1), // Remove @
        });
        moderatorDID = resolved.data.did;
      } else if (!moderatorDID.startsWith("did:")) {
        // Try to resolve as handle without @
        try {
          const resolved = await agent.com.atproto.identity.resolveHandle({
            handle: moderatorDID,
          });
          moderatorDID = resolved.data.did;
        } catch (e) {
          // If resolution fails, assume it's already a DID
          throw new Error(
            `Invalid DID or handle: ${moderatorDID}. Please use a valid DID (did:plc:...) or handle (@handle.bsky.social)`,
          );
        }
      }

      const record: PlaceStreamModerationPermission.Record = {
        $type: "place.stream.moderation.permission",
        moderator: moderatorDID,
        permissions: params.permissions,
        createdAt: new Date().toISOString(),
      };

      if (params.expirationTime) {
        (record as any).expirationTime = params.expirationTime;
      }

      const result = await agent.place.stream.moderation.permission.create(
        { repo: agent.did },
        record,
      );

      return result;
    } finally {
      setIsLoading(false);
    }
  };

  return { addModerator, isLoading };
}

/**
 * Hook to remove a moderator.
 * Deletes a place.stream.moderation.permission record by its rkey.
 */
export function useRemoveModerator() {
  const agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const removeModerator = async (rkey: string) => {
    if (!agent?.did) {
      throw new Error("Not logged in");
    }

    setIsLoading(true);
    try {
      await agent.place.stream.moderation.permission.delete({
        repo: agent.did,
        rkey,
      });
    } finally {
      setIsLoading(false);
    }
  };

  return { removeModerator, isLoading };
}
