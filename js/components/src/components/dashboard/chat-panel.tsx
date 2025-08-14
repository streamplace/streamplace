import { Text, View } from "react-native";
import emojiData from "../../assets/emoji-data.json";
import * as zero from "../../ui";
import { Chat } from "../chat/chat";
import { ChatBox } from "../chat/chat-box";

const { flex, bg, r, borders, p, px, py, text, layout } = zero;

interface ChatPanelProps {
  isLive: boolean;
  isConnected: boolean;
  messagesPerMinute?: number;
  canModerate?: boolean;
  shownMessages?: number;
}

export default function ChatPanel({
  isLive,
  isConnected,
  messagesPerMinute = 0,
  canModerate = false,
  shownMessages = 50,
}: ChatPanelProps) {
  return (
    <View
      style={[
        flex.values[1],
        bg.neutral[900],
        borders.width.thin,
        borders.color.neutral[700],
        layout.flex.column,
        r.lg,
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
          Chat
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
          <Chat canModerate={canModerate} shownMessages={shownMessages} />
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
