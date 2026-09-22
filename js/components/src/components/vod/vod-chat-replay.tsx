import { MessageSquare } from "lucide-react-native";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  FlatList,
  NativeScrollEvent,
  NativeSyntheticEvent,
  View as RNView,
  StyleProp,
  ViewStyle,
} from "react-native";
import { ChatMessageViewHydrated } from "streamplace";
import { spacing } from "../../lib/theme/tokens";
import { usePlayerStore } from "../../player-store";
import { useVideoStore } from "../../video-store/video-store";
import { RenderChatMessage } from "../chat/chat-message";
import { ProfileCardProvider } from "../chat/user-profile-card";
import { Text, useTheme, View } from "../ui";

// The livestream's chat, replayed beside its recording: read-only, and
// paced by the player. A message appears when the playhead reaches the
// moment it was sent (createdAt - the recording's start), so the replay
// reads the way the stream did; seeking back takes messages away again.
// Nothing here posts, replies or moderates. Renders nothing for a video
// that was not recorded from a livestream.

const keyExtractor = (item: ChatMessageViewHydrated) => item.cid;

// Index of the first message sent after t: messages[0..idx) are visible.
function visibleCount(times: number[], t: number): number {
  let lo = 0;
  let hi = times.length;
  while (lo < hi) {
    const mid = (lo + hi) >> 1;
    if (times[mid] <= t) lo = mid + 1;
    else hi = mid;
  }
  return lo;
}

export function VodChatReplay({ style }: { style?: StyleProp<ViewStyle> }) {
  const replay = useVideoStore((x) => x.replay);
  const playTime = usePlayerStore((x) => x.playTime);
  const { theme } = useTheme();
  const listRef = useRef<FlatList<ChatMessageViewHydrated>>(null);
  const [pinned, setPinned] = useState(true);

  const times = useMemo(
    () =>
      (replay?.messages ?? []).map((m) =>
        new Date(m.record.createdAt).getTime(),
      ),
    [replay],
  );
  const count = replay
    ? visibleCount(times, replay.startedAt + playTime * 1000)
    : 0;
  const shown = useMemo(
    () => (replay ? replay.messages.slice(0, count) : []),
    [replay, count],
  );

  // Follow the newest message while the viewer is at the bottom, as live
  // chat does; a viewer who scrolled up to read stays where they are.
  useEffect(() => {
    if (pinned && shown.length > 0) {
      const t = setTimeout(
        () => listRef.current?.scrollToEnd({ animated: false }),
        0,
      );
      return () => clearTimeout(t);
    }
  }, [shown.length, pinned]);

  const onScroll = (e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const { contentOffset, contentSize, layoutMeasurement } = e.nativeEvent;
    const fromBottom =
      contentSize.height - (contentOffset.y + layoutMeasurement.height);
    setPinned(fromBottom < 40);
  };

  if (!replay) return null;

  return (
    <RNView
      style={[
        {
          borderWidth: 1,
          borderColor: theme.colors.borderSubtle,
          borderRadius: 12,
          backgroundColor: theme.colors.surface0,
          overflow: "hidden",
        },
        style,
      ]}
    >
      <View
        style={{
          flexDirection: "row",
          alignItems: "center",
          gap: spacing[2],
          paddingHorizontal: spacing[3],
          paddingVertical: spacing[2],
          borderBottomWidth: 1,
          borderBottomColor: theme.colors.borderSubtle,
        }}
      >
        <MessageSquare size={16} color={theme.colors.mutedForeground} />
        <Text size="sm" weight="semibold">
          Chat replay
        </Text>
        <RNView style={{ flex: 1 }} />
        <Text size="xs" color="muted">
          {shown.length} of {replay.messages.length}
        </Text>
      </View>
      {shown.length === 0 ? (
        <View
          style={{
            flex: 1,
            alignItems: "center",
            justifyContent: "center",
            padding: spacing[4],
          }}
        >
          <Text size="sm" color="muted" center>
            {replay.messages.length === 0
              ? "Nobody chatted during this stream."
              : "Messages appear as the stream reaches them."}
          </Text>
        </View>
      ) : (
        <ProfileCardProvider>
          <FlatList
            ref={listRef}
            style={{ flex: 1 }}
            contentContainerStyle={{
              paddingHorizontal: spacing[2],
              paddingVertical: spacing[2],
            }}
            data={shown}
            keyExtractor={keyExtractor}
            renderItem={({ item }) => (
              <RNView style={{ paddingVertical: 4 }}>
                <RenderChatMessage item={item} />
              </RNView>
            )}
            onScroll={onScroll}
            scrollEventThrottle={32}
            nestedScrollEnabled
            removeClippedSubviews
            maxToRenderPerBatch={20}
            initialNumToRender={30}
          />
        </ProfileCardProvider>
      )}
    </RNView>
  );
}
