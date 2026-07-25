import {
  Chat,
  ChatBox,
  LivestreamProvider,
  PlayerProvider,
  StreamNotificationProvider,
  Text,
  tokens,
  usePlayerStore,
  zero,
} from "@streamplace/components";
import { EmojiPicker } from "components/emoji-picker/emoji-picker";
import { ArrowRight } from "lucide-react-native";
import { useEffect } from "react";
import { Pressable, View } from "react-native";
import { useStore } from "store";
import { useUserProfile } from "store/hooks";
import { useEmojiData } from "utils/emoji";

interface ChatPopoutParams {
  user: string;
  reverse?: string;
  hideAfter?: string;
  hideChatBox?: string;
  hidePinnedComments?: string;
  showNotifications?: string;
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
  const hideSidebar = useStore((x) => x.setSidebarHidden);
  const showSidebar = useStore((x) => x.setSidebarUnhidden);
  const openLoginModal = useStore((x) => x.openLoginModal);

  useEffect(() => {
    hideSidebar();
    return () => {
      showSidebar();
    };
  }, [showSidebar]);

  const reverseChat = params.reverse === "true";
  const hideAfter = params.hideAfter
    ? parseInt(params.hideAfter, 10)
    : undefined;
  const hideChatBox = params.hideChatBox === "true";
  const hidePinnedComments = params.hidePinnedComments === "true";

  useEffect(() => {
    setSrc(params.user);
  }, [params.user, setSrc]);

  const chat = (
    <Chat
      reverse={reverseChat}
      hideAfter={hideAfter}
      style={[zero.flex.values[1]]}
    />
  );

  return (
    <View
      style={[
        zero.flex.values[1],
        zero.layout.flex.column,
        zero.m[2],
        { maxHeight: "100vh" },
      ]}
    >
      {!hidePinnedComments ? (
        <StreamNotificationProvider position="top">
          {chat}
        </StreamNotificationProvider>
      ) : (
        chat
      )}
      {!hideChatBox &&
        (profile ? (
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
        ) : (
          <Pressable
            onPress={() => openLoginModal({ name: "ChatPopout" })}
            style={[
              zero.layout.flex.row,
              zero.layout.flex.center,
              zero.gap.all[4],
              zero.r.xl,
              {
                padding: 18,
                backgroundColor: "rgba(255, 255, 255, 0.1)",
                maxWidth: tokens.breakpoints.sm,
              },
            ]}
          >
            <Text>Log in or sign up to chat</Text>
            <ArrowRight />
          </Pressable>
        ))}
    </View>
  );
}
