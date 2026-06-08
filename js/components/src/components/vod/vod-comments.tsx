import { useCallback, useEffect, useState } from "react";
import {
  ActivityIndicator,
  FlatList,
  TouchableOpacity,
  View,
} from "react-native";
import type { place } from "streamplace";
import {
  borders,
  flex,
  gap,
  layout,
  mb,
  mt,
  pt,
  px,
  py,
  r,
  useTheme,
} from "../../ui";
import { useCreateVodComment, useGetVodComments } from "../../vod-store";
import { Text } from "../ui/text";
import { Textarea } from "../ui/textarea";

export function VodComments({ videoUri }: { videoUri: string }) {
  const [comments, setComments] = useState<place.stream.vod.defs.CommentView[]>(
    [],
  );
  const [loading, setLoading] = useState(true);
  const [draft, setDraft] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [cursor, setCursor] = useState<string | undefined>();

  const getComments = useGetVodComments();
  const createComment = useCreateVodComment();
  const { theme } = useTheme();

  const loadComments = useCallback(async () => {
    setLoading(true);
    try {
      const result = await getComments(videoUri, 50, cursor);
      setComments((prev) =>
        cursor ? [...prev, ...result.comments] : result.comments,
      );
      setCursor(result.cursor);
    } catch (e) {
      console.error("Failed to load comments", e);
    } finally {
      setLoading(false);
    }
  }, [videoUri, cursor, getComments]);

  useEffect(() => {
    setCursor(undefined);
    loadComments();
  }, [videoUri]);

  const handleSubmit = useCallback(async () => {
    if (!draft.trim()) return;
    setSubmitting(true);
    try {
      await createComment({ text: draft.trim(), video: videoUri });
      setDraft("");
      setCursor(undefined);
      await loadComments();
    } catch (e) {
      console.error("Failed to submit comment", e);
    } finally {
      setSubmitting(false);
    }
  }, [draft, videoUri, createComment, loadComments]);

  const renderComment = useCallback(
    ({ item }: { item: place.stream.vod.defs.CommentView }) => {
      const record = item.record as any;
      return (
        <View
          style={[
            py[2],
            borders.bottom.width.thin,
            { borderBottomColor: theme.colors.border },
          ]}
        >
          <View style={[layout.flex.row, layout.flex.alignCenter, gap.all[2]]}>
            <Text weight="semibold">
              {item.author.handle || item.author.did}
            </Text>
            <Text size="sm" style={{ color: theme.colors.textMuted }}>
              {new Date(item.indexedAt).toLocaleDateString()}
            </Text>
          </View>
          <Text style={[mt[1]]}>{record.text}</Text>
          <View style={[layout.flex.row, gap.all[3], mt[1]]}>
            <Text size="sm" style={{ color: theme.colors.textMuted }}>
              {item.likeCount > 0 ? `${item.likeCount} likes` : ""}
            </Text>
          </View>
        </View>
      );
    },
    [theme],
  );

  return (
    <View style={flex.values[1]}>
      <Text weight="semibold" size="lg" style={[mb[3]]}>
        Comments
      </Text>

      {loading && comments.length === 0 ? (
        <ActivityIndicator />
      ) : (
        <FlatList
          data={comments}
          renderItem={renderComment}
          keyExtractor={(item) => item.uri}
          onEndReached={cursor ? loadComments : undefined}
          onEndReachedThreshold={0.5}
          style={flex.values[1]}
        />
      )}

      <View
        style={[
          layout.flex.row,
          gap.all[2],
          mt[3],
          pt[3],
          borders.top.width.thin,
          { borderTopColor: theme.colors.border },
        ]}
      >
        <Textarea
          value={draft}
          onChangeText={setDraft}
          placeholder="Add a comment..."
          multiline
          numberOfLines={2}
          editable={!submitting}
          enterKeyHint="send"
          submitBehavior="submit"
          onSubmitEditing={(e) => {
            e.preventDefault();
            handleSubmit();
          }}
          style={flex.values[1]}
        />
        <TouchableOpacity
          onPress={handleSubmit}
          disabled={submitting || !draft.trim()}
          style={[
            r.md,
            px[4],
            layout.flex.center,
            {
              backgroundColor: submitting
                ? theme.colors.muted
                : theme.colors.primary,
            },
          ]}
        >
          <Text style={{ color: "#fff" }} weight="semibold">
            {submitting ? "..." : "Post"}
          </Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}
