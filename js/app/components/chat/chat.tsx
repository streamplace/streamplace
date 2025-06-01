import { Reply, Settings, X } from "@tamagui/lucide-icons";
import {
  createBlockRecord,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import {
  MessageViewHydrated,
  useChat,
  usePlayerActions,
  usePlayerLivestream,
} from "features/player/playerSlice";
import usePlatform from "hooks/usePlatform";
import { useEffect, useRef, useState } from "react";
import { Linking, TouchableOpacity } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";

import { RichText } from "@atproto/api";
import {
  isMention,
  Link,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import { $Typed } from "@atproto/api/src/client/util";
import { Button, ScrollView, Sheet, Text, useMedia, View } from "tamagui";
import { RichtextSegment, segmentize } from "../../utils/facet";

export default function Chat({
  isChatVisible,
  setIsChatVisible: _setIsChatVisible,
}: {
  isChatVisible: boolean;
  setIsChatVisible: (visible: boolean) => void;
}) {
  const [open, setOpen] = useState(false);
  const [modMessage, setMessage] = useState<MessageViewHydrated | null>(null);
  const [isAtBottom, setIsAtBottom] = useState(true);
  const chat = useAppSelector(useChat());
  const scrollRef = useRef<ScrollView>(null);
  const livestream = useAppSelector(usePlayerLivestream());
  const userProfile = useAppSelector(selectUserProfile);
  const myStream = !!(
    userProfile &&
    livestream &&
    userProfile.did &&
    livestream.author.did &&
    userProfile.did === livestream.author.did
  );

  const handleScroll = (event: any) => {
    const { layoutMeasurement, contentOffset, contentSize } = event.nativeEvent;
    const paddingToBottom = 100;
    const isAtBottom =
      layoutMeasurement.height + contentOffset.y >=
      contentSize.height - paddingToBottom;
    setIsAtBottom(isAtBottom);
  };

  useEffect(() => {
    if (chat && isAtBottom) {
      scrollRef.current?.scrollToEnd({ animated: true });
    }
  }, [chat, isAtBottom]);

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
                    <ChatMessageText message={modMessage} chat={chat || []} />
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
              if (isAtBottom) {
                scrollRef.current?.scrollToEnd({ animated: true });
              }
            }}
            onLayout={() => {
              if (isAtBottom) {
                scrollRef.current?.scrollToEnd({ animated: true });
              }
            }}
            onScroll={handleScroll}
            scrollEventThrottle={16}
          >
            {chat.map((message) => (
              <ChatMessageRow
                key={message.cid}
                message={message}
                setOpen={setOpen}
                setMessage={setMessage}
                myStream={myStream}
                replyHandle={undefined}
                chat={chat}
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
  replyHandle: _replyHandle,
  chat,
}: {
  message: MessageViewHydrated;
  setOpen: (open: boolean) => void;
  setMessage: (message: MessageViewHydrated) => void;
  myStream: boolean;
  replyHandle?: string;
  chat: MessageViewHydrated[];
}): JSX.Element {
  const [hover, setHover] = useState(false);
  const playerActions = usePlayerActions();
  const { isWeb } = usePlatform();

  const moderateMessage = () => {
    if (!myStream) {
      return;
    }
    setOpen(true);
    setMessage(message);
  };

  const handleReply = () => {
    playerActions.setReplyToMessage(message);
  };

  const replyTo = message.replyTo as MessageViewHydrated | undefined;
  const hasReply = !!replyTo;
  const replyToHandle = replyTo?.author?.handle;
  const replyToText = replyTo?.record?.text;
  const replyToColor = replyTo?.chatProfile?.color
    ? `rgb(${replyTo.chatProfile.color.red}, ${replyTo.chatProfile.color.green}, ${replyTo.chatProfile.color.blue})`
    : "$accentColor";

  return (
    <View
      flexDirection="row"
      display="block"
      paddingVertical={isWeb ? 6 : 4} // Adjust padding for web
      position="relative"
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      hoverStyle={{ backgroundColor: "rgba(255,255,255,0.1)" }}
      onPress={() => {
        if (!isWeb) {
          moderateMessage();
        }
      }}
      onLongPress={handleReply}
    >
      {hasReply && (
        <View
          position="absolute"
          left={6}
          top={-8}
          width={2}
          height={16}
          opacity={0.7}
        />
      )}
      <View flexDirection="column" gap="$1" flex={1}>
        {/* Reply section */}
        {hasReply && (
          <View
            flexDirection="column"
            marginBottom="$2"
            paddingLeft="$3"
            position="relative"
          >
            {/* Vertical reply line */}
            <View
              position="absolute"
              left={6}
              top={0}
              bottom={0}
              width={2}
              borderRadius={2}
              backgroundColor="$accentColor"
              opacity={0.5}
            />
            {/* Reply preview */}
            <View
              flexDirection="row"
              alignItems="center"
              gap="$1"
              paddingVertical="$1"
              paddingHorizontal="$2"
              borderRadius="$2"
              marginLeft="-$1"
            >
              <Text fontSize={12} color={replyToColor} fontWeight="bold">
                {replyToHandle ? `@${replyToHandle}` : ""}
              </Text>
              <Text
                fontSize={12}
                color="$color"
                opacity={0.7}
                numberOfLines={1}
                flex={1}
              >
                {replyToText || ""}
              </Text>
            </View>
          </View>
        )}

        {/* Message content */}
        <View flexDirection="row" alignItems="flex-start" gap="$2">
          <ChatMessageText message={message} chat={chat} />
        </View>
      </View>
      <View
        position="absolute"
        flexDirection="row"
        right={0}
        top={0}
        bottom={0}
        alignItems="stretch"
        justifyContent="flex-end"
        gap="$2"
      >
        {isWeb && (
          <TouchableOpacity
            style={{
              display: hover ? "flex" : "none",
              alignItems: "center",
              justifyContent: "center",
              padding: 4,
            }}
            onPress={handleReply}
          >
            <Reply size={16} />
          </TouchableOpacity>
        )}
        {isWeb && myStream && (
          <TouchableOpacity
            style={{
              display: hover ? "flex" : "none",
              alignItems: "center",
              justifyContent: "center",
              padding: 4,
            }}
            onPress={moderateMessage}
          >
            <Settings size={16} />
          </TouchableOpacity>
        )}
      </View>
    </View>
  );
}

function byteOffsetToCharIndex(text: string, byteOffset: number): number {
  const encoder = new TextEncoder();
  let bytesCount = 0;
  for (let i = 0; i < text.length; i++) {
    const charBytes = encoder.encode(text[i]);
    if (bytesCount + charBytes.length > byteOffset) {
      return i;
    }
    bytesCount += charBytes.length;
  }
  return text.length;
}

const ChatMessageText = ({
  message,
  chat = [],
}: {
  message: MessageViewHydrated;
  chat?: MessageViewHydrated[];
}) => {
  const rt = new RichText({ text: message.record.text });
  rt.detectFacetsWithoutResolution();

  // Process facets to add DID information for mentions
  rt.facets?.forEach((facet) => {
    facet.features.forEach((feature) => {
      if (isMention(feature)) {
        const startCharIndex = byteOffsetToCharIndex(
          message.record.text,
          facet.index.byteStart,
        );
        const endCharIndex = byteOffsetToCharIndex(
          message.record.text,
          facet.index.byteEnd,
        );
        const mentionText = message.record.text.slice(
          startCharIndex,
          endCharIndex,
        );
        // Find the mentioned user by their handle (removing the @ symbol)
        const mentionedUser = chat.find(
          (msg) => msg.author.handle === mentionText.slice(1),
        );
        if (mentionedUser) {
          feature.did = mentionedUser.author.did;
        }
      }
    });
  });

  return (
    <Text fontSize={13}>
      <Text
        color={getRgbColor(message.chatProfile?.color)}
        cursor="pointer"
        onPress={() =>
          Linking.openURL(`https://bsky.app/profile/${message.author.did}`)
        }
      >
        {message.author.handle
          ? `@${message.author.handle}`
          : message.author.did}
        :
      </Text>
      <Text> </Text>
      <RichTextMessage
        text={message.record.text}
        facets={rt.facets as Facet[]}
        chat={chat}
      />
    </Text>
  );
};

interface Facet {
  index: {
    byteStart: number;
    byteEnd: number;
  };
  features: Array<{
    $type: string;
    uri?: string;
    did?: string;
  }>;
}

const getRgbColor = (color?: { red: number; green: number; blue: number }) =>
  color ? `rgb(${color.red}, ${color.green}, ${color.blue})` : "$accentColor";

const segmentedObject = (
  obj: RichtextSegment,
  chat: MessageViewHydrated[],
  index: number,
) => {
  if (obj.features && obj.features.length > 0) {
    let ftr = obj.features[0];
    // afaik there shouldn't be a case where facets overlap, at least currently
    if (ftr.$type === "app.bsky.richtext.facet#link") {
      let linkftr = ftr as $Typed<Link>;
      return (
        <Text
          key={`mention-${index}`}
          color="#9090f0"
          cursor="pointer"
          onPress={() => Linking.openURL(linkftr.uri || "")}
        >
          {obj.text}
        </Text>
      );
    } else if (ftr.$type === "app.bsky.richtext.facet#mention") {
      let mtnftr = ftr as $Typed<Mention>;
      const mentionedUserMessage = chat.find(
        (msg) => msg.author.did === mtnftr.did,
      );
      return (
        <Text
          key={`mention-${index}`}
          color={getRgbColor(mentionedUserMessage?.chatProfile?.color)}
          cursor="pointer"
          onPress={() =>
            Linking.openURL(`https://bsky.app/profile/${mtnftr.did || ""}`)
          }
        >
          {obj.text}
        </Text>
      );
    }
  } else {
    return <Text>{obj.text}</Text>;
  }
};

const RichTextMessage = ({
  text,
  facets,
  chat = [],
}: {
  text: string;
  facets: Facet[];
  chat?: MessageViewHydrated[];
}) => {
  if (!facets?.length) return <Text>{text}</Text>;

  let segs = segmentize(text, facets);

  console.log(segs);

  return segs.map((seg, i) => segmentedObject(seg, chat, i));
};
