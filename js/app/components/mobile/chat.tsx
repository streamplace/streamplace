import { View, atoms } from "@streamplace/components";
import Chat from "components/chat/chat";
import ChatBox from "components/chat/chat-box";
const { borderRadius, bottom, gap, h, layout, w, zIndex } = atoms;

type ChatPanelProps = {
  isPlayerRatioGreater: boolean;
  slideKeyboard: number;
};

export function ChatPanel({
  isPlayerRatioGreater,
  slideKeyboard,
}: ChatPanelProps) {
  return (
    <View
      style={[
        isPlayerRatioGreater
          ? layout.position.relative
          : layout.position.absolute,
        h.percent[40],
        bottom[0],
        zIndex[10],
        w.percent[100],
        { transform: [{ translateY: slideKeyboard }] },
        {
          backgroundColor: "rgba(0, 0, 0, 0.4)",
          borderRadius: borderRadius["2xl"],
        },
      ]}
    >
      <Chat isChatVisible={true} setIsChatVisible={() => true} />
      <View style={[layout.flex.column, gap.all[2], { padding: 10 }]}>
        <ChatBox
          isChatVisible={true}
          chatBoxStyle={{ borderRadius: borderRadius.xl }}
        />
      </View>
    </View>
  );
}
