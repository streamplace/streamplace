import {
  Chat,
  ChatBox,
  useLivestreamStore,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import emojiData from "assets/emoji-data.json";
import React, { useEffect, useState } from "react";
import { Text, View } from "react-native";
import { useLiveUser } from "../../hooks/useLiveUser";

const { flex, bg, r, borders, p, text, layout } = zero;

function useNewItemsPer(array: any[], interval = 60) {
  const [newItems, setNewItems] = useState(0);

  useEffect(() => {
    const intervalId = setInterval(() => {
      setNewItems(array.length);
    }, interval * 1000);

    return () => clearInterval(intervalId);
  }, [array, interval]);

  return newItems;
}

export default function ChatPanel() {
  // Get real data from hooks
  const isLive = useLiveUser();
  const chat = useLivestreamStore((x) => x.chat);
  const ingestConnectionState = usePlayerStore((x) => x.ingestConnectionState);

  const isConnected = ingestConnectionState === "connected";
  const canModerate = isLive && isConnected;

  // Track initial load timestamp (when component mounts)
  const [initialLoadTime] = React.useState(() => Date.now());

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
        bg.gray[800],
        r[3],
        borders.width.thin,
        borders.color.gray[700],
        layout.flex.column,
      ]}
    >
      <View
        style={[
          layout.flex.row,
          layout.flex.spaceBetween,
          layout.flex.alignCenter,
          p[4],
          borders.bottom.width.thin,
          borders.bottom.color.gray[700],
        ]}
      >
        <Text style={[text.white, { fontSize: 18, fontWeight: "600" }]}>
          Live Chat
        </Text>
        <View style={[layout.flex.row, layout.flex.alignCenter, { gap: 8 }]}>
          <View
            style={[
              { width: 6, height: 6, borderRadius: 3 },
              isLive && isConnected ? bg.green[500] : bg.gray[500],
            ]}
          />
          <Text style={[text.gray[400], { fontSize: 12 }]}>
            {messagesPerMinute} msg/min
          </Text>
        </View>
      </View>
      <View style={[flex.values[1], p[2]]}>
        <Chat canModerate={canModerate} shownMessages={50} />
        <ChatBox
          emojiData={emojiData}
          chatBoxStyle={[
            bg.gray[700],
            borders.width.thin,
            borders.color.gray[600],
            r[2],
            p[3],
            !isConnected && { opacity: 0.6 },
          ]}
        />
      </View>
    </View>
  );
}
