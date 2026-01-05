import { Button, Icon, Text, zero } from "@streamplace/components";
import { Plus } from "lucide-react-native";
import React, { useEffect, useState } from "react";
import { View } from "react-native";
import { useStore } from "store";
import { useStreamplaceUrl } from "store/hooks";

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
  const streamplaceUrl = useStreamplaceUrl();
  const followUser = useStore((state) => state.followUser);
  const unfollowUser = useStore((state) => state.unfollowUser);

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
  }, [currentUserDID, streamerDID]);

  const handleFollow = async () => {
    setError(null);
    setIsFollowing(true); // Optimistic
    try {
      await followUser(streamerDID);
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
      await unfollowUser(streamerDID, followUri ?? undefined);
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
    <View
      style={[
        { flexDirection: "row" },
        { alignItems: "center" },
        zero.gap.all[2],
      ]}
    >
      <Button
        onPress={isFollowing ? handleUnfollow : handleFollow}
        variant={isFollowing ? "secondary" : "primary"}
        size="pill"
        width="min"
        disabled={isFollowing === null}
        loading={isFollowing === null}
        leftIcon={!isFollowing && <Icon icon={Plus} size="sm" />}
      >
        {isFollowing === null
          ? "Loading..."
          : isFollowing
            ? "Unfollow"
            : "Follow"}
      </Button>
      {error && <Text style={[{ color: "#c00" }, zero.ml[2]]}>{error}</Text>}
    </View>
  );
};

export default FollowButton;
