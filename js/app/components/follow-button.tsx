import { useRoute } from "@react-navigation/native";
import {
  Button,
  Icon,
  IconButton,
  Loader,
  Text,
  toast,
  useTheme,
  useTranslation,
  zero,
} from "@streamplace/components";
import { Check, Plus } from "lucide-react-native";
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
  iconOnly?: boolean;
}

const FollowButton: React.FC<FollowButtonProps> = ({
  streamerDID,
  currentUserDID,
  onFollowChange,
  iconOnly = false,
}) => {
  const [isFollowing, setIsFollowing] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [followUri, setFollowUri] = useState<string | null>(null);
  const [updating, setUpdating] = useState(false);
  const { theme } = useTheme();
  const { t } = useTranslation("common");
  const streamplaceUrl = useStreamplaceUrl();
  const route = useRoute();
  const openLoginModal = useStore((state) => state.openLoginModal);
  const followUser = useStore((state) => state.followUser);
  const unfollowUser = useStore((state) => state.unfollowUser);
  const following = !!currentUserDID && isFollowing === true;
  const checkingFollow = !!currentUserDID && isFollowing === null;

  // Fetch initial follow state using xrpc
  useEffect(() => {
    if (!currentUserDID || currentUserDID === streamerDID) return;
    let cancelled = false;

    const fetchFollowStatus = async () => {
      if (!currentUserDID || !streamerDID) return;

      setError(null);
      setIsFollowing(null);
      setFollowUri(null);
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

  useEffect(() => {
    if (iconOnly && currentUserDID && error)
      toast.show(t("follow-update-failed"), error, { variant: "error" });
  }, [iconOnly, currentUserDID, error, t]);

  if (currentUserDID === streamerDID) return null;

  const handleFollow = async () => {
    if (!currentUserDID) {
      openLoginModal({ name: route.name, params: route.params });
      return;
    }
    setError(null);
    setUpdating(true);
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
    } finally {
      setUpdating(false);
    }
  };

  const handleUnfollow = async () => {
    setError(null);
    setUpdating(true);
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
    } finally {
      setUpdating(false);
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
      {iconOnly ? (
        <IconButton
          accessibilityLabel={t(following ? "unfollow" : "follow")}
          accessibilityState={{ selected: following }}
          onPress={following ? handleUnfollow : handleFollow}
          loading={!!currentUserDID && (checkingFollow || updating)}
        >
          {checkingFollow ? (
            <Loader size="small" />
          ) : (
            <Icon icon={following ? Check : Plus} size="md" />
          )}
        </IconButton>
      ) : (
        <Button
          onPress={following ? handleUnfollow : handleFollow}
          variant={following ? "secondary" : "primary"}
          size="pill"
          width="min"
          disabled={checkingFollow}
          loading={!!currentUserDID && (checkingFollow || updating)}
          leftIcon={!following && <Icon icon={Plus} size="sm" />}
          hoverStyle={
            following ? { backgroundColor: theme.colors.danger } : undefined
          }
        >
          {checkingFollow
            ? t("loading")
            : following
              ? t("following")
              : t("follow")}
        </Button>
      )}
      {!iconOnly && currentUserDID && error && (
        <Text style={[{ color: theme.colors.danger }, zero.ml[2]]}>
          {error}
        </Text>
      )}
    </View>
  );
};

export default FollowButton;
