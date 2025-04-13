// import { useChat, ChatMessage } from "features/player/playerSlice";
import { Settings, X } from "@tamagui/lucide-icons";
import {
  createBlockRecord,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import {
  MessageViewHydrated,
  useChat,
  usePlayerLivestream,
} from "features/player/playerSlice";
import usePlatform from "hooks/usePlatform";
import { useEffect, useRef, useState } from "react";
import { TouchableOpacity } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";
import { Button, ScrollView, Sheet, Text, useMedia, View } from "tamagui";

export default function Chat({
  isChatVisible,
  setIsChatVisible,
}: {
  isChatVisible: boolean;
  setIsChatVisible: (visible: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  const [modMessage, setMessage] = useState<MessageViewHydrated | null>(null);
  const chat = useAppSelector(useChat());
  const scrollRef = useRef<ScrollView>(null);
  const livestream = useAppSelector(usePlayerLivestream());
  const userProfile = useAppSelector(selectUserProfile);
  const myStream = !!(
    userProfile &&
    livestream &&
    userProfile.did === livestream.author.did
  );
  useEffect(() => {
    if (chat) {
      scrollRef.current?.scrollToEnd({ animated: true });
    }
  }, [chat]);
  if (!chat) {
    return <></>;
  }
  const m = useMedia();
  const dispatch = useAppDispatch();
  return (
    <View f={1} position="relative">
      {isChatVisible && (
        <>
          <Sheet
            // forceRemoveScrollEnabled={open}
            open={open}
            modal={!m.gtXs}
            onOpenChange={setOpen}
            // snapPoints={snapPoints}
            // snapPointsMode={snapPointsMode}
            dismissOnSnapToBottom
            // position={position}
            // onPositionChange={setPosition}
            zIndex={100_000}
            animation="medium"
          >
            <Sheet.Overlay
              animation="lazy"
              backgroundColor="$shadow6"
              enterStyle={{ opacity: 0 }}
              exitStyle={{ opacity: 0 }}
            />
            <Sheet.Frame>
              <View
                f={1}
                alignItems="center"
                justifyContent="center"
                padding="$4"
                gap="$5"
                backgroundColor="$accentBackground"
              >
                <Button
                  position="absolute"
                  top="$0"
                  right="$0"
                  onPress={(e) => {
                    e.stopPropagation();
                    setOpen(false);
                  }}
                  marginRight={-15}
                  marginTop={-5}
                  backgroundColor="transparent"
                >
                  <X />
                </Button>
                {modMessage && (
                  <>
                    <ChatMessageText message={modMessage} />
                    {modMessage.author.did !== userProfile?.did && (
                      <Button
                        width="100%"
                        onPress={() => {
                          setOpen(false);
                          dispatch(
                            createBlockRecord({
                              subjectDID: modMessage.author.did,
                            }),
                          );
                        }}
                      >
                        <Text>Block @{modMessage.author.handle}</Text>
                      </Button>
                    )}
                    {modMessage.author.did === userProfile?.did && (
                      <>
                        <Button width="100%" disabled={true}>
                          <Text>(You can't block yourself!)</Text>
                        </Button>
                      </>
                    )}
                  </>
                )}
              </View>
            </Sheet.Frame>
          </Sheet>
          <ScrollView
            paddingHorizontal="$4"
            invertStickyHeaders={true}
            ref={scrollRef}
            onContentSizeChange={() => {
              scrollRef.current?.scrollToEnd({ animated: true });
            }}
            onLayout={() => {
              scrollRef.current?.scrollToEnd({ animated: true });
            }}
          >
            {chat.map((message) => (
              <ChatMessageRow
                key={message.cid}
                message={message}
                setOpen={setOpen}
                setMessage={setMessage}
                myStream={myStream}
              />
            ))}
          </ScrollView>
        </>
      )}
    </View>
  );
}

function ChatMessageRow({
  message,
  setMessage,
  setOpen,
  myStream,
}: {
  message: MessageViewHydrated;
  setOpen: (open: boolean) => void;
  setMessage: (message: MessageViewHydrated) => void;
  myStream: boolean;
}) {
  const [hover, setHover] = useState(false);
  const { isWeb } = usePlatform();
  const moderateMessage = () => {
    if (!myStream) {
      return;
    }
    setOpen(true);
    setMessage(message);
  };
  return (
    <View
      flexDirection="row"
      display="block"
      paddingVertical={4}
      position="relative"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      hoverStyle={{ backgroundColor: "rgba(255,255,255,0.1)" }}
      onPress={() => {
        if (!isWeb) {
          moderateMessage();
        }
      }}
    >
      <ChatMessageText message={message} />
      {isWeb && myStream && (
        <View
          position="absolute"
          flexDirection="row"
          right={0}
          top={0}
          left={0}
          bottom={0}
          alignItems="stretch"
          justifyContent="flex-end"
        >
          <TouchableOpacity
            style={{
              display: hover ? "flex" : "none",
              alignItems: "center",
              justifyContent: "center",
              padding: 4,
            }}
            onPress={() => {
              moderateMessage();
            }}
          >
            <Settings size={16} />
          </TouchableOpacity>
        </View>
      )}
    </View>
  );
}

const ChatMessageText = ({ message }: { message: MessageViewHydrated }) => {
  let color = "$accentColor";
  if (message.chatProfile?.color) {
    const { red, green, blue } = message.chatProfile.color;
    color = `rgb(${red}, ${green}, ${blue})`;
  }
  return (
    <Text fontSize={13}>
      <Text color={color}>
        {message.author.handle
          ? `@${message.author.handle}`
          : message.author.did}
        :
      </Text>
      <Text> {message.record.text}</Text>
    </Text>
  );
};
