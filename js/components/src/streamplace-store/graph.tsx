import { AppBskyGraphFollow } from "@atproto/api";
import { useEffect, useState } from "react";
import { useStreamplaceStore } from "./streamplace-store";
import { usePDSAgent } from "./xrpc";

export function useCreateFollowRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const createFollow = async (subjectDID: string) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
      const record: AppBskyGraphFollow.Record = {
        $type: "app.bsky.graph.follow",
        subject: subjectDID,
        createdAt: new Date().toISOString(),
      };
      const result = await agent.com.atproto.repo.createRecord({
        repo: agent.did,
        collection: "app.bsky.graph.follow",
        record,
      });
      return result;
    } finally {
      setIsLoading(false);
    }
  };

  return { createFollow, isLoading };
}

export function useDeleteFollowRecord() {
  let agent = usePDSAgent();
  const [isLoading, setIsLoading] = useState(false);

  const deleteFollow = async (followRecordUri: string) => {
    if (!agent) {
      throw new Error("No PDS agent found");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    try {
      const result = await agent.com.atproto.repo.deleteRecord({
        repo: agent.did,
        collection: "app.bsky.graph.follow",
        rkey: followRecordUri.split("/").pop()!,
      });
      return result;
    } finally {
      setIsLoading(false);
    }
  };

  return { deleteFollow, isLoading };
}

interface GraphManagerState {
  isFollowing: boolean | null;
  followUri: string | null;
  isLoading: boolean;
  error: string | null;
}

interface GraphManagerActions {
  follow: () => Promise<void>;
  unfollow: () => Promise<void>;
  refresh: () => Promise<void>;
}

export function useGraphManager(
  subjectDID: string | null | undefined,
): GraphManagerState & GraphManagerActions {
  const agent = usePDSAgent();
  const [isFollowing, setIsFollowing] = useState<boolean | null>(null);
  const [followUri, setFollowUri] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const userDID = agent?.did;

  const streamplaceUrl = useStreamplaceStore((state) => state.url);

  const fetchFollowStatus = async () => {
    if (!userDID || !subjectDID || !streamplaceUrl) {
      setIsFollowing(null);
      setFollowUri(null);
      return;
    }

    setIsLoading(true);
    setError(null);
    try {
      const res = await fetch(
        `${streamplaceUrl}/xrpc/place.stream.graph.getFollowingUser?subjectDID=${encodeURIComponent(subjectDID)}&userDID=${encodeURIComponent(userDID)}`,
        {
          credentials: "include",
        },
      );

      if (!res.ok) {
        const errorText = await res.text();
        throw new Error(`Failed to fetch follow status: ${errorText}`);
      }

      const data = await res.json();

      if (data.follow) {
        setIsFollowing(true);
        setFollowUri(data.follow.uri);
      } else {
        setIsFollowing(false);
        setFollowUri(null);
      }
    } catch (err) {
      setError(
        `Could not determine follow state: ${err instanceof Error ? err.message : `Unknown error: ${err}`}`,
      );
      setIsFollowing(null);
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    if (!userDID || !subjectDID) {
      setIsFollowing(null);
      setFollowUri(null);
      setError(null);
      return;
    }

    fetchFollowStatus();
  }, [userDID, subjectDID, streamplaceUrl]);

  const follow = async () => {
    if (!agent || !subjectDID) {
      throw new Error("Cannot follow: not logged in or no subject DID");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    setIsLoading(true);
    setError(null);
    const previousState = isFollowing;
    setIsFollowing(true); // Optimistic

    try {
      const record: AppBskyGraphFollow.Record = {
        $type: "app.bsky.graph.follow",
        subject: subjectDID,
        createdAt: new Date().toISOString(),
      };
      const result = await agent.com.atproto.repo.createRecord({
        repo: agent.did,
        collection: "app.bsky.graph.follow",
        record,
      });
      setFollowUri(result.data.uri);
      setIsFollowing(true);
    } catch (err) {
      setIsFollowing(previousState);
      const errorMsg = `Failed to follow: ${err instanceof Error ? err.message : "Unknown error"}`;
      setError(errorMsg);
      throw new Error(errorMsg);
    } finally {
      setIsLoading(false);
    }
  };

  const unfollow = async () => {
    if (!agent || !subjectDID) {
      throw new Error("Cannot unfollow: not logged in or no subject DID");
    }

    if (!agent.did) {
      throw new Error("No user DID found, assuming not logged in");
    }

    if (!followUri) {
      throw new Error("Cannot unfollow: no follow URI found");
    }

    setIsLoading(true);
    setError(null);
    const previousState = isFollowing;
    const previousUri = followUri;
    setIsFollowing(false); // Optimistic
    setFollowUri(null);

    try {
      await agent.com.atproto.repo.deleteRecord({
        repo: agent.did,
        collection: "app.bsky.graph.follow",
        rkey: followUri.split("/").pop()!,
      });
      setIsFollowing(false);
      setFollowUri(null);
    } catch (err) {
      setIsFollowing(previousState);
      setFollowUri(previousUri);
      const errorMsg = `Failed to unfollow: ${err instanceof Error ? err.message : "Unknown error"}`;
      setError(errorMsg);
      throw new Error(errorMsg);
    } finally {
      setIsLoading(false);
    }
  };

  return {
    isFollowing,
    followUri,
    isLoading,
    error,
    follow,
    unfollow,
    refresh: fetchFollowStatus,
  };
}
