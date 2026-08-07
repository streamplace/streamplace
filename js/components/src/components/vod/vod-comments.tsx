import { Image } from "expo-image";
import { useCallback, useEffect, useState } from "react";
import { ActivityIndicator, Pressable, View } from "react-native";
import type { place } from "streamplace";
import { spacing } from "../../lib/theme/tokens";
import {
  useDID,
  useOnNeedsLogin,
} from "../../streamplace-store/streamplace-store";
import { useTheme } from "../../ui";
import {
  useCreateVodComment,
  useGetVodComments,
} from "../../vod-store";
import { Button } from "../ui/button";
import { Text } from "../ui/text";
import { Textarea } from "../ui/textarea";

function timeAgo(iso?: string): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const secs = Math.max(0, (Date.now() - then) / 1000);
  const units: [number, string][] = [
    [31536000, "y"],
    [2592000, "mo"],
    [604800, "w"],
    [86400, "d"],
    [3600, "h"],
    [60, "m"],
  ];
  for (const [size, label] of units) {
    const v = Math.floor(secs / size);
    if (v >= 1) return `${v}${label} ago`;
  }
  return "now";
}

function CommentAvatar({ uri, size = 36 }: { uri?: string; size?: number }) {
  const { theme } = useTheme();
  return (
    <View
      style={{
        width: size,
        height: size,
        borderRadius: 999,
        overflow: "hidden",
        backgroundColor: theme.colors.surface2,
        flexShrink: 0,
      }}
    >
      {uri ? (
        <Image
          source={{ uri }}
          style={{ width: "100%", height: "100%" }}
          contentFit="cover"
        />
      ) : null}
    </View>
  );
}

export function VodComments({ videoUri }: { videoUri: string }) {
  const [comments, setComments] = useState<place.stream.vod.defs.CommentView[]>(
    [],
  );
  const [loading, setLoading] = useState(true);
  const [draft, setDraft] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [focused, setFocused] = useState(false);
  const [cursor, setCursor] = useState<string | undefined>();

  const getComments = useGetVodComments();
  const createComment = useCreateVodComment();
  const did = useDID();
  const onNeedsLogin = useOnNeedsLogin();
  const { theme } = useTheme();

  const loadComments = useCallback(
    async (nextCursor?: string) => {
      setLoading(true);
      try {
        const result = await getComments(videoUri, 50, nextCursor);
        setComments((prev) =>
          nextCursor ? [...prev, ...result.comments] : result.comments,
        );
        setCursor(result.cursor);
      } catch (e) {
        console.error("Failed to load comments", e);
      } finally {
        setLoading(false);
      }
    },
    [videoUri, getComments],
  );

  useEffect(() => {
    setCursor(undefined);
    loadComments();
  }, [videoUri]);

  const handleSubmit = useCallback(async () => {
    if (!did) {
      onNeedsLogin?.();
      return;
    }
    if (!draft.trim()) return;
    setSubmitting(true);
    try {
      await createComment({ text: draft.trim(), video: videoUri });
      setDraft("");
      setFocused(false);
      await loadComments();
    } catch (e) {
      console.error("Failed to submit comment", e);
    } finally {
      setSubmitting(false);
    }
  }, [did, onNeedsLogin, draft, videoUri, createComment, loadComments]);

  return (
    <View style={{ gap: spacing[5] }}>
      <Text weight="semibold" size="lg">
        {comments.length > 0 ? `${comments.length} ` : ""}Comments
      </Text>

      {/* Composer */}
      <View style={{ flexDirection: "row", gap: spacing[3] }}>
        <CommentAvatar size={36} />
        <View style={{ flex: 1, gap: spacing[2] }}>
          {!did ? (
            // Logged out: the box is a button that prompts sign-in (YouTube).
            <Pressable onPress={() => onNeedsLogin?.()}>
              <View pointerEvents="none">
                <Textarea
                  value=""
                  placeholder="Add a comment…"
                  multiline
                  editable={false}
                  // Non-editable but not visually disabled — it's a clickable
                  // sign-in prompt, so keep it full strength (override the
                  // Textarea's editable=false dimming).
                  style={{ minHeight: 40, opacity: 1 }}
                />
              </View>
            </Pressable>
          ) : (
            <Textarea
              value={draft}
              onChangeText={setDraft}
              onFocus={() => setFocused(true)}
              placeholder="Add a comment…"
              multiline
              editable={!submitting}
              style={{ minHeight: 40 }}
            />
          )}
          {did && (focused || draft.length > 0) ? (
            <View
              style={{
                flexDirection: "row",
                justifyContent: "flex-end",
                gap: spacing[2],
              }}
            >
              <Button
                variant="ghost"
                size="pill"
                width="min"
                onPress={() => {
                  setDraft("");
                  setFocused(false);
                }}
              >
                Cancel
              </Button>
              <Button
                variant="primary"
                size="pill"
                width="min"
                disabled={submitting || !draft.trim()}
                onPress={handleSubmit}
              >
                {submitting ? "Posting…" : "Comment"}
              </Button>
            </View>
          ) : null}
        </View>
      </View>

      {/* List */}
      {loading && comments.length === 0 ? (
        <ActivityIndicator color={theme.colors.text3} />
      ) : comments.length === 0 ? (
        <Text size="sm" style={{ color: theme.colors.text3 }}>
          No comments yet.
        </Text>
      ) : (
        <View style={{ gap: spacing[4] }}>
          {comments.map((item) => {
            const record = item.record as any;
            const author = item.author as any;
            return (
              <View
                key={item.uri}
                style={{ flexDirection: "row", gap: spacing[3] }}
              >
                <CommentAvatar uri={author?.avatar} size={36} />
                <View style={{ flex: 1, minWidth: 0, gap: 2 }}>
                  <View
                    style={{
                      flexDirection: "row",
                      alignItems: "center",
                      gap: spacing[2],
                    }}
                  >
                    <Text size="sm" weight="semibold">
                      {author?.handle ? `@${author.handle}` : author?.did}
                    </Text>
                    <Text size="xs" style={{ color: theme.colors.text3 }}>
                      {timeAgo(item.indexedAt)}
                    </Text>
                  </View>
                  <Text size="sm">{record?.text}</Text>
                  {item.likeCount > 0 ? (
                    <Text size="xs" style={{ color: theme.colors.text3 }}>
                      {item.likeCount} {item.likeCount === 1 ? "like" : "likes"}
                    </Text>
                  ) : null}
                </View>
              </View>
            );
          })}

          {cursor ? (
            <Pressable
              onPress={() => loadComments(cursor)}
              disabled={loading}
              style={{ alignSelf: "flex-start", paddingVertical: spacing[1] }}
            >
              <Text size="sm" weight="medium" color="primary">
                {loading ? "Loading…" : "Show more comments"}
              </Text>
            </Pressable>
          ) : null}
        </View>
      )}
    </View>
  );
}
