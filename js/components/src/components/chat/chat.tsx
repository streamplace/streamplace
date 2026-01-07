import { Ellipsis, Reply } from "lucide-react-native";
import { ComponentProps, memo, useEffect, useRef, useState } from "react";
import { Keyboard, Platform, Pressable } from "react-native";
import { FlatList } from "react-native-gesture-handler";
import Swipeable, {
  SwipeableMethods,
} from "react-native-gesture-handler/ReanimatedSwipeable";
import Reanimated, {
  SharedValue,
  useAnimatedStyle,
} from "react-native-reanimated";
import { ChatMessageViewHydrated } from "streamplace";
import {
  SystemMessage,
  Text,
  useChat,
  usePlayerStore,
  useSetReplyToMessage,
  View,
} from "../../";
import { bg, flex, px, py } from "../../lib/theme/atoms";
import { RenderChatMessage } from "./chat-message";
import { ModView } from "./mod-view";

function RightAction(prog: SharedValue<number>, drag: SharedValue<number>) {
  const styleAnimation = useAnimatedStyle(() => {
    return {
      transform: [{ translateX: drag.value + 25 }],
    };
  });

  return (
    <Reanimated.View style={[styleAnimation]}>
      <Reply color="white" />
    </Reanimated.View>
  );
}

function LeftAction(prog: SharedValue<number>, drag: SharedValue<number>) {
  const styleAnimation = useAnimatedStyle(() => {
    return {
      transform: [{ translateX: drag.value - 25 }],
    };
  });

  return (
    <Reanimated.View style={[styleAnimation]}>
      <Ellipsis color="white" />
    </Reanimated.View>
  );
}

// ios/android, 25, else 100 msgs
const SHOWN_MSGS =
  Platform.OS === "ios" || Platform.OS === "android" ? 25 : 100;

const keyExtractor = (item: ChatMessageViewHydrated, index: number) => {
  return `${item.uri}`;
};

// Actions bar for larger screens
const ActionsBar = memo(
  ({
    item,
    visible,
    hoverTimeoutRef,
  }: {
    item: ChatMessageViewHydrated;
    visible: boolean;
    hoverTimeoutRef: React.MutableRefObject<NodeJS.Timeout | null>;
  }) => {
    const setReply = useSetReplyToMessage();
    const setModMsg = usePlayerStore((state) => state.setModMessage);

    if (!visible) return null;

    return (
      <View
        style={[
          {
            position: "absolute",
            top: -14,
            right: 8,
            flexDirection: "row",
            backgroundColor: "rgba(180,180,180, 0.5)",
            borderRadius: 6,
            borderWidth: 1,
            padding: 1,
            gap: 4,
            zIndex: 10,
            maxWidth: 120,
            flexShrink: 0,
          },
        ]}
      >
        <Pressable
          onPress={() => setReply(item)}
          style={[
            {
              padding: 6,
              borderRadius: 4,
              backgroundColor: "rgba(255, 255, 255, 0.1)",
            },
          ]}
          onHoverIn={() => {
            // Keep the actions bar visible when hovering over it
            if (hoverTimeoutRef.current) {
              clearTimeout(hoverTimeoutRef.current);
              hoverTimeoutRef.current = null;
            }
          }}
        >
          <Reply color="white" size={16} />
        </Pressable>
        <Pressable
          onPress={() => setModMsg(item)}
          style={[
            {
              padding: 6,
              borderRadius: 4,
              backgroundColor: "rgba(255, 255, 255, 0.1)",
            },
          ]}
          onHoverIn={() => {
            // Keep the actions bar visible when hovering over it
            if (hoverTimeoutRef.current) {
              clearTimeout(hoverTimeoutRef.current);
              hoverTimeoutRef.current = null;
            }
          }}
        >
          <Ellipsis color="white" size={16} />
        </Pressable>
      </View>
    );
  },
);

