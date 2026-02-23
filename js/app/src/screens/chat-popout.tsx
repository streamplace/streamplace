import {
  Chat,
  ChatBox,
  LivestreamProvider,
  PlayerProvider,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { useEffect } from "react";
import { View } from "react-native";
import { useUserProfile } from "store/hooks";
import { useEmojiData } from "utils/emoji";

export default function PopoutChat({ route }) {
  const user = route.params?.user;
  if (typeof user !== "string") {
    return <View />;
  }

  return (
    <LivestreamProvider src={user}>
      <PlayerProvider>
        <PopoutChatInner user={user} />
      </PlayerProvider>
    </LivestreamProvider>
  );
}

export function PopoutChatInner({ user }: { user: string }) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  const profile = useUserProfile();
  const emojiData = useEmojiData();
  useEffect(() => {
    setSrc(user);
  }, [user]);

  return (
    <View style={[{ position: "relative" }, zero.flex.values[1], zero.m[2]]}>
      <View
        style={[
          zero.flex.values[1],
          { position: "absolute", width: "100%", minHeight: "100%", bottom: 0 },
        ]}
      >
        <Chat />
        {profile && (
          <ChatBox
            emojiData={emojiData}
            isPopout={true}
            emojiPicker={(isOpen, onClose) => (
              <EmojiPicker isOpen={isOpen} onClose={onClose} customEmoji={[]} />
            )}
          />
        )}
      </View>
    </View>
  );
}
