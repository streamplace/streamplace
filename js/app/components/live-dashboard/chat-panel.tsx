import { Chat, ChatBox, zero } from "@streamplace/components";
import emojiData from "assets/emoji-data.json";
import { Text, View } from "react-native";

const { flex, bg, r, borders, p, text, layout } = zero;

export default function ChatPanel() {
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
      </View>
      <View style={[flex.values[1], p[2]]}>
        <Chat canModerate={true} shownMessages={50} />
        <ChatBox
          emojiData={emojiData}
          chatBoxStyle={[
            bg.gray[700],
            borders.width.thin,
            borders.color.gray[600],
            r[2],
            p[3],
          ]}
        />
      </View>
    </View>
  );
}