const ChatLine = memo(({ item }: { item: ChatMessageViewHydrated }) => {
  const setReply = useSetReplyToMessage();
  const setModMsg = usePlayerStore((state) => state.setModMessage);
  const swipeableRef = useRef<SwipeableMethods | null>(null);
  const [isHovered, setIsHovered] = useState(false);
  const hoverTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const handleHoverIn = () => {
    if (hoverTimeoutRef.current) {
      clearTimeout(hoverTimeoutRef.current);
      hoverTimeoutRef.current = null;
    }
    setIsHovered(true);
  };

  const handleHoverOut = () => {
    hoverTimeoutRef.current = setTimeout(() => {
      setIsHovered(false);
    }, 50);
  };

  useEffect(() => {
    return () => {
      if (hoverTimeoutRef.current) {
        clearTimeout(hoverTimeoutRef.current);
      }
    };
  }, []);

  if (item.author.did === "did:sys:system") {
    return (
      <SystemMessage
        timestamp={new Date(item.record.createdAt)}
        title={item.record.text}
      />
    );
  }

  if (Platform.OS === "web") {
    return (
      <View
        style={[
          py[1],
          px[2],
          {
            position: "relative",
            borderRadius: 8,
            minWidth: 0,
            maxWidth: "100%",
          },
          isHovered && bg.gray[950],
        ]}
        onPointerEnter={handleHoverIn}
        onPointerLeave={handleHoverOut}
      >
        <Pressable style={[{ minWidth: 0, maxWidth: "100%" }]}>
          <RenderChatMessage item={item} />
        </Pressable>
        <ActionsBar
          item={item}
          visible={isHovered}
          hoverTimeoutRef={hoverTimeoutRef}
        />
      </View>
    );
  }

  return (
    <>
      <Swipeable
        containerStyle={[py[1]]}
        friction={2}
        enableTrackpadTwoFingerGesture
        rightThreshold={40}
        leftThreshold={40}
        renderRightActions={Platform.OS === "android" ? undefined : RightAction}
        renderLeftActions={Platform.OS === "android" ? undefined : LeftAction}
        overshootFriction={9}
        ref={swipeableRef}
        onSwipeableOpen={(r) => {
          if (r === (Platform.OS === "android" ? "right" : "left")) {
            setReply(item);
          }
          if (r === (Platform.OS === "android" ? "left" : "right")) {
            setModMsg(item);
          }
          // close this swipeable
          const swipeable = swipeableRef.current;
          if (swipeable) {
            swipeable.close();
          }
        }}
      >
        <RenderChatMessage item={item} />
      </Swipeable>
    </>
  );
});

export function Chat({
  shownMessages = SHOWN_MSGS,
  style: propsStyle,
  ...props
}: ComponentProps<typeof View> & {
  shownMessages?: number;
  style?: ComponentProps<typeof View>["style"];
}) {
  const chat = useChat();
  const [isScrolledUp, setIsScrolledUp] = useState(false);

  const handleScroll = (event: any) => {
    const { contentOffset } = event.nativeEvent;

    const scrolledUp = contentOffset.y > 20; // threshold

    if (scrolledUp !== isScrolledUp) {
      setIsScrolledUp(scrolledUp);

      // Dismiss keyboard when scrolled up
      if (scrolledUp && Platform.OS !== "web") {
        Keyboard.dismiss();
      }
    }
  };

  if (!chat)
    return (
      <View style={[flex.shrink[1], { minWidth: 0, maxWidth: "100%" }]}>
        <Text>Loading chat...</Text>
      </View>
    );

  return (
    <View
      style={[flex.shrink[1], { minWidth: 0, maxWidth: "100%" }].concat(
        propsStyle || [],
      )}
    >
      <FlatList
        style={[
          flex.grow[1],
          flex.shrink[1],
          { minWidth: 0, maxWidth: "100%" },
        ]}
        data={chat.slice(0, shownMessages)}
        inverted={true}
        keyExtractor={keyExtractor}
        renderItem={({ item, index }) => <ChatLine item={item} />}
        removeClippedSubviews={true}
        maxToRenderPerBatch={10}
        initialNumToRender={10}
        updateCellsBatchingPeriod={50}
        onScroll={handleScroll}
        scrollEventThrottle={16}
        nestedScrollEnabled={true}
      />
      <ModView />
    </View>
  );
}
