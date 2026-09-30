import { useSession } from "@/lib/session";
import { useStore } from "@/lib/store";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

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
    queryFn: async ({ signal }): Promise<string | null> => {
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
      return data.follow?.uri ?? null;
    },
  });

  const toggle = async () => {
    if (
      !enabled ||
      !pdsAgent ||
      !subjectDID ||
      query.data === undefined ||
      query.isFetching ||
      query.isError ||
      pending
    ) {
      return;
    }

    setPending(true);
    try {
      await queryClient.cancelQueries({ queryKey });
      if (query.data) {
        await pdsAgent.deleteFollow(query.data);
        queryClient.setQueryData(queryKey, null);
      } else {
        const result = await pdsAgent.follow(subjectDID);
        // Use the write result rather than waiting for the node to index it.
        queryClient.setQueryData(queryKey, result.uri);
      }
    } finally {
      setPending(false);
    }
  };

  return {
    following: !!query.data,
    loading: enabled && (query.isPending || query.isFetching || pending),
    error: query.error,
    toggle,
  };
}
