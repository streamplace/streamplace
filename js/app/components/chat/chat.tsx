// import { useChat, ChatMessage } from "features/player/playerSlice";
import { ChatMessage, useChat } from "features/player/playerSlice";
import { useEffect, useRef } from "react";
import { useAppSelector } from "store/hooks";
import { View, Text, ScrollView } from "tamagui";

export default function Chat() {
  const chat = useAppSelector(useChat());
  const scrollRef = useRef<ScrollView>(null);
  useEffect(() => {
    if (chat) {
      scrollRef.current?.scrollToEnd({ animated: true });
    }
  }, [chat]);
  if (!chat) {
    return <></>;
  }
  return (
    <ScrollView
      paddingHorizontal="$4"
      invertStickyHeaders={true}
      ref={scrollRef}
    >
      {chat.toReversed().map((message) => (
        <ChatMessageRow key={message.cid} message={message} />
      ))}
    </ScrollView>
  );
}

function ChatMessageRow({ message }: { message: ChatMessage }) {
  return (
    <View flexDirection="row" display="block" paddingVertical="$2">
      <Text>
        <Text color="$accentColor">@{message.repo.handle}: </Text>
        <Text>{message.post.text}</Text>
      </Text>
    </View>
  );
}
