import { useEffect, useState } from "react";
import { place } from "streamplace";
import { useLivestreamStore } from "../livestream-store/livestream-store";
import { usePDSAgent } from "./xrpc";

export interface ModerationPermissions {
  canBan: boolean;
  canHide: boolean;
  canPin: boolean;
  canManageLivestream: boolean;
  isOwner: boolean;
  isLoading: boolean;
  error: string | null;
}

/**
 * Hook to check if the current user can moderate for a given streamer.
 * Returns permission flags based on:
 * - Owner: full permissions if userDID === streamerDID
 * - Delegated: permissions from place.stream.moderation.permission records
 */
export function useCanModerate(
  streamerDID: string | null | undefined,
): ModerationPermissions {
  const agent = usePDSAgent();
  const userDID = agent?.did;

  // Get moderation permissions from livestream store (updated via WebSocket)
  const moderationPermissions = useLivestreamStore(
    (state) => state.moderationPermissions,
  );
  const setModerationPermissions = useLivestreamStore(
    (state) => state.setModerationPermissions,
  );

  const [isOwner, setIsOwner] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!userDID || !streamerDID) {
      setModerationPermissions([]);
      setIsOwner(false);
      setError(null);
      return;
    }

    // If user is the streamer, they have full permissions
    if (userDID === streamerDID) {
      setIsOwner(true);
      setModerationPermissions([]); // Not needed for owner
      setIsLoading(false);
      setError(null);
      return;
    }

    // Otherwise, fetch delegation records from the streamer's repo
    // This initial fetch populates the store, then WebSocket updates will keep it in sync
    const fetchDelegation = async () => {
      if (!agent) {
        setModerationPermissions([]);
        setIsLoading(false);
        return;
      }

      setIsLoading(true);
      setError(null);
      setIsOwner(false);

      try {
        // Use authenticated agent to list permission records from the streamer's repo
        const result = await agent.client.list(
          place.stream.moderation.permission,
          {
            repo: streamerDID as any,
            limit: 100,
          },
        );

        const records = result.records || [];
        const permissionRecords: place.stream.moderation.permission.Main[] =
          records
            .map((r: { value: any }) => r.value)
            .filter(
              (v: any) => v && v.$type === "place.stream.moderation.permission",
            );

        // Store all permissions in the livestream store
        // WebSocket updates will keep this in sync
        setModerationPermissions(permissionRecords);
      } catch (err) {
        console.error("[useCanModerate] Error fetching permissions:", err);
        setError(
          `Could not fetch moderation permissions: ${err instanceof Error ? err.message : `Unknown error: ${err}`}`,
        );
        setModerationPermissions([]);
      } finally {
        setIsLoading(false);
      }
    };

    // Fetch immediately on mount or when dependencies change
    fetchDelegation();
  }, [userDID, streamerDID, agent, setModerationPermissions]);

  // If permissions were cleared (e.g., due to deletion), trigger a refetch
  useEffect(() => {
    // If permissions were cleared and we're not the owner, refetch
    if (
      moderationPermissions.length === 0 &&
      !isOwner &&
      userDID &&
      streamerDID &&
      agent
    ) {
      const fetchDelegation = async () => {
        try {
          const result = await agent.client.list(
            place.stream.moderation.permission,
            {
              repo: streamerDID as any,
              limit: 100,
            },
          );

          const records = result.records || [];
          const permissionRecords: place.stream.moderation.permission.Main[] =
            records
              .map((r: { value: any }) => r.value)
              .filter(
                (v: any) =>
                  v && v.$type === "place.stream.moderation.permission",
              );

          setModerationPermissions(permissionRecords);
        } catch (err) {
          console.error("[useCanModerate] Error refetching permissions:", err);
        }
      };

      // Small delay to avoid rapid refetches
      const timeout = setTimeout(fetchDelegation, 100);
      return () => clearTimeout(timeout);
    }
  }, [
    moderationPermissions.length,
    isOwner,
    userDID,
    streamerDID,
    agent,
    setModerationPermissions,
  ]);

  // Find ALL delegation records for this moderator and merge their permissions
  const delegations = moderationPermissions.filter(
    (perm) => perm.moderator === userDID,
  );

  // Merge permissions from all delegation records for this moderator
  const permissions: string[] = delegations.reduce(
    (acc: string[], delegation) => {
      // Check if delegation has expired
      if (delegation.expirationTime) {
        const expiration = new Date(delegation.expirationTime);
        if (new Date() > expiration) {
          return acc; // Skip expired delegations
        }
      }

      // Add all permissions from this delegation, avoiding duplicates
      const delegationPerms = delegation.permissions || [];
      for (const perm of delegationPerms) {
        if (!acc.includes(perm)) {
          acc.push(perm);
        }
      }
      return acc;
    },
    [],
  );

  return {
    canBan: isOwner || permissions.includes("ban"),
    canHide: isOwner || permissions.includes("hide"),
    canPin: isOwner || permissions.includes("message.pin"),
    canManageLivestream: isOwner || permissions.includes("livestream.manage"),
    isOwner,
    isLoading,
    error,
  };
}
