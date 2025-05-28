import {
  LivestreamProvider,
  StreamplaceProvider,
  useChat,
  useStreamplaceStore,
} from "@streamplace/components";
import React from "react";
import { ScrollView, Text, View } from "react-native";

export default function App() {
  return (
    <StreamplaceProvider url="https://longos.iameli.link">
      <Content />
    </StreamplaceProvider>
  );
}

function Content() {
  const liveUsers = useStreamplaceStore((x) => x.liveUsers);
  return (
    <View
      style={{
        flex: 1,
        justifyContent: "center",
        alignItems: "center",
        backgroundColor: "#222",
      }}
    >
      {liveUsers.map((user) => (
        <Text style={{ color: "white" }} key={user.author.did}>
          {user.author.handle}
        </Text>
      ))}
      <LivestreamProvider src="scumb.ag">
        <LivestreamContent />
      </LivestreamProvider>
    </View>
  );
}

function LivestreamContent() {
  const chat = useChat();
  return (
    <ScrollView>
      {chat.map((message) => (
        <Text style={{ color: "white" }} key={message.uri}>
          {message.record.text}
        </Text>
      ))}
    </ScrollView>
  );
}
