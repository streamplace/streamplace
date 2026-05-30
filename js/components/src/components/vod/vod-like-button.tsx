import { useCallback, useEffect, useState } from "react";
import { TouchableOpacity, View } from "react-native";
import { useDID } from "../../streamplace-store/streamplace-store";
import { gap, layout, p, useTheme } from "../../ui";
import {
  useCreateVodLike,
  useDeleteVodLike,
  useGetVodLikes,
} from "../../vod-store/vod-interactions";
import { Text } from "../ui/text";

export function VodLikeButton({ subjectUri }: { subjectUri: string }) {
  const [likeCount, setLikeCount] = useState(0);
  const [userLiked, setUserLiked] = useState(false);
  const [userLikeUri, setUserLikeUri] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const userDID = useDID();
  const getLikes = useGetVodLikes();
  const createLike = useCreateVodLike();
  const deleteLike = useDeleteVodLike();
  const { theme } = useTheme();

  const loadLikes = useCallback(async () => {
    try {
      const result = await getLikes(subjectUri, 50);
      setLikeCount(result.count);
      if (userDID && result.likes) {
        const myLike = result.likes.find((l) => l.author.did === userDID);
        setUserLiked(!!myLike);
        setUserLikeUri(myLike?.uri || null);
      }
    } catch (e) {
      console.error("Failed to load likes", e);
    }
  }, [subjectUri, getLikes, userDID]);

  useEffect(() => {
    loadLikes();
  }, [loadLikes]);

  const toggleLike = useCallback(async () => {
    setLoading(true);
    try {
      if (userLiked && userLikeUri) {
        await deleteLike(userLikeUri);
        setUserLiked(false);
        setUserLikeUri(null);
        setLikeCount((c) => Math.max(0, c - 1));
      } else {
        const result = await createLike(subjectUri);
        setUserLiked(true);
        setUserLikeUri(result.data.uri);
        setLikeCount((c) => c + 1);
      }
    } catch (e) {
      console.error("Failed to toggle like", e);
    } finally {
      setLoading(false);
    }
  }, [userLiked, userLikeUri, subjectUri, createLike, deleteLike]);

  return (
    <TouchableOpacity onPress={toggleLike} disabled={loading}>
      <View
        style={[layout.flex.row, layout.flex.alignCenter, gap.all[1], p[2]]}
      >
        <Text style={{ fontSize: 18 }}>
          {userLiked ? "\u2764\uFE0F" : "\u2661"}
        </Text>
        <Text size="sm" style={{ color: theme.colors.textMuted }}>
          {likeCount}
        </Text>
      </View>
    </TouchableOpacity>
  );
}
