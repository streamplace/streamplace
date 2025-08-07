import {
  Chat,
  ChatBox,
  useLivestreamStore,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import emojiData from "assets/emoji-data.json";
import { useState } from "react";
import { Text, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";
const { flex, bg, r, borders, p, px, py, text, layout } = zero;

export default function ChatPanel() {
  // Get real data from hooks
  const isLive = useLiveUser();
  const chat = useLivestreamStore((x) => x.chat);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);

  const isConnected = ingestConnectionState === "connected";
  const canModerate = isLive && isConnected;

  // Track initial load timestamp (when component mounts)
  const [initialLoadTime] = useState(() => Date.now());

  // Calculate new messages per minute (received after initial load)
  const now = Date.now();
  const oneMinuteAgo = now - 60 * 1000;
  const newMessages =
    chat?.filter((msg) =>
      typeof msg.timestamp === "number"
        ? msg.timestamp > initialLoadTime && msg.timestamp > oneMinuteAgo
        : false,
    ) ?? [];
  const messagesPerMinute = newMessages.length;
  return (
    <View
      style={[
        flex.values[1],
        bg.neutral[900],
        borders.width.thin,
        borders.color.neutral[700],
        layout.flex.column,
        r.lg,
        { minWidth: 300, maxWidth: 600, flexShrink: 0 },
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          borders.bottom.width.thin,
          borders.bottom.color.neutral[700],
          p[4],
        ]}
      >
        <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
          Live Chat
        </Text>
        <View style={[layout.flex.row, layout.flex.alignCenter]}>
          <View
            style={[
              { width: 6, height: 6, borderRadius: 3 },
              isLive && isConnected ? bg.green[500] : bg.gray[500],
            ]}
          />
          <Text style={[text.gray[400], { fontSize: 12, marginLeft: 8 }]}>
            {messagesPerMinute} msg/min
          </Text>
        </View>
      </View>
      <View style={[flex.values[1], px[2], { minHeight: 0 }]}>
        <View style={[flex.values[1], { minHeight: 0 }]}>
          <Chat canModerate={canModerate} shownMessages={50} />
        </View>
        <View style={[{ flexShrink: 0 }]}>
          <ChatBox
            emojiData={emojiData}
            chatBoxStyle={[
              bg.gray[700],
              borders.width.thin,
              borders.color.gray[600],
              r.md,
              p[3],
              !isConnected && { opacity: 0.6 },
            ]}
          />
        </View>
      </View>
    </View>
  );
}
