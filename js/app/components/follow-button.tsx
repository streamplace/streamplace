import { Check, Plus } from "@tamagui/lucide-icons";
import React, { useEffect, useState } from "react";
import { Button, Text, View } from "tamagui";
import { followUser, unfollowUser } from "../features/bluesky/blueskySlice";
import { selectStreamplace } from "../features/streamplace/streamplaceSlice";
import { useAppDispatch, useAppSelector } from "../store/hooks";

/**
 * FollowButton component for following/unfollowing a streamer.
 *
 * Props:
 * - streamerDID: string — The DID of the streamer to follow/unfollow
 * - currentUserDID?: string — The DID of the current user (optional)
 * - onFollowChange?: (isFollowing: boolean) => void — Optional callback when follow state changes
 */
interface FollowButtonProps {
  streamerDID: string;
  currentUserDID?: string;
  onFollowChange?: (isFollowing: boolean) => void;
}

const FollowButton: React.FC<FollowButtonProps> = ({
  streamerDID,
  currentUserDID,
  onFollowChange,
}) => {
  const [isFollowing, setIsFollowing] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [followUri, setFollowUri] = useState<string | null>(null);
  const { url: streamplaceUrl } = useAppSelector(selectStreamplace);
  const dispatch = useAppDispatch();

  // Hide button if not logged in or viewing own stream
  if (!currentUserDID || currentUserDID === streamerDID) return null;

  // Fetch initial follow state using xrpc
  useEffect(() => {
    let cancelled = false;

    const fetchFollowStatus = async () => {
      if (!currentUserDID || !streamerDID) return;

      setError(null);
      try {
        const res = await fetch(
          `${streamplaceUrl}/xrpc/place.stream.graph.getFollowingUser?subjectDID=${encodeURIComponent(streamerDID)}&userDID=${encodeURIComponent(currentUserDID)}`,
          {
            credentials: "include",
          },
        );

        if (!res.ok) {
          const errorText = await res.text();
          throw new Error(`Failed to fetch follow status: ${errorText}`);
        }

        const data = await res.json();
        if (cancelled) return;

        if (data.follow) {
          setIsFollowing(true);
          setFollowUri(data.follow.uri);
        } else {
          setIsFollowing(false);
          setFollowUri(null);
        }
      } catch (err) {
        if (!cancelled) setError("Could not determine follow state");
      }
    };

    fetchFollowStatus();
    return () => {
      cancelled = true;
    };
  }, [currentUserDID, streamerDID, streamplaceUrl]);

  const handleFollow = async () => {
    setError(null);
    setIsFollowing(true); // Optimistic
    try {
      await dispatch(followUser(streamerDID)).unwrap();
      setIsFollowing(true);
      onFollowChange?.(true);
    } catch (err) {
      setIsFollowing(false);
      setError(
        `Failed to follow: ${err instanceof Error ? err.message : "Unknown error"}`,
      );
    }
  };

  const handleUnfollow = async () => {
    setError(null);
    setIsFollowing(false); // Optimistic
    try {
      await dispatch(
        unfollowUser({
          subjectDID: streamerDID,
          ...(followUri ? { followUri } : {}),
        }),
      ).unwrap();
      setIsFollowing(false);
      setFollowUri(null);
      onFollowChange?.(false);
    } catch (err) {
      setIsFollowing(true);
      setError(
        `Failed to unfollow: ${err instanceof Error ? err.message : "Unknown error"}`,
      );
    }
  };

  return (
    <View flexDirection="row" alignItems="center" gap={8}>
      {isFollowing === null ? (
        // Skeleton loader to prevent layout shift
        <Button backgroundColor="transparent" disabled>
          &nbsp;
        </Button>
      ) : isFollowing ? (
        <Button
          backgroundColor="transparent"
          onPress={handleUnfollow}
          aria-label="Following"
          icon={Check}
        >
          Following
        </Button>
      ) : (
        <Button
          backgroundColor="transparent"
          onPress={handleFollow}
          aria-label="Follow"
          icon={Plus}
        >
          Follow
        </Button>
      )}
      {error && (
        <Text color="#c00" marginLeft={8}>
          {error}
        </Text>
      )}
    </View>
  );
};

export default FollowButton;
