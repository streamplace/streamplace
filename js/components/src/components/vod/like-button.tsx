import { ThumbsUp } from "lucide-react-native";
import { useCallback, useEffect, useState } from "react";
import { TouchableOpacity, View } from "react-native";
import Animated, {
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { useDID } from "../../streamplace-store/streamplace-store";
import { gap, layout, useTheme } from "../../ui";
import { useCreateLike, useDeleteLike, useGetLikes } from "../../vod-store";
import { Text } from "../ui/text";

export function LikeButton({ subjectUri }: { subjectUri: string }) {
  const [likeCount, setLikeCount] = useState(0);
  const [userLiked, setUserLiked] = useState(false);
  const [userLikeUri, setUserLikeUri] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const userDID = useDID();
  const getLikes = useGetLikes();
  const createLike = useCreateLike();
  const deleteLike = useDeleteLike();
  const { theme } = useTheme();

  const scale = useSharedValue(1);

  const animatedStyle = useAnimatedStyle(() => ({
    transform: [{ scale: scale.value }],
  }));

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
    scale.value = withSpring(1.05, { stiffness: 500, damping: 10 }, () => {
      scale.value = withSpring(1, { stiffness: 500 });
    });
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
  }, [userLiked, userLikeUri, subjectUri, createLike, deleteLike, scale]);

  const heartColor = userLiked
    ? theme.colors.background
    : theme.colors.foreground;

  return (
    <TouchableOpacity onPress={toggleLike} disabled={loading}>
      <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
        <Animated.View style={animatedStyle}>
          <ThumbsUp color={heartColor} fill={userLiked ? heartColor : "none"} />
        </Animated.View>
        <Text size="sm">{likeCount}</Text>
      </View>
    </TouchableOpacity>
  );
}
