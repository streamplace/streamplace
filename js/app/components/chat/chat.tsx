import {
  useChat,
  useProfile,
  useReplyToMessage,
  useSetReplyToMessage,
} from "@streamplace/components";
import { Reply, ReplyAll, Settings, X } from "@tamagui/lucide-icons";
import {
  createBlockRecord,
  selectUserProfile,
} from "features/bluesky/blueskySlice";
import usePlatform from "hooks/usePlatform";
import { useEffect, useRef, useState } from "react";
import { Linking, TouchableOpacity } from "react-native";
import { useAppDispatch, useAppSelector } from "store/hooks";

import { $Typed, RichText } from "@atproto/api";
import {
  isMention,
  Link,
  Mention,
} from "@atproto/api/dist/client/types/app/bsky/richtext/facet";
import ReanimatedSwipeable, {
  SwipeableMethods,
} from "react-native-gesture-handler/ReanimatedSwipeable";
import Animated, {
  SharedValue,
  useAnimatedStyle,
} from "react-native-reanimated";
import { ChatMessageViewHydrated } from "streamplace";
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
  const [modMessage, setMessage] = useState<ChatMessageViewHydrated | null>(
    null,
  );
  const [isAtBottom, setIsAtBottom] = useState(true);
  const chat = useChat();
  const scrollRef = useRef<ScrollView>(null);
  const streamerProfile = useProfile();
  const userProfile = useAppSelector(selectUserProfile);
  const myStream = !!(
    userProfile &&
    userProfile.did &&
    userProfile.did === streamerProfile?.did
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
            marginHorizontal="$2"
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
            {chat.map((message, index) => (
              <ChatMessageRow
                key={message.cid + index}
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

const RightAction = (
  _progress: SharedValue<number>,
  drag: SharedValue<number>,
) => {
  const styleAnimation = useAnimatedStyle(() => {
    return {
      transform: [
        {
          translateX: drag.value + 50,
        },
      ],
    };
  });

  return (
    <Animated.View style={[styleAnimation, { height: "auto" }]}>
      <Text
        width="$4"
        backgroundColor="rgba(255,255,255,0.5)"
        borderRadius="$2"
        pt={"$1"}
        px={"$1"}
      >
        <ReplyAll />
      </Text>
    </Animated.View>
  );
};

function ChatMessageRow({
  message,
  setMessage,
  setOpen,
  myStream,
  replyHandle: _replyHandle,
  chat,
}: {
  message: ChatMessageViewHydrated;
  setOpen: (open: boolean) => void;
  setMessage: (message: ChatMessageViewHydrated) => void;
  myStream: boolean;
  replyHandle?: string;
  chat: ChatMessageViewHydrated[];
}): JSX.Element {
  const [hover, setHover] = useState(false);
  const setReplyToMessage = useSetReplyToMessage();
  const { isWeb } = usePlatform();

  const swipeableRef = useRef<SwipeableMethods>(null);
  const close = () => {
    let current: any = swipeableRef.current;
    if (current) {
      current.close();
    }
  };

  const currentReplyTo = useReplyToMessage();

  const moderateMessage = () => {
    if (!myStream) {
      return;
    }
    setOpen(true);
    setMessage(message);
  };

  const handleReply = () => {
    setReplyToMessage(message);
  };

  const replyTo = message.replyTo as ChatMessageViewHydrated | undefined;
  const hasReply = !!replyTo;
  const replyToHandle = replyTo?.author?.handle;
  const replyToText = replyTo?.record?.text;
  const replyToColor = replyTo?.chatProfile?.color
    ? `rgb(${replyTo.chatProfile.color.red}, ${replyTo.chatProfile.color.green}, ${replyTo.chatProfile.color.blue})`
    : "$accentColor";

  return (
    <View
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      onPress={() => {
        if (!isWeb) {
          moderateMessage();
        }
      }}
    >
      <View
        position="absolute"
        flexDirection="row"
        right={0}
        top="$-3"
        alignItems="stretch"
        justifyContent="flex-end"
        gap="$1"
        px="$1"
        backgroundColor="rgba(255,255,255,0.5)"
        borderRadius="$2"
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
      <ReanimatedSwipeable
        ref={swipeableRef}
        renderRightActions={RightAction}
        overshootRight={false}
        friction={2}
        enableTrackpadTwoFingerGesture
        rightThreshold={40}
        onSwipeableOpen={(r) => {
          if (r === "right") {
            handleReply();
          }
          close();
        }}
      >
        <View
          flexDirection="row"
          display="block"
          paddingVertical={isWeb ? 6 : 4} // Adjust padding for web
          paddingHorizontal={isWeb ? 6 : 4} // Adjust padding for web
          position="relative"
          hoverStyle={{ backgroundColor: "rgba(255,255,255,0.1)" }}
          backgroundColor={
            currentReplyTo?.cid === message.cid
              ? "rgba(180,180,255,0.1)"
              : "transparent"
          }
          borderRadius={isWeb ? 4 : 4}
          onPress={() => {
            if (!isWeb) {
              moderateMessage();
            }
          }}
          overflow="visible"
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
          <View flexDirection="column" gap="$1" flex={1} overflow="visible">
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
            <View
              flexDirection="row"
              alignItems="flex-start"
              justifyContent="flex-start"
              gap="$2"
            >
              <Text
                color="$gray10"
                fontSize="$2"
                mt="$0.5"
                style={{ fontVariantNumeric: "tabular-nums" }}
              >
                {new Date(message.record.createdAt).toLocaleString(undefined, {
                  hour: "2-digit",
                  minute: "2-digit",
                  hour12: false,
                })}
              </Text>
              <ChatMessageText message={message} chat={chat} />
            </View>
          </View>
        </View>
      </ReanimatedSwipeable>
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
  message: ChatMessageViewHydrated;
  chat?: ChatMessageViewHydrated[];
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
    <Text fontSize={13} flex={1}>
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
  chat: ChatMessageViewHydrated[],
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
    return <Text key={`text-${index}`}>{obj.text}</Text>;
  }
};

const RichTextMessage = ({
  text,
  facets,
  chat = [],
}: {
  text: string;
  facets: Facet[];
  chat?: ChatMessageViewHydrated[];
}) => {
  if (!facets?.length) return <Text>{text}</Text>;

  let segs = segmentize(text, facets);

  return segs.map((seg, i) => segmentedObject(seg, chat, i));
};

export function timeAgo(time: Date | number | string): string {
  let timestamp: number;

  if (typeof time === "number") {
    timestamp = time;
  } else if (typeof time === "string") {
    timestamp = new Date(time).getTime();
  } else if (time instanceof Date) {
    timestamp = time.getTime();
  } else {
    timestamp = Date.now();
  }

  const now = Date.now();
  let seconds = (now - timestamp) / 1000;
  let token = "ago";
  let listChoice = 1;

  if (seconds === 0) {
    return "Just now";
  }

  if (seconds < 0) {
    seconds = Math.abs(seconds);
    token = "from now";
    listChoice = 2;
  }

  // Show time for > 1 hour difference
  if (seconds > 3600) {
    const date = new Date(timestamp);
    // More than 1 day ago/from now: show shortened date + time
    if (seconds > 86400) {
      // Example format: "Apr 27, 14:35"
      return date.toLocaleString(undefined, {
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    }
    // Otherwise show only time for > 1 hour
    return date.toLocaleTimeString(undefined, {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  const timeFormats: [number, string, number | string][] = [
    [60, "seconds", 1], // 60
    [120, "1 minute ago", "1 minute from now"], // 60*2
    [3600, "minutes", 60], // 60*60, 60
    [7200, "1 hour ago", "1 hour from now"], // 60*60*2
    [86400, "hours", 3600], // 60*60*24, 60*60
    [172800, "Yesterday", "Tomorrow"], // 60*60*24*2
    [604800, "days", 86400], // 60*60*24*7, 60*60*24
    [1209600, "Last week", "Next week"], // 60*60*24*7*2
    [2419200, "weeks", 604800], // 60*60*24*7*4, 60*60*24*7
    [4838400, "Last month", "Next month"], // 60*60*24*7*4*2
    [29030400, "months", 2419200], // 60*60*24*7*4*12, 60*60*24*7*4
    [58060800, "Last year", "Next year"], // 60*60*24*7*4*12*2
    [2903040000, "years", 29030400], // 60*60*24*7*4*12*100, 60*60*24*7*4*12
    [5806080000, "Last century", "Next century"], // 60*60*24*7*4*12*100*2
    [58060800000, "centuries", 2903040000], // 60*60*24*7*4*12*100*20, 60*60*24*7*4*12*100
  ];

  for (const format of timeFormats) {
    if (seconds < format[0]) {
      if (typeof format[2] === "string") {
        return format[listChoice] as string;
      } else {
        return (
          Math.floor(seconds / (format[2] as number)) +
          " " +
          format[1] +
          " " +
          token
        );
      }
    }
  }

  return new Date(timestamp).toString();
}
