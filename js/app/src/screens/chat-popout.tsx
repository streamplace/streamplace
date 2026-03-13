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

interface ChatPopoutParams {
  user: string;
  reverse?: string;
  hideAfter?: string;
  hideChatBox?: string;
}

export default function PopoutChat({ route }) {
  const user = route.params?.user;
  if (typeof user !== "string") {
    return <View />;
  }

  const params: ChatPopoutParams = {
    user,
    ...(route.params || {}),
  };

  return (
    <LivestreamProvider src={user}>
      <PlayerProvider>
        <PopoutChatInner params={params} />
      </PlayerProvider>
    </LivestreamProvider>
  );
}

export function PopoutChatInner({ params }: { params: ChatPopoutParams }) {
  const setSrc = usePlayerStore((x) => x.setSrc);
  const profile = useUserProfile();
  const emojiData = useEmojiData();

  const reverseChat = params.reverse === "true";
  const hideAfter = params.hideAfter
    ? parseInt(params.hideAfter, 10)
    : undefined;
  const hideChatBox = params.hideChatBox === "true";

  useEffect(() => {
    setSrc(params.user);
  }, [params.user, setSrc]);

  return (
    <View style={[{ position: "relative" }, zero.flex.values[1], zero.m[2]]}>
      <View
        style={[
          zero.flex.values[1],
          { position: "absolute", width: "100%", minHeight: "100%", bottom: 0 },
        ]}
      >
        <Chat
          {...(reverseChat || hideAfter
            ? ({
                ...(reverseChat ? { reverse: true } : {}),
                ...(hideAfter ? { hideAfter } : {}),
              } as any)
            : {})}
        />
        {profile && !hideChatBox && (
          <ChatBox
            emojiData={emojiData}
            isPopout={true}
            emojiPicker={(isOpen, onClose, onSelect) => (
              <EmojiPicker
                isOpen={isOpen}
                onClose={onClose}
                onSelect={onSelect}
                customEmoji={[]}
              />
            )}
          />
        )}
      </View>
    </View>
  );
}
