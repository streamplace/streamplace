import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

type FollowStatus = {
  uri: string | null;
  reconcileUntil: number | null;
};

const FOLLOW_RECONCILE_WINDOW_MS = 60_000;

export function useFollow(subjectDID: string | undefined) {
  const { did, pdsAgent } = useSession();
  const node = useStore((state) => state.url);
  const queryClient = useQueryClient();
  const [pending, setPending] = useState(false);
  const enabled = !!pdsAgent && !!did && !!subjectDID && did !== subjectDID;
  const queryKey = ["follow", node, did, subjectDID];
  const query = useQuery({
    queryKey,
    enabled,
    queryFn: async ({ signal }): Promise<FollowStatus> => {
      const params = new URLSearchParams({
        userDID: did!,
        subjectDID: subjectDID!,
      });
      const response = await fetch(
        `${node}/xrpc/place.stream.graph.getFollowingUser?${params}`,
        { credentials: "include", signal },
      );
      if (!response.ok) throw new Error("Failed to load follow status");
      const data: { follow?: { uri: string } } = await response.json();
      const indexedURI = data.follow?.uri ?? null;
      const localStatus = queryClient.getQueryData<FollowStatus>(queryKey);
      if (
        localStatus?.reconcileUntil &&
        Date.now() < localStatus.reconcileUntil &&
        localStatus.uri !== indexedURI
      ) {
        return localStatus;
      }
      return { uri: indexedURI, reconcileUntil: null };
    },
    refetchInterval: (query) =>
      query.state.data?.reconcileUntil &&
      Date.now() < query.state.data.reconcileUntil
        ? 10_000
        : false,
  });

  const toggle = async () => {
    if (
      !enabled ||
      !pdsAgent ||
      !subjectDID ||
      query.data === undefined ||
      pending
    ) {
      return;
    }

    setPending(true);
    try {
      await queryClient.cancelQueries({ queryKey });
      if (query.data.uri) {
        await pdsAgent.deleteFollow(query.data.uri);
        queryClient.setQueryData<FollowStatus>(queryKey, {
          uri: null,
          reconcileUntil: Date.now() + FOLLOW_RECONCILE_WINDOW_MS,
        });
      } else {
        const result = await pdsAgent.follow(subjectDID);
        // Use the write result rather than waiting for the node to index it.
        queryClient.setQueryData<FollowStatus>(queryKey, {
          uri: result.uri,
          reconcileUntil: Date.now() + FOLLOW_RECONCILE_WINDOW_MS,
        });
      }
    } finally {
      setPending(false);
    }
  };

  return {
    following: !!query.data?.uri,
    loading: enabled && (query.isPending || pending),
    error: query.error,
    toggle,
  };
}
